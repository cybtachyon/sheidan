// Package webbuild bootstraps the GopherJS toolchain and transpiles
// Sheidan web clients to JavaScript.
//
// A Sheidan web client is a Go module that GopherJS compiles to a
// single JavaScript file, which the app serves to the browser. webbuild
// makes that build self-bootstrapping: the first call that needs a
// rebuild provisions the GopherJS toolchain (a Go 1.21 SDK and a
// gopherjs CLI), and later calls transpile only the client.
//
// The bootstrap is opt-in and overridable. An app that does not use a
// Sheidan front-end never calls webbuild and pays no cost. An app with
// its own GopherJS client calls Build or Test with its client directory
// and output path. The GOROOT the client compiles against is taken from
// the GOPHERJS_GOROOT environment variable when set, otherwise from the
// provisioned toolchain.
package webbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Build transpiles the GopherJS client module in dir to the JavaScript
// file out. It rebuilds only when out is missing or older than the
// client sources, so a steady-state call is a fast transpile. The first
// call that needs a rebuild provisions the GopherJS toolchain, which
// takes a few minutes.
//
// dir is the client module's directory (it must contain a go.mod). out
// is the output JavaScript file; its directory is created if missing.
func Build(dir, out string) error {
	stale, err := isStale(dir, out)
	if err != nil {
		return err
	}
	if !stale {
		return nil
	}
	t, err := ensureToolchain(dir)
	if err != nil {
		return err
	}
	if err := generateTempl(dir); err != nil {
		return err
	}
	return transpile(dir, out, t)
}

// Test runs the GopherJS client module's Go tests with the gopherjs
// test command, provisioning the toolchain on first use.
func Test(dir string) error {
	t, err := ensureToolchain(dir)
	if err != nil {
		return err
	}
	return runGopherjs(t, dir, "test")
}

// isStale reports whether out is missing or older than the newest source
// file in dir.
func isStale(dir, out string) (bool, error) {
	outInfo, err := os.Stat(out)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	newest, err := newestSource(dir)
	if err != nil {
		return false, err
	}
	return newest.After(outInfo.ModTime()), nil
}

// newestSource returns the modification time of the newest source file
// in dir, considering .go, .templ, go.mod, and go.sum files.
func newestSource(dir string) (time.Time, error) {
	var newest time.Time
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !isSourceFile(info.Name()) {
			return nil
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}
	return newest, nil
}

// isSourceFile reports whether name is a file that affects the
// transpiled output.
func isSourceFile(name string) bool {
	switch {
	case strings.HasSuffix(name, ".go"), strings.HasSuffix(name, ".templ"):
		return true
	case name == "go.mod", name == "go.sum":
		return true
	}
	return false
}

// transpile runs the gopherjs build command, compiling the client
// module's main package to out.
func transpile(dir, out string, t *toolchain) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	// Resolve out to an absolute path, because gopherjs runs in dir and
	// a relative -o would be interpreted relative to dir.
	absOut, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	return runGopherjs(t, dir, "build", "-o", absOut, ".")
}

// runGopherjs runs the gopherjs CLI in dir, compiling against the
// toolchain's GOROOT. A GOPHERJS_GOROOT already set in the environment
// wins, so a consumer can override the GOROOT without code changes.
func runGopherjs(t *toolchain, dir string, args ...string) error {
	env := os.Environ()
	if os.Getenv("GOPHERJS_GOROOT") == "" {
		env = append(env, "GOPHERJS_GOROOT="+t.goroot)
	}
	cmd := exec.Command(t.cli, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gopherjs %s: %w", args[0], err)
	}
	return nil
}
