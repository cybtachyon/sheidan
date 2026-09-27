package bodycap

import "testing"

// TestNormalizeSuppliesDefaults verifies the zero table adopts the
// shipped ceiling, the make-the-zero-value-useful posture.
func TestNormalizeSuppliesDefaults(t *testing.T) {
	tb := Table{}.Normalize()
	if tb.Default != 1<<20 {
		t.Errorf("default = %d; want 1 MiB", tb.Default)
	}
}

// TestLongestPrefixWins verifies specific routes outrank broad ones,
// and a path matching no rule falls through to the default.
func TestLongestPrefixWins(t *testing.T) {
	tb := Table{Default: 100, Rules: []Rule{{"", 50}, {"/up", 500}}}.Normalize()
	for _, tc := range []struct {
		path string
		want int64
	}{
		{"/up/big", 500},
		{"/upload", 500},
		{"/notes", 50},
		{"/", 50},
	} {
		if got := tb.LimitFor(tc.path); got != tc.want {
			t.Errorf("LimitFor(%q) = %d; want %d", tc.path, got, tc.want)
		}
	}
}

// TestSpecificBeatsBroad verifies a deeper prefix beats shallower
// ones even when the shallow one appears earlier in the table.
func TestSpecificBeatsBroad(t *testing.T) {
	tb := Table{Default: 1, Rules: []Rule{{"/a", 10}, {"/a/b", 20}}}
	if got := tb.LimitFor("/a/b/c"); got != 20 {
		t.Errorf("LimitFor = %d; want 20", got)
	}
	if got := tb.LimitFor("/a/z"); got != 10 {
		t.Errorf("LimitFor = %d; want 10", got)
	}
}
