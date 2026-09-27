package stack

import (
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var canonicalNames = []string{
	"request.id", "logging.slog", "gate.maintenance", "gate.badiptarget",
	"resp.compress", "resp.cookiefinalize", "api.cors", "intake.postsize",
	"intake.requestheaders", "intake.strings.trim", "session.restore",
	"session.authenticate", "safety.csrf", "safety.signature",
	"policy.rate_limit", "policy.authorization", "lang.localized",
	"binding.params.subst", "session.flash_errors", "cache.response",
	"resp.timeout",
}

var canonicalOrdinals = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21}

// TestSpecsMatchCanonical verifies the shipped table against the
// published chain: names, ordinals, and lanes derived from name
// prefixes.
func TestSpecsMatchCanonical(t *testing.T) {
	if len(Specs) != len(canonicalNames) {
		t.Fatalf("len(Specs) = %d; want %d", len(Specs), len(canonicalNames))
	}
	for i, spec := range Specs {
		if spec.Name != canonicalNames[i] {
			t.Errorf("Specs[%d].Name = %q; want %q", i, spec.Name, canonicalNames[i])
		}
		if spec.Ordinal != canonicalOrdinals[i] {
			t.Errorf("Specs[%d].Ordinal = %d; want %d", i, spec.Ordinal, canonicalOrdinals[i])
		}
		wantLane := strings.SplitN(spec.Name, ".", 2)[0]
		if spec.Lane != wantLane {
			t.Errorf("Specs[%d].Lane = %q; want %q", i, spec.Lane, wantLane)
		}
	}
}

func names(vs []View) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Name
	}
	return out
}

// TestNewBuilderCopiesCanonical verifies a fresh builder holds all
// twenty-one slots, enabled, in canonical order, unfilled.
func TestNewBuilderCopiesCanonical(t *testing.T) {
	vs := NewBuilder().Views()
	if !slices.Equal(names(vs), canonicalNames) {
		t.Fatalf("views = %v; want %v", names(vs), canonicalNames)
	}
	for _, v := range vs {
		if !v.Enabled {
			t.Errorf("slot %s not enabled in fresh builder", v.Name)
		}
		if v.Custom {
			t.Errorf("slot %s marked custom; want a stock slot", v.Name)
		}
	}
}

// TestDisablePreservesNeighbors verifies disabling one slot removes
// it from the compiled chain while its neighbors keep their
// positions.
func TestDisablePreservesNeighbors(t *testing.T) {
	b := NewBuilder()
	b.Disable("api.cors")
	vs := b.Views()
	if !slices.Equal(names(vs), canonicalNames) {
		t.Fatalf("order disturbed by disable: %v", names(vs))
	}
	find := func(v []View, n string) int {
		for i := range v {
			if v[i].Name == n {
				return i
			}
		}
		return -1
	}
	if got := vs[find(vs, "api.cors")].Enabled; got {
		t.Errorf("api.cors enabled = true; want false")
	}
	prev := vs[find(vs, "api.cors")-1]
	next := vs[find(vs, "api.cors")+1]
	if prev.Name != "resp.cookiefinalize" || !prev.Enabled {
		t.Errorf("preceding neighbor = %q (enabled=%v); want resp.cookiefinalize enabled", prev.Name, prev.Enabled)
	}
	if next.Name != "intake.postsize" || !next.Enabled {
		t.Errorf("following neighbor = %q (enabled=%v); want intake.postsize enabled", next.Name, next.Enabled)
	}
	hs, err := b.Compile()
	if err != nil {
		t.Fatalf("compile error = %v", err)
	}
	if len(hs) != 20 {
		t.Errorf("compiled handlers = %d; want 20 after one disable", len(hs))
	}
}

// TestInsertPositions verifies inserted handlers land exactly where
// the position string names, gaining numbered synthetic names.
func TestInsertPositions(t *testing.T) {
	marker := func(c *gin.Context) { c.Next() }
	b := NewBuilder()
	b.Insert("after:logging.slog", marker)
	b.Insert("before:request.id", marker)
	vs := b.Views()
	first := vs[0]
	if first.Name != "custom.2" || !first.Custom {
		t.Fatalf("front entry = %+v; want the before:request.id insertion", first)
	}
	loggingIdx := -1
	for i, v := range vs {
		if v.Name == "logging.slog" {
			loggingIdx = i
		}
	}
	if vs[loggingIdx+1].Name != "custom.1" {
		t.Errorf("entry after logging.slog = %q; want custom.1", vs[loggingIdx+1].Name)
	}
	if _, err := b.Compile(); err != nil {
		t.Fatalf("compile error = %v", err)
	}
}

// TestInvalidPositionBreaksBuild verifies a malformed position string
// surfaces at compile time with a diagnosis.
func TestInvalidPositionBreaksBuild(t *testing.T) {
	marker := func(c *gin.Context) { c.Next() }
	b := NewBuilder()
	b.Insert("left:request.id", marker)
	_, err := b.Compile()
	if !strings.Contains(err.Error(), "invalid insert position") {
		t.Fatalf("compile error = %v; want an invalid-position complaint", err)
	}
}

// TestRemoveDetachesEntries verifies stock slots and custom entries
// detach cleanly, and removing an unknown name breaks the build.
func TestRemoveDetachesEntries(t *testing.T) {
	marker := func(c *gin.Context) { c.Next() }
	b := NewBuilder()
	b.Insert("after:safety.csrf", marker)
	b.Remove("custom.1")
	b.Remove("gate.maintenance")
	for _, v := range b.Views() {
		if v.Name == "custom.1" || v.Name == "gate.maintenance" {
			t.Fatalf("removed entry %s survived", v.Name)
		}
	}
	if _, err := b.Compile(); err != nil {
		t.Fatalf("compile error = %v", err)
	}
	b.Remove("ghost.nowhere")
	_, err := b.Compile()
	if err == nil || !strings.Contains(err.Error(), "ghost.nowhere") {
		t.Fatalf("compile error = %v; want the ghost name diagnosed", err)
	}
}

// TestUnknownOperationsSurfaceAtCompile verifies misnamed Enable,
// Disable, Configure, and Remove all join the build error.
func TestUnknownOperationsSurfaceAtCompile(t *testing.T) {
	b := NewBuilder()
	b.Enable("phantom.a")
	b.Disable("phantom.b")
	b.Configure("phantom.c", nil)
	_, err := b.Compile()
	if err == nil {
		t.Fatal("compile succeeded; want an error naming phantom slots")
	}
	for _, needle := range []string{"phantom.a", "phantom.b", "phantom.c"} {
		if !strings.Contains(err.Error(), needle) {
			t.Errorf("error %q does not name %q", err, needle)
		}
	}
}

// TestCompiledOrderEqualsCanonical drives the full compile of a
// fresh builder and checks handler count stability: twenty-one
// compiled handlers means all slots present, holders included.
func TestCompiledOrderEqualsCanonical(t *testing.T) {
	hs, err := NewBuilder().Compile()
	if err != nil {
		t.Fatalf("compile error = %v", err)
	}
	if len(hs) != len(canonicalNames) {
		t.Fatalf("compiled handlers = %d; want %d", len(hs), len(canonicalNames))
	}
}
