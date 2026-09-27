package stack

import (
	"bytes"
	"io"
	"strings"

	"github.com/cybtachyon/sheidan/internal/trim"
	"github.com/gin-gonic/gin"
)

// rawBodyKey is the context key under which the slot parks the
// original body bytes, so an auditor can re-read the request exactly
// as it arrived even after the slot has swapped in a cleaned copy.
const rawBodyKey = "sheidan:rawbody"

// StringTrimParams tunes the intake.strings.trim slot. Zero
// selections recover the shipped defaults: the standard excluded field
// names and the recursion depth bound.
type StringTrimParams struct {
	Exclude  []string
	MaxDepth int
}

// acceptsStringTrimParams reports whether the bag shapes the
// intake.strings.trim slot expects.
func acceptsStringTrimParams(params any) bool {
	_, ok := params.(*StringTrimParams)
	return ok
}

// isJSONContentType reports whether the content type names a JSON
// media type, including the structured suffix form.
func isJSONContentType(contentType string) bool {
	media := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	return media == "application/json" || strings.HasSuffix(media, "+json")
}

// isFormContentType reports whether the content type names a
// urlencoded form body.
func isFormContentType(contentType string) bool {
	media := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	return media == "application/x-www-form-urlencoded"
}

// transformJSONBody rewrites a JSON body with its string values
// trimmed, parking the raw original in the context and swapping the
// request body to the cleaned copy. A body that is not valid JSON is
// restored untouched.
func transformJSONBody(c *gin.Context, exclude map[string]bool, maxDepth int) {
	req := c.Request
	raw, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return
	}
	if len(raw) == 0 {
		req.Body = io.NopCloser(bytes.NewReader(nil))
		return
	}
	transformed, kept, ok := trim.JSON(raw, exclude, maxDepth)
	if !ok {
		req.Body = io.NopCloser(bytes.NewReader(raw))
		return
	}
	c.Set(rawBodyKey, kept)
	req.Body = io.NopCloser(bytes.NewReader(transformed))
	req.ContentLength = int64(len(transformed))
}

// transformFormBody parses a urlencoded form body, trims its string
// fields in place, and restores the raw original so the body stays
// readable once for auditors.
func transformFormBody(c *gin.Context, exclude map[string]bool) {
	req := c.Request
	raw, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return
	}
	c.Set(rawBodyKey, raw)
	req.Body = io.NopCloser(bytes.NewReader(raw))
	if err := req.ParseForm(); err != nil {
		return
	}
	trim.Form(req.PostForm, exclude)
	trim.Form(req.Form, exclude)
	req.Body = io.NopCloser(bytes.NewReader(raw))
}

// newStringTrim builds the intake.strings.trim slot middleware. It is
// the last transform before state restoration, so every downstream
// binder sees cleaned values. It trims string leaves of a JSON body,
// the fields of a urlencoded form, and the query parameters, leaving
// excluded fields and the raw body intact.
func newStringTrim(params any) gin.HandlerFunc {
	cfg := trim.Params{}
	if params != nil {
		bag := params.(*StringTrimParams)
		cfg = trim.Params{Exclude: bag.Exclude, MaxDepth: bag.MaxDepth}
	}
	cfg = cfg.Normalize()
	exclude := trim.ExcludedSet(cfg.Exclude)
	return func(c *gin.Context) {
		req := c.Request
		contentType := req.Header.Get("Content-Type")
		switch {
		case isJSONContentType(contentType) && req.Body != nil:
			transformJSONBody(c, exclude, cfg.MaxDepth)
		case isFormContentType(contentType) && req.Body != nil:
			transformFormBody(c, exclude)
		}
		if req.URL.RawQuery != "" {
			req.URL.RawQuery = trim.Query(req.URL.RawQuery, exclude)
		}
		c.Next()
	}
}
