// The web program is the demo app's browser client. The GopherJS
// compiler transpiles it to JavaScript, and the demo app serves the
// output at /web/web.js.
package main

import "github.com/gopherjs/gopherjs/js"

// main boots the reactive app when the page has its app container,
// and is a no-op otherwise, so the transpiled build can run under
// Node.js for tests.
func main() {
	doc := js.Global.Get("document")
	if doc == nil || doc == js.Undefined {
		return
	}
	container := doc.Call("querySelector", "#app")
	if container == nil {
		return
	}
	NewApp(doc, container).run()
}
