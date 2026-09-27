package stack

import (
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybtachyon/sheidan/internal/ipdeny"
	"github.com/gin-gonic/gin"
)

// defaultBadTargetEnv is the standard blocklist variable deploy
// environments already speak.
const defaultBadTargetEnv = "SHEIDAN_BLOCK_IPS"

// BadTargetParams tunes the gate.badiptarget slot. A nil bag selects
// the shipped defaults: the roster rides the standard environment
// variable as comma-separated entries, refusals speak 403, and the
// source is sampled once at build time. A provided bag takes its
// fields verbatim, where an empty EnvVar disables the environment
// source, a File names a roster file that unions with the
// environment, and RefreshEvery bounds how eagerly the file's
// modification time is polled; zero periods sample the file once.
type BadTargetParams struct {
	EnvVar       string
	File         string
	DenyStatus   int
	RefreshEvery time.Duration
}

// acceptsBadTargetParams reports whether the bag shapes the
// gate.badiptarget slot expects.
func acceptsBadTargetParams(params any) bool {
	_, ok := params.(*BadTargetParams)
	return ok
}

// rosterSource stewards the live blocklist. The current list publishes
// through an atomic pointer, so a refresh never stalls a request, and
// the source remembers the modification time it last ingested to
// throttle re-reads behind a mutex.
type rosterSource struct {
	current  atomic.Pointer[ipdeny.List]
	mu       sync.Mutex
	file     string
	period   time.Duration
	lastPoll time.Time
	lastSeen time.Time
}

// load hands out the current roster for a match.
func (rs *rosterSource) load() *ipdeny.List { return rs.current.Load() }

// tick re-reads the file roster when its modification time advances
// past the last ingestion. Absent files clear the roster, because
// withdrawing the roster is an explicit act; unreadable or malformed
// revisions keep the last good one, so a botched edit cannot lock
// everybody in or out.
func (rs *rosterSource) tick() {
	if rs.file == "" || rs.period <= 0 {
		return
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	now := time.Now()
	if now.Sub(rs.lastPoll) < rs.period {
		return
	}
	rs.lastPoll = now
	info, err := os.Stat(rs.file)
	if err != nil {
		if os.IsNotExist(err) {
			empt := ipdeny.List{}
			rs.current.Store(&empt)
		}
		return
	}
	if info.ModTime().Equal(rs.lastSeen) {
		return
	}
	fresh, err := ipdeny.LoadFile(rs.file)
	if err != nil {
		log.Printf("ERROR: badtarget roster %s is unusable (%v); keeping the last good roster", rs.file, err)
		return
	}
	rs.lastSeen = info.ModTime()
	rs.current.Store(&fresh)
}

// newBadTarget builds the gate.badiptarget slot middleware. The gate
// tests the normalized client address against the roster and answers
// the configured denial status on a hit. Unlike the capacity guards,
// it ignores the intake-bypass stamp: a monitor probing from a
// blocked address is precisely the traffic a blocklist exists to
// kill. Startup announcements name malformed rosters and private
// ranges loudly, per the slice's no-silence rule.
func newBadTarget(params any) gin.HandlerFunc {
	cfg := BadTargetParams{EnvVar: defaultBadTargetEnv, DenyStatus: http.StatusForbidden}
	if params != nil {
		cfg = *(params.(*BadTargetParams))
	}
	if cfg.DenyStatus == 0 {
		cfg.DenyStatus = http.StatusForbidden
	}
	combined := ""
	if cfg.EnvVar != "" {
		combined += os.Getenv(cfg.EnvVar)
	}
	if cfg.File != "" {
		data, err := os.ReadFile(cfg.File)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("ERROR: badtarget roster file %s is unreadable (%v)", cfg.File, err)
			}
		} else {
			combined += "\n" + string(data)
		}
	}
	list, err := ipdeny.Parse(combined)
	if err != nil {
		log.Printf("ERROR: badtarget roster is malformed (%v); the gate ships empty until the source heals", err)
		list = ipdeny.List{}
	} else {
		for _, text := range list.PrivateTexts() {
			log.Printf("WARNING: badtarget roster entry %q names a private or loopback range; behind NAT you are likely screening your own fleet", text)
		}
	}
	src := &rosterSource{file: cfg.File, period: cfg.RefreshEvery}
	src.current.Store(&list)
	return func(c *gin.Context) {
		src.tick()
		roster := src.load()
		if roster.Len() == 0 {
			c.Next()
			return
		}
		client := c.ClientIP()
		if client == "" {
			c.Next()
			return
		}
		addr := net.ParseIP(client)
		if addr == nil || !roster.Matches(addr) {
			c.Next()
			return
		}
		c.AbortWithStatus(cfg.DenyStatus)
	}
}
