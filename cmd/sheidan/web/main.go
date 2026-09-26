// The web program is the demo app's browser client. The GopherJS
// compiler transpiles it to JavaScript, and the demo app serves the
// output at /web/web.js.
package main

import (
	"github.com/gopherjs/gopherjs/js"
)

// main initializes the editable fields on the page. It runs when the
// page loads, and it is a no-op outside a browser, so the same build
// can run under Node.js for tests.
func main() {
	doc := js.Global.Get("document")
	if doc == js.Undefined {
		return
	}
	list := doc.Call("querySelectorAll", "[data-field]")
	for i := 0; i < list.Length(); i++ {
		NewField(list.Index(i)).init()
	}
}
