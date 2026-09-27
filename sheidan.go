// Package sheidan provides the entry points for building web apps
// on Gin, Templ, and GORM.
package sheidan

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// New creates a Gin engine preconfigured for a Sheidan app: the
// default middleware chain assembled behind the recovery middleware,
// with trusted proxy parsing disabled so ClientIP reports the direct
// peer address. Adjust the chain for a bespoke engine through Stack.
func New() *gin.Engine {
	engine, err := Stack().Apply()
	if err != nil {
		// The stock chain cannot fault; reaching here is a defect
		// in this package, not a caller mistake.
		panic(err)
	}
	return engine
}

// Wrap adapts a templ component to a Gin handler that renders it
// with status 200 and a text/html content type.
func Wrap(component templ.Component) gin.HandlerFunc {
	return gin.WrapH(templ.Handler(component))
}

// Bind connects a GORM model load to a templ render, producing a Gin
// handler. The load function loads the model, and the render function
// builds the templ component from it. On success, Bind renders the
// component through Wrap, so it uses status 200 and a text/html
// content type. A load error responds with a JSON error body: GORM's
// ErrRecordNotFound maps to status 404, because a missing resource is
// not a server failure, and every other load error maps to status 500.
func Bind[M any](load func(c *gin.Context) (M, error), render func(M) templ.Component) gin.HandlerFunc {
	return func(c *gin.Context) {
		model, err := load(c)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, gorm.ErrRecordNotFound) {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		Wrap(render(model))(c)
	}
}
