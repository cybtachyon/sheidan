package stack

import (
	"github.com/gin-gonic/gin"

	gsrequestid "github.com/gin-contrib/requestid"
)

// RequestIDParams tunes the request.id slot. A nil Generator selects
// the module's UUID generator, and an empty HeaderKey selects the
// standard X-Request-ID header. Renaming the header breaks
// interoperability with tooling that expects the standard name, so
// treat it as an expert knob.
type RequestIDParams struct {
	Generator func() string
	HeaderKey string
}

// acceptsRequestIDParams reports whether the bag shapes the request.id
// slot expects.
func acceptsRequestIDParams(params any) bool {
	_, ok := params.(*RequestIDParams)
	return ok
}

// newRequestID builds the request.id slot middleware. Caller-supplied
// identifiers propagate unchanged, and generated ones are stamped onto
// both the request and the response, so every later consumer, from
// the slog records to the timeout responses, correlates on the same
// value.
func newRequestID(params any) gin.HandlerFunc {
	cfg := RequestIDParams{}
	if params != nil {
		cfg = *(params.(*RequestIDParams))
	}
	opts := []gsrequestid.Option{
		gsrequestid.WithCustomHeaderStrKey(gsrequestid.HeaderStrKey(orDefault(cfg.HeaderKey, "X-Request-ID"))),
	}
	if cfg.Generator != nil {
		opts = append(opts, gsrequestid.WithGenerator(cfg.Generator))
	}
	return gsrequestid.New(opts...)
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
