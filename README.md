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

## Notes demo

The demo app in [cmd/sheidan](cmd/sheidan) shows the full stack: a
templ-rendered notes list and note detail page, GORM-backed create,
update, and delete endpoints, and a GopherJS client that makes the
fields inline editable, adds a new-note form, and adds a delete button.
Run it with:

```
go run ./cmd/sheidan
```

Then open http://localhost:8080/notes. Each note's title links to its
detail page, and a back link on the detail page returns to the list.
Hover a field for its edit and delete buttons. The first run
provisions the GopherJS toolchain, so it takes a few minutes; later
runs start in seconds. The app stores notes in `data/sheidan.db` by
default, or in the DSN named by the `SHEIDAN_DB` environment variable.

## Database

Sheidan uses the GORM SQLite driver and treats [Turso](https://turso.tech)
as a first-class database. `db.Open` handles local (`file:`) and remote
(`libsql://`, `http(s)://`, `ws(s)://`) DSNs.
