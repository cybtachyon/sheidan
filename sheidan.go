// Package sheidan provides the entry points for building web apps
// on Gin, Templ, and GORM.
package sheidan

import (
	"net/http"

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

// Bind connects a GORM model load to a templ render, producing a Gin
// handler. The load function loads the model, and the render function
// builds the templ component from it. A load error responds with status
// 500 and a JSON error body. On success, Bind renders the component
// through Wrap, so it uses status 200 and a text/html content type.
func Bind[M any](load func(c *gin.Context) (M, error), render func(M) templ.Component) gin.HandlerFunc {
	return func(c *gin.Context) {
		model, err := load(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		Wrap(render(model))(c)
	}
}
