# Sheidan

Sheidan is a Go web framework built on Gin, Templ, and GORM. The full
stack is written in Go. Front-end components are Go compiled to
JavaScript by GopherJS, following an MVVM pattern: a Go model (GORM), a
view model (routing and data-binding), and a view (templ templates plus
GopherJS observation).

## Usage

A Sheidan app is a Go module that requires `github.com/cybtachyon/sheidan`:

```go
package main

import (
	"log"

	"github.com/cybtachyon/sheidan"
)

func main() {
	engine := sheidan.New()
	engine.GET("/", sheidan.Wrap(Home()))
	if err := engine.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
```

`New` creates a preconfigured Gin engine, and `Wrap` adapts a templ
component to a Gin handler. See [test/hello](test/hello) for a complete
example.

## Web client

A Sheidan app's front-end is a GopherJS client: a Go module compiled to a
single JavaScript file that the app serves to the browser. The build is
self-bootstrapping. Running the app provisions the GopherJS toolchain on
first use and transpiles the client, so the whole flow is `go get` +
import + `go run`. First run takes a few minutes; later runs are instant.
Front-end developers can transpile the client alone with `sheidan web`, a
fast transpile-only build.

## Database

Sheidan uses the GORM SQLite driver and treats [Turso](https://turso.tech)
as a first-class database. `db.Open` handles local (`file:`) and remote
(`libsql://`, `http(s)://`, `ws(s)://`) DSNs.
