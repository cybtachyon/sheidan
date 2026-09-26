// Package sheidan provides the entry points for building web apps
// on Gin, Templ, and GORM.
package sheidan

import (
	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

// New creates a Gin engine preconfigured for a Sheidan app: Logger
// and Recovery middleware, and trusted proxy parsing disabled, so
// ClientIP reports the direct peer address.
func New() *gin.Engine {
	engine := gin.Default()
	// A nil list returns without an error, so the discard is safe.
	_ = engine.SetTrustedProxies(nil)
	return engine
}

// Wrap adapts a templ component to a Gin handler that renders it
// with status 200 and a text/html content type.
func Wrap(component templ.Component) gin.HandlerFunc {
	return gin.WrapH(templ.Handler(component))
}
