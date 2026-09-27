// Package maintenance provides the primitives behind the maintenance
// gate: activators that report whether the service is down for
// maintenance, and live rules that keep selected paths reachable while
// it is.
package maintenance

import (
	"os"
	"strings"
)

// Activator reports whether maintenance is engaged. Activators
// evaluate per request, so flipping a trigger mid-life takes effect on
// the next request without a restart, and in-flight work completes
// naturally.
type Activator func() bool

// trueSpellings lists the variable readings Env treats as engaged.
var trueSpellings = []string{"1", "true", "yes"}

// Env builds an activator bound to a process variable. The activator
// engages when the variable takes any true spelling, ignoring case
// and surrounding blanks; a blank or absent reading leaves the
// service open.
func Env(name string) Activator {
	return func() bool {
		got := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
		for _, spelled := range trueSpellings {
			if got == spelled {
				return true
			}
		}
		return false
	}
}

// Flag builds an activator bound to the existence of a file. Deploy
// scripts toggle the file without signaling the server, so engagement
// follows the filesystem.
func Flag(path string) Activator {
	return func() bool { _, err := os.Stat(path); return err == nil }
}

// Any merges activators into one that engages when any member
// engages. An empty merge stays disengaged.
func Any(ms ...Activator) Activator {
	return func() bool {
		for _, m := range ms {
			if m() {
				return true
			}
		}
		return false
	}
}

// Direct builds an activator frozen at the supplied state. Tests use
// it to pin the gate, and apps managing maintenance through their own
// control plane can roll their own closure instead.
func Direct(on bool) Activator {
	return func() bool { return on }
}

// LiveRule selects one path family that stays reachable during
// maintenance. Rules ending in a slash match the whole prefix tree,
// and the others match one exact path.
type LiveRule string

// Matches reports whether path falls inside the rule's family.
func (r LiveRule) Matches(path string) bool {
	rule := string(r)
	if strings.HasSuffix(rule, "/") {
		return strings.HasPrefix(path, rule)
	}
	return path == rule
}

// DefaultLive lists the families monitors and asset pipelines depend
// on: the health probe and the static tree.
var DefaultLive = []LiveRule{"/healthz", "/static/"}

// Lives reports whether path stays reachable under the given rules.
func Lives(rules []LiveRule, path string) bool {
	for _, r := range rules {
		if r.Matches(path) {
			return true
		}
	}
	return false
}
