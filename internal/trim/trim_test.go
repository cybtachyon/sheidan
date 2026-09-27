package trim

import (
	"encoding/json"
	"net/url"
	"testing"
)

// testExclude builds the default exclusion set for the golden cases.
func testExclude() map[string]bool {
	return ExcludedSet(defaultExcluded)
}

// TestValuePads verifies the golden padding cases: a plain space pad,
// a tab pad, a newline pad, and a Unicode space pad all trim to the
// bare value.
func TestValuePads(t *testing.T) {
	exclude := testExclude()
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{"space", "  hello  ", "hello"},
		{"tab", "\thello\t", "hello"},
		{"newline", "\nhello\n", "hello"},
		{"unicode", "\u00a0hello\u00a0", "hello"},
		{"mixed", " \t\n hello \t\n", "hello"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Value(tc.value, exclude, defaultMaxDepth); got != tc.want {
				t.Errorf("Value(%q) = %q; want %q", tc.value, got, tc.want)
			}
		})
	}
}

// TestValueExcluded verifies a field named by the exclusion list keeps
// its padding verbatim, even when nested.
func TestValueExcluded(t *testing.T) {
	exclude := testExclude()
	tree := map[string]any{
		"title":    "  hello  ",
		"password": "  secret  ",
		"nested":   map[string]any{"api_key": "  key  "},
	}
	got := Value(tree, exclude, defaultMaxDepth).(map[string]any)
	if got["title"] != "hello" {
		t.Errorf("title = %q; want trimmed", got["title"])
	}
	if got["password"] != "  secret  " {
		t.Errorf("password = %q; want verbatim", got["password"])
	}
	nested := got["nested"].(map[string]any)
	if nested["api_key"] != "  key  " {
		t.Errorf("nested api_key = %q; want verbatim", nested["api_key"])
	}
}

// TestValueDeepNesting verifies the walk reaches string leaves deep in
// an array-of-object structure and trims them.
func TestValueDeepNesting(t *testing.T) {
	exclude := testExclude()
	tree := map[string]any{
		"rows": []any{
			map[string]any{"cells": []any{"  a  ", "  b  "}},
		},
	}
	got := Value(tree, exclude, defaultMaxDepth).(map[string]any)
	rows := got["rows"].([]any)
	cells := rows[0].(map[string]any)["cells"].([]any)
	if cells[0] != "a" || cells[1] != "b" {
		t.Errorf("cells = %v; want trimmed leaves", cells)
	}
}

// TestValueDepthCap verifies the walk stops trimming past the depth
// bound, leaving deeper leaves untouched.
func TestValueDepthCap(t *testing.T) {
	exclude := testExclude()
	tree := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "  deep  ",
			},
		},
	}
	// A depth cap of 2 means paths of length >= 2 are untouched; the
	// leaf sits at path ["a","b","c"], length 3.
	got := Value(tree, exclude, 2).(map[string]any)
	inner := got["a"].(map[string]any)["b"].(map[string]any)
	if inner["c"] != "  deep  " {
		t.Errorf("deep leaf = %q; want untouched past the cap", inner["c"])
	}
}

// TestJSONRewrite verifies a JSON body rewrite trims string leaves,
// preserves numbers exactly, keeps excluded fields verbatim, and
// reports the raw original.
func TestJSONRewrite(t *testing.T) {
	exclude := testExclude()
	raw := []byte(`{"title":"  hello  ","count":42,"password":"  keep  "}`)
	transformed, kept, ok := JSON(raw, exclude, defaultMaxDepth)
	if !ok {
		t.Fatal("rewrite did not happen")
	}
	if string(kept) != string(raw) {
		t.Errorf("kept = %s; want the raw original", kept)
	}
	var got struct {
		Title    string `json:"title"`
		Count    int    `json:"count"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(transformed, &got); err != nil {
		t.Fatalf("unmarshal transformed = %v", err)
	}
	if got.Title != "hello" {
		t.Errorf("title = %q; want trimmed", got.Title)
	}
	if got.Count != 42 {
		t.Errorf("count = %d; want preserved", got.Count)
	}
	if got.Password != "  keep  " {
		t.Errorf("password = %q; want verbatim", got.Password)
	}
}

// TestJSONInvalid verifies a body that is not valid JSON is returned
// unchanged with the ok flag false.
func TestJSONInvalid(t *testing.T) {
	exclude := testExclude()
	raw := []byte(`{not json`)
	transformed, kept, ok := JSON(raw, exclude, defaultMaxDepth)
	if ok {
		t.Fatal("invalid JSON reported as rewritten")
	}
	if string(transformed) != string(raw) || string(kept) != string(raw) {
		t.Error("invalid JSON was altered")
	}
}

// TestQueryTrim verifies query values trim and excluded keys stay
// verbatim.
func TestQueryTrim(t *testing.T) {
	exclude := testExclude()
	got := Query("title=%20%20hello%20%20&password=%20keep%20", exclude)
	values, err := url.ParseQuery(got)
	if err != nil {
		t.Fatalf("parse = %v", err)
	}
	if values["title"][0] != "hello" {
		t.Errorf("title = %q; want trimmed", values["title"][0])
	}
	if values["password"][0] != " keep " {
		t.Errorf("password = %q; want verbatim", values["password"][0])
	}
}

// TestFormTrim verifies form values trim in place and excluded keys
// stay verbatim.
func TestFormTrim(t *testing.T) {
	exclude := testExclude()
	form := map[string][]string{
		"title":    {"  hello  "},
		"password": {"  keep  "},
	}
	Form(form, exclude)
	if form["title"][0] != "hello" {
		t.Errorf("title = %q; want trimmed", form["title"][0])
	}
	if form["password"][0] != "  keep  " {
		t.Errorf("password = %q; want verbatim", form["password"][0])
	}
}
