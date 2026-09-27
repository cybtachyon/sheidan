package stack

import "github.com/gin-gonic/gin"

// Chain is a per-group filtered copy of the default chain. Groups take
// the current chain minus the excluded slots, so an exclusion silences
// a slot on that branch only; later groups built from the same base
// keep the original slots. Wrappers excluded from a branch lose their
// bracketing effect on that branch only.
type Chain struct {
	Base    *gin.Engine
	builder *Builder
}

// NewChain copies the stock default chain for one router group. Base
// is the engine the group descends from, kept for provenance and
// diagnostics.
func NewChain(base *gin.Engine) *Chain {
	return &Chain{Base: base, builder: NewBuilder()}
}

// Without excludes the named slots from this branch's copy. Unknown
// names are reported by Handlers.
func (c *Chain) Without(names ...string) *Chain {
	for _, name := range names {
		c.builder.Disable(name)
	}
	return c
}

// Handlers compiles this branch's middleware list for use with
// RouterGroup.Use, surfacing unknown names as an error.
func (c *Chain) Handlers() ([]gin.HandlerFunc, error) {
	return c.builder.Compile()
}
