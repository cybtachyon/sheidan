//go:build tools

// Package tools pins toolchain dependencies that application code does
// not import, keeping their versions in go.mod.
//
// The gopherjs pin tracks master, not a release. GopherJS 1.21 compiles
// only a Go 1.21 GOROOT, so webbuild downloads a Go 1.21 SDK when the
// user's Go is newer. If a future release accepts a modern GOROOT, the
// SDK download disappears and the setup collapses to `go install
// github.com/gopherjs/gopherjs@vX` with the user's toolchain. Bump this
// pin to a release and re-test on each GopherJS release.
package tools

import (
	_ "github.com/gopherjs/gopherjs/js"
)
