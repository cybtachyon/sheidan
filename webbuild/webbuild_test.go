package webbuild

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGoMajorMinor(t *testing.T) {
	for _, tc := range []struct {
		version string
		major   int
		minor   int
	}{
		{"go1.21.13", 1, 21},
		{"go1.26.0", 1, 26},
		{"go1.27.1", 1, 27},
		{"go1.20", 1, 20},
	} {
		major, minor, err := goMajorMinor(tc.version)
		if err != nil {
			t.Fatalf("goMajorMinor(%q): %v", tc.version, err)
		}
		if major != tc.major || minor != tc.minor {
			t.Errorf("goMajorMinor(%q) = (%d, %d), want (%d, %d)", tc.version, major, minor, tc.major, tc.minor)
		}
	}
}

func TestIsStale(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "out.js")

	// A missing output is stale.
	stale, err := isStale(dir, out)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Error("expected stale when the output is missing")
	}

	// An output newer than every source is not stale.
	if err := os.WriteFile(out, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err = isStale(dir, out)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("expected not stale when the output is newer than the sources")
	}

	// A source newer than the output is stale.
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(out, old, old); err != nil {
		t.Fatal(err)
	}
	stale, err = isStale(dir, out)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Error("expected stale when a source is newer than the output")
	}
}
