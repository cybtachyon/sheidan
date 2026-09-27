package sheidan

import (
	"github.com/cybtachyon/sheidan/internal/stack"
	ccors "github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Slot names of the default chain. Applications reference these
// constants instead of retyping the dotted names, so renames surface
// as compile errors.
const (
	RequestID          = stack.RequestID
	LoggingSlog        = stack.LoggingSlog
	GateMaintenance    = stack.GateMaintenance
	GateBadTarget      = stack.GateBadTarget
	RespCompress       = stack.RespCompress
	RespCookiefinalize = stack.RespCookiefinalize
	ApiCors            = stack.ApiCors
	IntakePostsize     = stack.IntakePostsize
	IntakeReqHeaders   = stack.IntakeReqHeaders
	IntakeStringTrim   = stack.IntakeStringTrim
	SessionRestore     = stack.SessionRestore
	SessionAuth        = stack.SessionAuthenticate
	SafetyCsrf         = stack.SafetyCsrf
	SafetySignature    = stack.SafetySignature
	PolicyRateLimit    = stack.PolicyRateLimit
	PolicyAuthorize    = stack.PolicyAuthorize
	LangLocale         = stack.LangLocale
	BindingParamSubst  = stack.BindingParamSubst
	SessionFlashError  = stack.SessionFlashError
	CacheResponse      = stack.CacheResponse
	RespTimeout        = stack.RespTimeout
)

// StackBuilder is a mutable builder over the default middleware chain. It
// composes an engine without copying the chain by hand: Enable,
// Disable, Configure, Insert, and Remove express the adjustments, and
// Apply compiles the result. Construction starts from a fresh copy of
// the canonical chain with permissive defaults, so adjustments never
// disturb sheidan.New. Unknown names and mismatched parameter bags
// surface as errors from Apply. Use one builder during bootstrap only,
// not across goroutines.
type StackBuilder struct {
	builder *stack.Builder
}

// Stack returns a builder preloaded with the full default chain,
// every slot enabled with no parameters.
func Stack() *StackBuilder {
	return &StackBuilder{builder: stack.NewBuilder()}
}

// Enable turns the named slot back on.
func (s *StackBuilder) Enable(name string) { s.builder.Enable(name) }

// Disable removes the named slot from the compiled chain.
func (s *StackBuilder) Disable(name string) { s.builder.Disable(name) }

// Configure stores the parameter bag for the named slot.
func (s *StackBuilder) Configure(name string, params any) { s.builder.Configure(name, params) }

// Insert places a custom handler before or after the named slot,
// through a position string shaped "before:<name>" or "after:<name>".
func (s *StackBuilder) Insert(position string, fn gin.HandlerFunc) { s.builder.Insert(position, fn) }

// Remove detaches the named slot or previously inserted handler.
func (s *StackBuilder) Remove(name string) { s.builder.Remove(name) }

// Views reports the current chain state, including disabled slots, in
// compiled order.
func (s *StackBuilder) Views() []stack.View { return s.builder.Views() }

// Apply compiles the chain into a Gin engine: a guarded recovery
// leads the slots, trusted proxy parsing stays disabled, and the
// slots run in canonical order. The recovery records panics through
// the logging slot's writer, so a crash leaves the same structured
// trail as a graded 5xx. Compilation refuses the build when an
// operation named an unknown slot or a mismatched parameter bag.
func (s *StackBuilder) Apply() (*gin.Engine, error) {
	return s.builder.Assemble()
}

// ChainFilter is a per-router-group filtered copy of the default chain.
// Each group starts from the stock defaults and names the slots that
// never run on that branch through Without. Handlers compiles the
// survivors for use with RouterGroup.Use. Copies are independent:
// excluding a slot in one group leaves it armed in every other, and
// wrappers excluded from a branch lose their bracketing effect on
// that branch only.
type ChainFilter struct {
	group *stack.Chain
}

// Chain returns a group filter rooted at the stock default chain.
// Passing a nil engine panics, because a group without an owning
// engine has no provenance.
func Chain(base *gin.Engine) *ChainFilter {
	if base == nil {
		panic("sheidan: Chain requires a non-nil engine")
	}
	return &ChainFilter{group: stack.NewChain(base)}
}

// Without excludes the named slots from this group's copy.
func (c *ChainFilter) Without(names ...string) *ChainFilter {
	c.group.Without(names...)
	return c
}

// Handlers compiles the group's middleware list, surfacing unknown
// slot names as an error.
func (c *ChainFilter) Handlers() ([]gin.HandlerFunc, error) {
	return c.group.Handlers()
}

// Parameter bag types for the filled slots. They are aliases for the
// internal stack's bag types, so a Stack().Configure call and the
// slot's factory agree on one shape.
type (
	RequestIDParams   = stack.RequestIDParams
	SlogParams        = stack.SlogParams
	MaintenanceParams = stack.MaintenanceParams
	BadTargetParams   = stack.BadTargetParams
	CorsOptions       = ccors.Config
)
