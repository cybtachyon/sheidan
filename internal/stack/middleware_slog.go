package stack

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"

	gsrequestid "github.com/gin-contrib/requestid"
	gsslog "github.com/gin-contrib/slog"
	"github.com/gin-gonic/gin"
)

// defaultLogWriter is the shared console outlet for the logging slot
// and the guarded recovery, so both kinds of records interleave in
// one stream.
var defaultLogWriter = os.Stdout

// defaultSlogSkipPatterns anchors the route families whose traffic is
// noise: statically served assets and the debug profiler group. The
// module's exact-match skip list cannot cover subtree families, so
// the defaults ride on anchored prefix regexps instead.
var defaultSlogSkipPatterns = []string{
	"^/static/",
	"^/web/",
	"^/debug/pprof/",
}

// baseHiddenSlogHeaders extends the module's built-in privacy list
// with the unprefixed flavors that CSRF-bearing frameworks ship.
var baseHiddenSlogHeaders = []string{
	"authorization",
	"cookie",
	"set-cookie",
	"x-auth-token",
	"x-csrf-token",
	"x-xsrf-token",
	"user-agent",
	"csrf-token",
	"xsrf-token",
}

// SlogParams tunes the logging.slog slot. A nil Writer selects
// os.stdout, because gin's stock writer aims at stderr and splits the
// console narrative. An empty Message selects the module's Request
// label. SkipPathPatterns appends regexps to the default skip list,
// and ExtraHiddenHeaders appends names to the hidden header list.
type SlogParams struct {
	Writer             io.Writer
	Message            string
	SkipPathPatterns   []string
	ExtraHiddenHeaders []string
}

// acceptsSlogParams reports whether the bag shapes the logging.slog
// slot expects.
func acceptsSlogParams(params any) bool {
	_, ok := params.(*SlogParams)
	return ok
}

// newSlog builds the logging.slog slot middleware. The slot sits
// inside request.id and outside resp.compress, so every record
// carries the request identifier. Recorded body sizes count whatever
// crosses this slot's writer boundary: the raw payload for now, and
// the compressed bytes once resp.compress lands inside it.
// Preflights (OPTIONS) and the default skip families produce no
// records at all.
func newSlog(params any) gin.HandlerFunc {
	cfg := SlogParams{}
	if params != nil {
		cfg = *(params.(*SlogParams))
	}
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	patterns := append([]string{}, defaultSlogSkipPatterns...)
	patterns = append(patterns, cfg.SkipPathPatterns...)
	regs := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		comp, err := regexp.Compile(p)
		if err != nil {
			// A broken pattern is a configuration defect, not a
			// request-time surprise: degrade to the module default
			// rather than refusing every request.
			continue
		}
		regs = append(regs, comp)
	}
	hidden := append([]string{}, baseHiddenSlogHeaders...)
	hidden = append(hidden, cfg.ExtraHiddenHeaders...)
	opts := []gsslog.Option{
		gsslog.WithWriter(writer),
		gsslog.WithMessage(orDefault(cfg.Message, "Request")),
		gsslog.WithSkipPathRegexps(regs...),
		gsslog.WithSkipper(isPreflight),
		gsslog.WithLogger(graftRequestID),
		gsslog.WithHiddenRequestHeaders(hidden),
	}
	return gsslog.SetLogger(opts...)
}

// isPreflight skips logging for CORS preflight exchanges, which
// settle in the cors slot before any business logic runs.
func isPreflight(c *gin.Context) bool {
	return c.Request.Method == http.MethodOptions
}

// graftRequestID stitches the request identifier onto every record,
// correlating logs with whichever slot minted or propagated the ID.
func graftRequestID(c *gin.Context, logger *slog.Logger) *slog.Logger {
	id := gsrequestid.Get(c)
	if id == "" {
		return logger
	}
	return logger.With("request_id", id)
}
