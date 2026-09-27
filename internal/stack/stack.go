// Package stack defines the default Sheidan middleware chain as an
// ordered list of named slots and the builders that compile that chain
// into a Gin engine. Each slot has a stable dotted name, a lane, a
// fixed ordinal, a kind, and an optional parameter bag. Until a slot
// gains a factory, compiling it yields a no-op holder, so the chain
// length and ordering stay stable across releases.
package stack

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// Kind classifies how a slot participates in the request cycle.
type Kind uint8

const (
	// Guard inspects the request before c.Next() and may abort it.
	Guard Kind = iota
	// Transform prepares request data before c.Next() and lets the
	// request continue.
	Transform
	// State restores or advances per-client state before c.Next().
	State
	// Decorator augments the response after c.Next() returns.
	Decorator
	// Wrapper brackets c.Next() and reacts to the finished exchange,
	// such as timing, compression, or draining deferred effects.
	Wrapper
)

// String returns the kind's lowercase name for diagnostics.
func (k Kind) String() string {
	switch k {
	case Guard:
		return "guard"
	case Transform:
		return "transform"
	case State:
		return "state"
	case Decorator:
		return "decorator"
	case Wrapper:
		return "wrapper"
	default:
		return fmt.Sprintf("kind(%d)", uint8(k))
	}
}

// SlotSpec describes one slot of the canonical chain. Factory is nil
// while the slot awaits its release, and Accept reports whether a
// configured parameter bag has the type the eventual factory consumes.
type SlotSpec struct {
	Name    string
	Lane    string
	Ordinal int
	Kind    Kind
	Factory func(params any) gin.HandlerFunc
	Accept  func(params any) bool
}

// Slot names of the canonical chain. They are the single source of
// truth for the Specs table and for the public constants the root
// package republishes.
const (
	RequestID           = "request.id"
	LoggingSlog         = "logging.slog"
	GateMaintenance     = "gate.maintenance"
	GateBadTarget       = "gate.badiptarget"
	RespCompress        = "resp.compress"
	RespCookiefinalize  = "resp.cookiefinalize"
	ApiCors             = "api.cors"
	IntakePostsize      = "intake.postsize"
	IntakeReqHeaders    = "intake.requestheaders"
	IntakeStringTrim    = "intake.strings.trim"
	SessionRestore      = "session.restore"
	SessionAuthenticate = "session.authenticate"
	SafetyCsrf          = "safety.csrf"
	SafetySignature     = "safety.signature"
	PolicyRateLimit     = "policy.rate_limit"
	PolicyAuthorize     = "policy.authorization"
	LangLocale          = "lang.localized"
	BindingParamSubst   = "binding.params.subst"
	SessionFlashError   = "session.flash_errors"
	CacheResponse       = "cache.response"
	RespTimeout         = "resp.timeout"
)

// Specs holds the canonical chain in ordinal order. It is the single
// source of truth for every compiled engine and group filter.
var Specs = []SlotSpec{
	{Name: RequestID, Ordinal: 1, Kind: Wrapper, Factory: newRequestID, Accept: acceptsRequestIDParams},
	{Name: LoggingSlog, Ordinal: 2, Kind: Wrapper, Factory: newSlog, Accept: acceptsSlogParams},
	{Name: GateMaintenance, Ordinal: 3, Kind: Guard, Factory: newMaintenance, Accept: acceptsMaintenanceParams},
	{Name: GateBadTarget, Ordinal: 4, Kind: Guard, Factory: newBadTarget, Accept: acceptsBadTargetParams},
	{Name: RespCompress, Ordinal: 5, Kind: Wrapper},
	{Name: RespCookiefinalize, Ordinal: 6, Kind: Wrapper},
	{Name: ApiCors, Ordinal: 7, Kind: Guard},
	{Name: IntakePostsize, Ordinal: 8, Kind: Guard},
	{Name: IntakeReqHeaders, Ordinal: 9, Kind: Guard},
	{Name: IntakeStringTrim, Ordinal: 10, Kind: Transform},
	{Name: SessionRestore, Ordinal: 11, Kind: State},
	{Name: SessionAuthenticate, Ordinal: 12, Kind: Guard},
	{Name: SafetyCsrf, Ordinal: 13, Kind: Guard},
	{Name: SafetySignature, Ordinal: 14, Kind: Guard},
	{Name: PolicyRateLimit, Ordinal: 15, Kind: Guard},
	{Name: PolicyAuthorize, Ordinal: 16, Kind: Guard},
	{Name: LangLocale, Ordinal: 17, Kind: Decorator},
	{Name: BindingParamSubst, Ordinal: 18, Kind: Transform},
	{Name: SessionFlashError, Ordinal: 19, Kind: Decorator},
	{Name: CacheResponse, Ordinal: 20, Kind: Decorator},
	{Name: RespTimeout, Ordinal: 21, Kind: Wrapper},
}

func init() {
	// Derive each lane from the first segment of the slot name, so
	// the name carries the sole source of truth.
	for i := range Specs {
		Specs[i].Lane = strings.SplitN(Specs[i].Name, ".", 2)[0]
	}
}

// View is the observable state of one chain entry, for tests and
// diagnostics. Disabled entries appear too, so order claims stay
// checkable.
type View struct {
	Name    string
	Lane    string
	Ordinal int
	Kind    Kind
	Filled  bool
	Custom  bool
	Enabled bool
}

// entry is one positioned element of a built chain. A nil spec marks a
// custom middleware inserted with Insert.
type entry struct {
	spec    *SlotSpec
	name    string
	fn      gin.HandlerFunc
	params  any
	enabled bool
}

// Builder is a mutable copy of the canonical chain. Operations are
// recorded immediately, but unknown names and mismatched parameter
// bags surface as errors when the chain compiles, so a misnamed call
// is an error at build time, not a silent no-op. Builders are not
// safe for concurrent use; adjust one during bootstrap only.
type Builder struct {
	entries   []entry
	faults    []string
	nextIndex int
}

// NewBuilder returns a builder holding a full copy of the canonical
// chain, every slot enabled with no parameters.
func NewBuilder() *Builder {
	b := &Builder{}
	for i := range Specs {
		b.entries = append(b.entries, entry{spec: &Specs[i], name: Specs[i].Name, enabled: true})
	}
	return b
}

// Index returns the position of the entry with the given name, or -1.
func (b *Builder) Index(name string) int {
	for i := range b.entries {
		if b.entries[i].name == name {
			return i
		}
	}
	return -1
}

// Enable turns the named slot back on.
func (b *Builder) Enable(name string) {
	i := b.Index(name)
	if i < 0 {
		b.faults = append(b.faults, fmt.Sprintf("unknown slot %q", name))
		return
	}
	b.entries[i].enabled = true
}

// Disable removes the named slot from the compiled chain. Neighbors
// keep their positions.
func (b *Builder) Disable(name string) {
	i := b.Index(name)
	if i < 0 {
		b.faults = append(b.faults, fmt.Sprintf("unknown slot %q", name))
		return
	}
	b.entries[i].enabled = false
}

// Configure stores the parameter bag for the named slot. The bag is
// type-checked against the slot's expectations at compile time.
func (b *Builder) Configure(name string, params any) {
	i := b.Index(name)
	if i < 0 {
		b.faults = append(b.faults, fmt.Sprintf("unknown slot %q", name))
		return
	}
	b.entries[i].params = params
}

// Insert places a custom handler before or after the named slot. The
// position string is "before:<name>" or "after:<name>"; anything else
// breaks the build. Custom entries gain synthetic names, custom.1,
// custom.2, and so on, usable by later operations.
func (b *Builder) Insert(position string, fn gin.HandlerFunc) {
	side, target, ok := strings.Cut(position, ":")
	if !ok || (side != "before" && side != "after") {
		b.faults = append(b.faults, fmt.Sprintf("invalid insert position %q; want before:<name> or after:<name>", position))
		return
	}
	i := b.Index(target)
	if i < 0 {
		b.faults = append(b.faults, fmt.Sprintf("unknown slot %q", target))
		return
	}
	b.nextIndex++
	at := i
	if side == "after" {
		at++
	}
	newEntry := entry{name: fmt.Sprintf("custom.%d", b.nextIndex), fn: fn, enabled: true}
	// Copy the tail first: an in-place append would let the new
	// element land where the tail still begins, corrupting the shift.
	tail := make([]entry, len(b.entries)-at)
	copy(tail, b.entries[at:])
	b.entries = append(append(b.entries[:at], newEntry), tail...)
}

// Remove detaches the named slot or custom entry from the chain.
func (b *Builder) Remove(name string) {
	i := b.Index(name)
	if i < 0 {
		b.faults = append(b.faults, fmt.Sprintf("unknown slot %q", name))
		return
	}
	b.entries = append(b.entries[:i], b.entries[i+1:]...)
}

// Views reports the chain state in compiled order, including disabled
// entries, so tests can verify neighbor survival.
func (b *Builder) Views() []View {
	vs := make([]View, len(b.entries))
	for i, e := range b.entries {
		v := View{Name: e.name, Enabled: e.enabled}
		if e.spec != nil {
			v.Lane = e.spec.Lane
			v.Ordinal = e.spec.Ordinal
			v.Kind = e.spec.Kind
			v.Filled = e.spec.Factory != nil
		} else {
			v.Custom = true
		}
		vs[i] = v
	}
	return vs
}

// compile assembles the enabled entries in order. Faults collected by
// earlier operations surface here as one joined error, so a faulty
// build is refused wholesale rather than partially applied.
func (b *Builder) Compile() ([]gin.HandlerFunc, error) {
	for _, e := range b.entries {
		if !e.enabled || e.spec == nil || e.spec.Factory == nil || e.params == nil {
			continue
		}
		if e.spec.Accept == nil || !e.spec.Accept(e.params) {
			b.faults = append(b.faults, fmt.Sprintf("slot %s: parameter bag of type %T does not match", e.name, e.params))
		}
	}
	if len(b.faults) > 0 {
		return nil, fmt.Errorf("broken chain: %s", strings.Join(b.faults, "; "))
	}
	hs := make([]gin.HandlerFunc, 0, len(b.entries))
	for _, e := range b.entries {
		if !e.enabled {
			continue
		}
		if e.fn != nil {
			hs = append(hs, e.fn)
			continue
		}
		if e.spec != nil && e.spec.Factory != nil {
			hs = append(hs, e.spec.Factory(e.params))
			continue
		}
		hs = append(hs, Holder)
	}
	return hs, nil
}

// Holder is the no-op middleware emitted for slots that have not been
// filled yet, keeping chain length and ordering stable.
var Holder = func(c *gin.Context) { c.Next() }
