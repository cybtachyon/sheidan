package stack

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// defaultHeaderCaps sizes the intake.requestheaders slot for browsers
// and mobile agents: fifty header lines, eight kibibytes per value,
// and thirty-two kibibytes for the whole block.
const (
	defaultMaxHeaders    = 50
	defaultMaxValueBytes = 8 << 10
	defaultMaxTotalBytes = 32 << 10
)

// RequestHeaderParams tunes the intake.requestheaders slot. Zero
// selections recover the shipped defaults: the browser-sized caps, and
// the control-sequence ban engaged. BanControlChars is a pointer so a
// nil bag keeps the ban on while an explicit false turns it off.
type RequestHeaderParams struct {
	MaxHeaders      int
	MaxValueBytes   int
	MaxTotalBytes   int
	BanControlChars *bool
}

// acceptsRequestHeaderParams reports whether the bag shapes the
// intake.requestheaders slot expects.
func acceptsRequestHeaderParams(params any) bool {
	_, ok := params.(*RequestHeaderParams)
	return ok
}

// measureHeaders reports the header block as transmitted: the number
// of header lines, the total byte count (keys plus values plus the
// ": " and CRLF framing each line carries on the wire), and the
// longest single value.
func measureHeaders(header http.Header) (lines, total, longestValue int) {
	for key, values := range header {
		for _, value := range values {
			lines++
			total += len(key) + 2 + len(value) + 2
			if len(value) > longestValue {
				longestValue = len(value)
			}
		}
	}
	return
}

// headerViolation names the first breached dimension, or an empty
// string when the block is within every cap. The dimension string is
// what the 431 body carries, so an operator can tell a flood (too many
// lines) from a fat cookie (one oversized value).
func headerViolation(header http.Header, maxLines, maxValue, maxTotal int) string {
	lines, total, longestValue := measureHeaders(header)
	switch {
	case lines > maxLines:
		return fmt.Sprintf("request carries %d header lines; the cap is %d", lines, maxLines)
	case longestValue > maxValue:
		return fmt.Sprintf("a header value is %d bytes; the cap is %d", longestValue, maxValue)
	case total > maxTotal:
		return fmt.Sprintf("the header block is %d bytes; the cap is %d", total, maxTotal)
	}
	return ""
}

// newRequestHeaders builds the intake.requestheaders slot middleware.
// It bounds the header block against flooding and smuggling and answers
// 431 naming the breached dimension. Requests wearing the
// intake-bypass stamp skip the guard, so a maintenance probe never
// trips it. The control-sequence ban is belt-and-braces: Go's http
// server already rejects CR/LF in values at parse time, but the check
// stays for configurations that loosen that behavior.
func newRequestHeaders(params any) gin.HandlerFunc {
	cfg := RequestHeaderParams{}
	if params != nil {
		cfg = *(params.(*RequestHeaderParams))
	}
	maxLines := cfg.MaxHeaders
	if maxLines <= 0 {
		maxLines = defaultMaxHeaders
	}
	maxValue := cfg.MaxValueBytes
	if maxValue <= 0 {
		maxValue = defaultMaxValueBytes
	}
	maxTotal := cfg.MaxTotalBytes
	if maxTotal <= 0 {
		maxTotal = defaultMaxTotalBytes
	}
	ban := true
	if cfg.BanControlChars != nil {
		ban = *cfg.BanControlChars
	}
	return func(c *gin.Context) {
		if c.GetBool(intakeBypass) {
			c.Next()
			return
		}
		header := c.Request.Header
		if reason := headerViolation(header, maxLines, maxValue, maxTotal); reason != "" {
			c.AbortWithStatusJSON(http.StatusRequestHeaderFieldsTooLarge,
				gin.H{"error": reason})
			return
		}
		if ban {
			for key, values := range header {
				for _, value := range values {
					if strings.ContainsAny(value, "\r\n") {
						c.AbortWithStatusJSON(http.StatusRequestHeaderFieldsTooLarge,
							gin.H{"error": fmt.Sprintf("header %q carries a control sequence", key)})
						return
					}
				}
			}
		}
		c.Next()
	}
}
