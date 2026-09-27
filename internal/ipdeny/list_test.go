package ipdeny

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestParseMixedForms verifies the parser eats CSV, line, and
// comment styling in one breath, collapsing duplicate spellings to
// one entry.
func TestParseMixedForms(t *testing.T) {
	l, err := Parse("1.2.3.4, 10.0.0.0/8,#noise\n2001:db8::/32 1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 3 {
		t.Fatalf("distinct entries = %d; want 3", l.Len())
	}
}

// TestMappedEquivalentsBlocked verifies an IPv4-mapped IPv6 spelling
// of a blocked quad is blocked, the classic evasion the slice calls
// out.
func TestMappedEquivalentsBlocked(t *testing.T) {
	l, err := Parse("10.1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	mapped := net.ParseIP("::ffff:0a01:0203")
	if !l.Matches(mapped) {
		t.Error("IPv4-mapped spelling of a blocked address is not blocked")
	}
	if l.Matches(net.ParseIP("10.1.2.4")) {
		t.Error("neighbor of a blocked address is blocked")
	}
}

// TestNetworkMembership verifies CIDR entries embrace their subnet and
// exclude the rest.
func TestNetworkMembership(t *testing.T) {
	l, err := Parse("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	if !l.Matches(net.ParseIP("10.255.1.2")) {
		t.Error("member of the blocked network is not blocked")
	}
	if l.Matches(net.ParseIP("11.0.0.1")) {
		t.Error("outsider of the blocked network is blocked")
	}
}

// TestBlankListsBlockNobody verifies comment-only and blank rosters
// parse to an empty, harmless list.
func TestBlankListsBlockNobody(t *testing.T) {
	l, err := Parse("# only a comment,,\n  ")
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 0 {
		t.Fatalf("entries = %d; want 0", l.Len())
	}
	if l.Matches(net.ParseIP("8.8.8.8")) {
		t.Error("empty list blocks an address")
	}
}

// TestMalformedEntryNamed verifies a broken item raises an error that
// names the offender, keeping startup diagnoses actionable.
func TestMalformedEntryNamed(t *testing.T) {
	_, err := Parse("banana/24")
	if err == nil || !strings.Contains(err.Error(), "banana") {
		t.Fatalf("error = %v; want a diagnosis naming banana", err)
	}
}

// TestPrivateAnnouncements verifies private and loopback entries are
// surfaced for the startup warning, while public entries stay quiet.
func TestPrivateAnnouncements(t *testing.T) {
	l, err := Parse("10.0.0.1, 8.8.8.8, ::1")
	if err != nil {
		t.Fatal(err)
	}
	got := l.PrivateTexts()
	if !slices.Equal(got, []string{"10.0.0.1", "::1"}) {
		t.Errorf("private announcements = %v; want [10.0.0.1 ::1]", got)
	}
}

// TestLoadFileBehaviors verifies the file loader's contract: a
// readable roster loads, an absent file yields an empty list without
// error, and a malformed file surfaces its error.
func TestLoadFileBehaviors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roster")
	if err := os.WriteFile(path, []byte("203.0.113.7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := LoadFile(path)
	if err != nil || l.Len() != 1 {
		t.Fatalf("LoadFile(readable) = %d entries, %v; want 1, nil", l.Len(), err)
	}
	l, err = LoadFile(filepath.Join(dir, "absent"))
	if err != nil || l.Len() != 0 {
		t.Fatalf("LoadFile(absent) = %d entries, %v; want 0, nil", l.Len(), err)
	}
	if err := os.WriteFile(path, []byte("wat?"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadFile(path); err == nil {
		t.Fatal("LoadFile(malformed) = nil; want an error")
	}
}
