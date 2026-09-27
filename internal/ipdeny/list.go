// Package ipdeny maintains a blocklist of client addresses. It
// parses lists mixing single addresses and CIDR masks, normalizes the
// address forms so equivalents compare equal, and matches incoming
// addresses against the loaded set.
package ipdeny

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// separators splits blocklist text into items. Whitespace and commas
// all qualify, so CSV-style and line-style rosters share one parser.
var separators = " \t\r\n,"

// Item is one parsed blocklist entry: either a single exact address
// or a network, kept with its original text for diagnostics.
type Item struct {
	text    string
	address net.IP
	network *net.IPNet
	priv    bool
}

// Private reports whether the entry names a private, loopback, or
// link-local range. Blocking such ranges only separates clients from
// the operator's own fleet, so the slot's factory announces them
// loudly at startup.
func (e Item) Private() bool { return e.priv }

// Text returns the entry's original spelling.
func (e Item) Text() string { return e.text }

// List is a parsed blocklist. The zero value blocks nobody, so a
// slot shipped with an empty list costs one empty sweep per request.
type List struct {
	items []Item
}

// Len reports the number of distinct entries.
func (l List) Len() int { return len(l.items) }

// Items reports the parsed entries in load order.
func (l List) Items() []Item { return l.items }

// Matches reports whether addr falls inside any entry. Incoming
// addresses normalize through the same canonicalization the entries
// underwent, so an IPv4-mapped IPv6 spelling of a blocked quad
// slides nowhere.
func (l List) Matches(addr net.IP) bool {
	if addr == nil {
		return false
	}
	target := canonical(addr)
	for _, e := range l.items {
		if len(e.address) > 0 && e.address.Equal(target) {
			return true
		}
		if e.network != nil && e.network.Contains(addr) {
			return true
		}
	}
	return false
}

// canonical folds an address into its shortest comparable form: four
// bytes for IPv4 (covering the IPv4-mapped IPv6 spellings), sixteen
// for native IPv6.
func canonical(addr net.IP) net.IP {
	if v4 := addr.To4(); v4 != nil {
		return v4
	}
	return addr
}

// privateRanges are the families whose presence deserves a loud
// startup warning.
var privateRanges = []*net.IPNet{
	mustCIDR("10.0.0.0/8"),
	mustCIDR("172.16.0.0/12"),
	mustCIDR("192.168.0.0/16"),
	mustCIDR("127.0.0.0/8"),
	mustCIDR("169.254.0.0/16"),
	mustCIDR("fc00::/7"),
	mustCIDR("::1/128"),
	mustCIDR("fe80::/10"),
}

func mustCIDR(text string) *net.IPNet {
	_, nw, err := net.ParseCIDR(text)
	if err != nil {
		panic(err)
	}
	return nw
}

func inPrivateRange(addr net.IP) bool {
	for _, nw := range privateRanges {
		if nw.Contains(addr) {
			return true
		}
	}
	return false
}

// Parse digests blocklist text into a List. Lines starting with '#'
// are comments, and everything after a mid-line '#' is clipped, so
// rosters annotate themselves freely. Surviving fragments separate on
// whitespace or commas: a bare address becomes an exact-match entry,
// and a slashed address becomes a network. Entries deduplicate on
// their canonical form, so mixed spellings of the same address land
// once. A malformed item raises an error naming the offender,
// because a broken list is a startup defect rather than a
// request-time surprise.
func Parse(text string) (List, error) {
	list := List{}
	seen := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		for _, raw := range strings.FieldsFunc(line, func(r rune) bool {
			return strings.ContainsRune(separators, r)
		}) {
			item := strings.TrimSpace(raw)
			if item == "" {
				continue
			}
			e, err := parseItem(item)
			if err != nil {
				return List{}, fmt.Errorf("ipdeny: unrecognized entry %q: %v", item, err)
			}
			key := itemKey(e)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = item
			list.items = append(list.items, e)
		}
	}
	return list, nil
}

func parseItem(text string) (Item, error) {
	if strings.ContainsRune(text, '/') {
		_, nw, err := net.ParseCIDR(text)
		if err != nil {
			return Item{}, err
		}
		return Item{text: text, network: nw, priv: inPrivateRange(nw.IP)}, nil
	}
	addr := net.ParseIP(text)
	if addr == nil {
		return Item{}, fmt.Errorf("neither an address nor a CIDR")
	}
	return Item{text: text, address: canonical(addr), priv: inPrivateRange(addr)}, nil
}

// itemKey fingerprints an entry for deduplication: the canonical
// bytes of an address, or the network's mask plus subnet bytes.
func itemKey(e Item) string {
	if len(e.address) > 0 {
		return "addr:" + string(e.address)
	}
	return "net:" + string(e.network.Mask) + ":" + string(e.network.IP.To16())
}

// PrivateTexts lists the original spellings of the entries that fall
// inside private or loopback ranges, for the startup announcement.
func (l List) PrivateTexts() []string {
	var out []string
	for _, e := range l.items {
		if e.priv {
			out = append(out, e.text)
		}
	}
	return out
}

// LoadFile reads and parses a blocklist file. A missing file yields
// an empty list without error, so a not-yet-shipped roster cannot
// sink the boot; a readable but malformed file does surface its
// error.
func LoadFile(path string) (List, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return List{}, nil
		}
		return List{}, err
	}
	return Parse(string(data))
}
