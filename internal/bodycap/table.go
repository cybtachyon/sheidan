// Package bodycap provides the primitives for bounding inbound
// payloads: a rule table that resolves per-path limits, and a
// clamping reader that meters delivery and truncates at the cap.
package bodycap

import "strings"

// defaultCeiling is the shipped body cap: one megabyte, comfortable
// for ordinary JSON traffic and stingy enough to starve a flood.
const defaultCeiling = 1 << 20

// Rule pairs a path prefix with the cap it imposes. Specific routes
// typically loosen the default for uploads, so longer prefixes beat
// shorter ones.
type Rule struct {
	Prefix string
	Limit  int64
}

// Table resolves the effective cap for a request path. The zero
// table normalizes to the shipped default ceiling and no rules.
type Table struct {
	Default int64
	Rules   []Rule
}

// Normalize fills zero selections with the shipped defaults, so a
// partially specified bag still behaves sensibly.
func (tb Table) Normalize() Table {
	if tb.Default <= 0 {
		tb.Default = defaultCeiling
	}
	return tb
}

// LimitFor resolves the governing cap for a path. The longest
// matching prefix wins, so a specific route outranks a broad one,
// and a path matching no rule falls through to the default.
func (tb Table) LimitFor(path string) int64 {
	lim := tb.Default
	bestPrefix := -1
	for _, rule := range tb.Rules {
		if rule.Limit <= 0 {
			continue
		}
		if len(rule.Prefix) > bestPrefix && strings.HasPrefix(path, rule.Prefix) {
			lim = rule.Limit
			bestPrefix = len(rule.Prefix)
		}
	}
	return lim
}
