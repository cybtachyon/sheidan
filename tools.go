//go:build tools

// Package tools pins runtime and toolchain dependencies that application
// code does not import yet, keeping their versions in go.mod.
package tools

import (
	_ "github.com/gopherjs/gopherjs/js"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "gorm.io/driver/sqlite"
	_ "gorm.io/gorm"
)
