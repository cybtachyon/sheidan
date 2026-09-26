// The hello command serves a Hello World web app built on the
// Sheidan framework. It tests that the sheidan module works as a
// dependency of an external app.
package main

import (
	"log"

	"github.com/cybtachyon/sheidan"
)

func main() {
	engine := sheidan.New()
	engine.GET("/", sheidan.Wrap(Hello()))
	if err := engine.Run(":8081"); err != nil {
		log.Fatal(err)
	}
}
