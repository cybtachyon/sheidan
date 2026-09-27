package stack

import (
	"fmt"
	"io"
	"net/http"

	"github.com/cybtachyon/sheidan/internal/bodycap"
	"github.com/gin-gonic/gin"
)

// drainBudget bounds how much of an offending body the slot pumps
// away before abandoning the connection. Draining more trades
// bandwidth for keep-alive courtesy; draining less lets the pool
// recycle sooner.
const drainBudget = 64 << 10

// PostSizeParams tunes the intake.postsize slot. Zero selections
// recover the shipped default: a one megabyte ceiling on every body
// with no per-route relief. Rules pair a path prefix with a relaxed
// ceiling, longest prefix winning, so upload routes absorb more
// than the default.
type PostSizeParams struct {
	DefaultLimit int64
	Rules        []PostSizeRule
}

// PostSizeRule pairs a path prefix with the ceiling it imposes.
type PostSizeRule struct {
	Prefix string
	Limit  int64
}

// acceptsPostSizeParams reports whether the bag shapes the
// intake.postsize slot expects.
func acceptsPostSizeParams(params any) bool {
	_, ok := params.(*PostSizeParams)
	return ok
}

// politeDrain pumps a bounded stretch of an offending body into the
// void, improving connection reuse without courting a flood.
func politeDrain(reader io.Reader) {
	io.Copy(io.Discard, io.LimitReader(reader, drainBudget))
}

// oversizeAnswers the 413 settlement, naming the effective ceiling
// so clients can adapt their transfers.
func oversizeAnswer(c *gin.Context, limit int64) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge,
		gin.H{"error": fmt.Sprintf("payload exceeds the %d byte cap", limit), "limit_bytes": limit})
}

// newPostSize builds the intake.postsize slot middleware. Bodies with
// a declared length that fits the ceiling pass unmetered, because the
// server enforces the declared length anyway; oversized declarations
// die before spending handler work. Undeclared lengths (chunked
// transfers) run through a metering wrapper that truncates at the
// ceiling, and a handler that balks before committing a response gets
// the honest 413 instead of a decoder-error 400 or 500. Requests
// wearing the intake-bypass stamp skip the guard, so maintenance
// probes never trip it.
func newPostSize(params any) gin.HandlerFunc {
	table := bodycap.Table{}
	if params != nil {
		bag := *(params.(*PostSizeParams))
		table.Default = bag.DefaultLimit
		for _, rule := range bag.Rules {
			table.Rules = append(table.Rules, bodycap.Rule{Prefix: rule.Prefix, Limit: rule.Limit})
		}
	}
	table = table.Normalize()
	return func(c *gin.Context) {
		if c.GetBool(intakeBypass) {
			c.Next()
			return
		}
		request := c.Request
		if request.Body == nil || request.ContentLength == 0 {
			c.Next()
			return
		}
		path := c.FullPath()
		if path == "" {
			path = request.URL.Path
		}
		limit := table.LimitFor(path)
		if clen := request.ContentLength; clen >= 0 {
			if clen > limit {
				politeDrain(request.Body)
				oversizeAnswer(c, limit)
				return
			}
			c.Next()
			return
		}
		metered := bodycap.NewReader(request.Body, limit)
		request.Body = metered
		c.Next()
		if metered.Over() && !c.Writer.Written() {
			politeDrain(metered)
			oversizeAnswer(c, limit)
		}
	}
}
