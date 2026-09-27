package maintenance

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEnvSpelling verifies the variable activator recognizes the true
// spellings, ignores their casing and padding, and dismisses
// everything else.
func TestEnvSpelling(t *testing.T) {
	a := Env("PROBE_ENV_GATE")
	set := func(val string) bool {
		t.Setenv("PROBE_ENV_GATE", val)
		return a()
	}
	for _, tc := range []struct {
		val  string
		want bool
	}{
		{"", false},
		{"0", false},
		{"nope", false},
		{"1", true},
		{" TRUE ", true},
		{"yes", true},
	} {
		if got := set(tc.val); got != tc.want {
			t.Errorf("Env(\"PROBE_ENV_GATE\") with %q = %v; want %v", tc.val, got, tc.want)
		}
	}
}

// TestFlagTransition verifies the file activator tracks the flag
// appearing and disappearing, the motion deploy scripts make without a
// restart.
func TestFlagTransition(t *testing.T) {
	dir := t.TempDir()
	flag := filepath.Join(dir, "down.flag")
	a := Flag(flag)
	if a() {
		t.Fatal("Fresh flag file engages the gate; want disengaged")
	}
	if err := os.WriteFile(flag, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if !a() {
		t.Fatal("Created flag file does not engage the gate")
	}
	if err := os.Remove(flag); err != nil {
		t.Fatal(err)
	}
	if a() {
		t.Fatal("Removed flag file still engages the gate")
	}
}

// TestAnyComposition verifies the merger engages through whichever
// member engages, and rests when all members rest.
func TestAnyComposition(t *testing.T) {
	on := Direct(true)
	off := Direct(false)
	if Any(off, off)() {
		t.Error("Any of resting members engages; want rest")
	}
	if !Any(off, on)() {
		t.Error("Any with one engaging member does not engage")
	}
	if !Any(on, on)() {
		t.Error("Any of engaging members does not engage")
	}
}

// TestLiveMatching verifies exact and prefix rules draw their borders
// correctly, including the sibling-path trap where a prefix rule must
// not swallow a longer path that merely starts with the same stem.
func TestLiveMatching(t *testing.T) {
	rules := []LiveRule{"/healthz", "/static/"}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/healthz", true},
		{"/healthz/detail", false},
		{"/healthzz", false},
		{"/static/css/site.css", true},
		{"/static/deep/nested/img.png", true},
		{"/statically-hosted", false},
		{"/notes", false},
	} {
		if got := Lives(rules, tc.path); got != tc.want {
			t.Errorf("Lives(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
}

// TestEmptyLives verifies no rules admit nothing, the closed-default
// posture.
func TestEmptyLives(t *testing.T) {
	if Lives(nil, "/healthz") {
		t.Error("Nil rule set admits a path; want the closed default")
	}
}
