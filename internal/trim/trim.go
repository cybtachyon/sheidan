// Package trim provides the transformer for the intake.strings.trim
// slot. It removes leading and trailing whitespace from string-typed
// input values (query parameters, form fields, and JSON body strings)
// before handlers decode them, absorbing the padding that paste and
// form tooling love to add. Excluded fields keep their padding
// verbatim, and the raw body is preserved for auditors.
package trim

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// defaultExcluded lists field names that keep their padding verbatim.
// Stripping whitespace from a secret the user chose is data
// corruption dressed up as kindness, so the common secret names lead
// the list.
var defaultExcluded = []string{
	"password", "passwd", "api_key", "apikey", "secret",
	"token", "session", "csrf", "authorization",
}

// defaultMaxDepth bounds how deep the walk descends before it stops
// trimming, so a pathological nesting cannot burn the request budget.
const defaultMaxDepth = 32

// Params tunes the trim transform. Zero selections recover the
// shipped defaults: the standard excluded field names and the depth
// bound.
type Params struct {
	Exclude  []string
	MaxDepth int
}

// Normalize fills zero selections with the shipped defaults, so a
// partially specified bag still behaves sensibly.
func (p Params) Normalize() Params {
	if len(p.Exclude) == 0 {
		p.Exclude = defaultExcluded
	}
	if p.MaxDepth <= 0 {
		p.MaxDepth = defaultMaxDepth
	}
	return p
}

// ExcludedSet builds the case-insensitive lookup set of excluded field
// names.
func ExcludedSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[strings.ToLower(name)] = true
	}
	return set
}

// isExcluded reports whether the last path segment names an excluded
// field.
func isExcluded(set map[string]bool, path []string) bool {
	if len(path) == 0 {
		return false
	}
	return set[strings.ToLower(path[len(path)-1])]
}

// Value removes leading and trailing whitespace from the string leaves
// of a decoded JSON tree. It walks arrays and objects to their
// leaves, trims string leaves that are not excluded, and leaves
// numbers, booleans, and nulls intact. Unicode space forms trim
// through unicode.IsSpace, and the walk stops at maxDepth.
func Value(value any, exclude map[string]bool, maxDepth int) any {
	return valueAt(value, exclude, maxDepth, nil)
}

// valueAt is the recursive core of Value. It copies the path for each
// child instead of appending in place, so sibling branches never alias
// one another's backing array.
func valueAt(value any, exclude map[string]bool, maxDepth int, path []string) any {
	if len(path) >= maxDepth {
		return value
	}
	switch v := value.(type) {
	case string:
		if isExcluded(exclude, path) {
			return v
		}
		return strings.TrimSpace(v)
	case map[string]any:
		for key, child := range v {
			childPath := make([]string, len(path)+1)
			copy(childPath, path)
			childPath[len(path)] = key
			v[key] = valueAt(child, exclude, maxDepth, childPath)
		}
		return v
	case []any:
		for i, child := range v {
			childPath := make([]string, len(path)+1)
			copy(childPath, path)
			childPath[len(path)] = strconv.Itoa(i)
			v[i] = valueAt(child, exclude, maxDepth, childPath)
		}
		return v
	default:
		return value
	}
}

// JSON rewrites a JSON body with its string values trimmed, returning
// the transformed bytes, the raw original, and whether a rewrite
// happened. A body that is not valid JSON is returned unchanged.
// Numbers keep their exact form through json.Number, so a rewrite
// never corrupts a numeric field.
func JSON(raw []byte, exclude map[string]bool, maxDepth int) (transformed, kept []byte, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return raw, raw, false
	}
	trimmed := Value(tree, exclude, maxDepth)
	out, err := json.Marshal(trimmed)
	if err != nil {
		return raw, raw, false
	}
	return out, raw, true
}

// Query trims the values of a query string, leaving excluded fields
// intact, and returns the re-encoded string. An unparseable string is
// returned unchanged.
func Query(rawQuery string, exclude map[string]bool) string {
	if rawQuery == "" {
		return rawQuery
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	for key, list := range values {
		if isExcluded(exclude, []string{key}) {
			continue
		}
		for i, value := range list {
			values[key][i] = strings.TrimSpace(value)
		}
	}
	return values.Encode()
}

// Form trims the values of a parsed form in place, leaving excluded
// fields intact.
func Form(form url.Values, exclude map[string]bool) {
	for key, list := range form {
		if isExcluded(exclude, []string{key}) {
			continue
		}
		for i, value := range list {
			form[key][i] = strings.TrimSpace(value)
		}
	}
}
