// The sheidan binary serves a Hello World page rendered by a templ component.
package main

import (
	"log"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

func main() {
	engine := gin.Default()
	// Disable trusted proxy parsing for the local dev server.
	if err := engine.SetTrustedProxies(nil); err != nil {
		log.Fatal(err)
	}
	engine.GET("/", gin.WrapH(templ.Handler(Home())))
	if err := engine.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
