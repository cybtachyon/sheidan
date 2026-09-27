package stack

import (
	"log"
	"net/http"
	"strings"
	"time"

	ccors "github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// defaultCorsMethods is the REST set the preflight advertisement
// starts from.
var defaultCorsMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPatch,
	http.MethodDelete,
}

// defaultCorsHeaders names the non-simple headers browsers need for
// authenticated JSON exchanges.
var defaultCorsHeaders = []string{"Authorization", "Content-Type"}

// acceptsCorsParams reports whether the bag shapes the api.cors slot
// expects, and refuses the one combination the library validates only
// in the browser: a wildcard grant paired with credentials. The
// refusal lands at build time, where it hurts, instead of at request
// time, where it confuses.
func acceptsCorsParams(params any) bool {
	var cfg ccors.Config
	switch p := params.(type) {
	case ccors.Config:
		cfg = p
	case *ccors.Config:
		cfg = *p
	default:
		return false
	}
	if !cfg.AllowCredentials {
		return true
	}
	if cfg.AllowAllOrigins {
		return false
	}
	for _, origin := range cfg.AllowOrigins {
		if strings.Contains(origin, "*") {
			return false
		}
	}
	return true
}

// hasOriginSelector reports whether the config names any way to
// admit an origin at all.
func hasOriginSelector(cfg ccors.Config) bool {
	return cfg.AllowAllOrigins ||
		cfg.AllowOriginFunc != nil ||
		cfg.AllowOriginWithContextFunc != nil ||
		len(cfg.AllowOrigins) > 0
}

// newCors builds the api.cors slot middleware. The Sheidan default
// grants no origins at all: same-origin traffic sails through
// invisibly, and cross-origin traffic meets 403 until an app
// configures grants. The library cannot express an empty grant, so
// the closed default rides an explicitly denying origin function.
// Configurations the library rejects outright (selector conflicts)
// demote to the closed posture with a loud error rather than
// panicking the bootstrap.
func newCors(params any) gin.HandlerFunc {
	cfg := ccors.Config{}
	if params != nil {
		switch p := params.(type) {
		case ccors.Config:
			cfg = p
		case *ccors.Config:
			cfg = *p
		}
	}
	if !hasOriginSelector(cfg) {
		cfg.AllowOriginFunc = func(string) bool { return false }
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = defaultCorsMethods
	}
	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = defaultCorsHeaders
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = time.Hour
	}
	// A wildcard grant plus credentials is a contradiction browsers
	// refuse; accept already bars the pair at build time, so reaching
	// this point means somebody routed around the builder. Strip the
	// credentials and shout about it.
	if cfg.AllowCredentials && (cfg.AllowAllOrigins || strings.Contains(strings.Join(cfg.AllowOrigins, ","), "*")) {
		log.Printf("ERROR: api.cors grants a wildcard origin with credentials enabled; credentials stripped, browsers would refuse the pairing")
		cfg.AllowCredentials = false
	}
	if err := cfg.Validate(); err != nil {
		log.Printf("ERROR: api.cors configuration is conflicted (%v); the slot compiles closed", err)
		cfg = ccors.Config{AllowOriginFunc: func(string) bool { return false }}
		cfg.AllowMethods = defaultCorsMethods
		cfg.AllowHeaders = defaultCorsHeaders
		cfg.MaxAge = time.Hour
	}
	return ccors.New(cfg)
}
