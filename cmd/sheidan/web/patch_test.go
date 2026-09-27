package main

import (
	"testing"

	"github.com/gopherjs/gopherjs/js"
)

// mountIn mounts tree into a fresh container and returns the container
// and the mounted root node.
func mountIn(doc *js.Object, tree *VNode) (*js.Object, *js.Object) {
	container := fakeElement("div")
	node := mount(doc, tree)
	container.Call("appendChild", node)
	return container, node
}

// TestMountElementTree verifies that mount builds the real DOM tree:
// the element nesting, the attributes, and the text nodes.
func TestMountElementTree(t *testing.T) {
	doc := fakeDocument()
	tree := El("ul", nil,
		El("li", []Attr{{Name: "class", Value: "a"}}, Text("one"), Text("two")),
		El("li", nil, Text("three")),
	)
	container, _ := mountIn(doc, tree)

	if got := fakeChildTags(container); len(got) != 1 || got[0] != "ul" {
		t.Fatalf("container children = %v; want [ul]", got)
	}
	ul := container.Get("children").Index(0)
	if got := fakeChildTags(ul); len(got) != 2 || got[0] != "li" || got[1] != "li" {
		t.Fatalf("ul children = %v; want [li li]", got)
	}
	li := ul.Get("children").Index(0)
	if got := fakeChildTags(li); len(got) != 2 || got[0] != "text:one" || got[1] != "text:two" {
		t.Fatalf("li children = %v; want [text:one text:two]", got)
	}
	if got := li.Call("getAttribute", "class").String(); got != "a" {
		t.Errorf("li class = %q; want %q", got, "a")
	}
}

// TestPatchTextUpdate verifies that a text change updates the existing
// text node's value in place, with no structural DOM writes.
func TestPatchTextUpdate(t *testing.T) {
	doc := fakeDocument()
	old := Text("one")
	container, _ := mountIn(doc, old)
	fakeOps = 0

	patchChild(doc, old, Text("two"), container)

	if got := old.node.Get("nodeValue").String(); got != "two" {
		t.Errorf("nodeValue = %q; want %q", got, "two")
	}
	if ops := fakeOpsCount(); ops != 0 {
		t.Errorf("ops = %d; want 0 (a text update writes nodeValue only)", ops)
	}
}

// TestPatchTextUnchanged verifies that an unchanged text node costs no
// DOM writes.
func TestPatchTextUnchanged(t *testing.T) {
	doc := fakeDocument()
	old := Text("one")
	container, _ := mountIn(doc, old)
	fakeOps = 0

	patchChild(doc, old, Text("one"), container)

	if ops := fakeOpsCount(); ops != 0 {
		t.Errorf("ops = %d; want 0", ops)
	}
}

// TestPatchAttrChanges verifies that the patch sets attributes whose
// value changed, adds new ones, and removes ones the new render does
// not have.
func TestPatchAttrChanges(t *testing.T) {
	doc := fakeDocument()
	old := El("p", []Attr{{Name: "class", Value: "a"}, {Name: "id", Value: "x"}})
	container, _ := mountIn(doc, old)
	fakeOps = 0

	patchChild(doc, old, El("p", []Attr{{Name: "class", Value: "b"}, {Name: "title", Value: "t"}}), container)

	el := old.node
	if got := el.Call("getAttribute", "class").String(); got != "b" {
		t.Errorf("class = %q; want %q", got, "b")
	}
	if got := el.Call("getAttribute", "title").String(); got != "t" {
		t.Errorf("title = %q; want %q", got, "t")
	}
	if got := el.Call("getAttribute", "id"); got != nil {
		t.Errorf("id = %v; want it removed", got)
	}
	if ops := fakeOpsCount(); ops != 3 {
		t.Errorf("ops = %d; want 3 (change class, add title, remove id)", ops)
	}
}

// TestPatchAddAndRemoveChildren verifies positional reconciliation of
// child lists of different lengths: the overlap is patched in place,
// the surplus old children are removed, and the surplus new children
// are appended.
func TestPatchAddAndRemoveChildren(t *testing.T) {
	doc := fakeDocument()
	old := El("ul", nil,
		El("li", nil, Text("a")),
		El("li", nil, Text("b")),
		El("li", nil, Text("c")),
	)
	container, ul := mountIn(doc, old)
	fakeOps = 0

	patchChild(doc, old, El("ul", nil,
		El("li", nil, Text("a")),
		El("li", nil, Text("d")),
	), container)

	if got := fakeChildTags(ul); len(got) != 2 || got[0] != "li" || got[1] != "li" {
		t.Fatalf("children = %v; want [li li]", got)
	}
	first, second := ul.Get("children").Index(0), ul.Get("children").Index(1)
	if got := first.Get("children").Index(0).Get("nodeValue").String(); got != "a" {
		t.Errorf("first text = %q; want %q", got, "a")
	}
	if got := second.Get("children").Index(0).Get("nodeValue").String(); got != "d" {
		t.Errorf("second text = %q; want %q", got, "d")
	}
	// The first li is reused, the second li's text is updated in
	// place, and the third li is removed: one DOM write.
	if ops := fakeOpsCount(); ops != 1 {
		t.Errorf("ops = %d; want 1 (remove the third li)", ops)
	}
}

// TestPatchTagChange verifies that a tag change replaces the element
// with a fresh mount instead of patching it in place.
func TestPatchTagChange(t *testing.T) {
	doc := fakeDocument()
	old := El("h1", nil, Text("t"))
	container, oldNode := mountIn(doc, old)
	fakeOps = 0
	fakeCreates = 0

	patchChild(doc, old, El("h2", nil, Text("t")), container)

	if got := container.Get("children").Index(0); got == oldNode {
		t.Fatal("the old node was kept; want a replacement")
	}
	if got := container.Get("children").Index(0).Get("tagName").String(); got != "h2" {
		t.Errorf("tag = %q; want %q", got, "h2")
	}
	// The new element is mounted with its text child (one appendChild)
	// and then swapped in (one replaceChild).
	if ops := fakeOpsCount(); ops != 2 {
		t.Errorf("ops = %d; want 2 (appendChild the text child, replaceChild)", ops)
	}
	if creates := fakeCreatesCount(); creates != 2 {
		t.Errorf("creates = %d; want 2 (the new element and its text child)", creates)
	}
}

// TestPatchKeyedReorder verifies that a reorder of keyed children moves
// the existing nodes instead of rebuilding them, and that the result
// is in the new order.
func TestPatchKeyedReorder(t *testing.T) {
	doc := fakeDocument()
	mk := func(keys ...string) *VNode {
		var children []*VNode
		for _, k := range keys {
			children = append(children, Keyed(El("li", nil, Text(k)), k))
		}
		return El("ul", nil, children...)
	}
	old := mk("a", "b", "c")
	container, ul := mountIn(doc, old)
	nodeA, nodeB, nodeC := ul.Get("children").Index(0), ul.Get("children").Index(1), ul.Get("children").Index(2)
	fakeOps = 0
	fakeCreates = 0

	patchChild(doc, old, mk("c", "a", "b"), container)

	if got := fakeChildTags(ul); len(got) != 3 || got[0] != "li" || got[1] != "li" || got[2] != "li" {
		t.Fatalf("children = %v; want [li li li]", got)
	}
	got := []*js.Object{
		ul.Get("children").Index(0),
		ul.Get("children").Index(1),
		ul.Get("children").Index(2),
	}
	if got[0] != nodeC || got[1] != nodeA || got[2] != nodeB {
		t.Error("the reorder did not reuse the existing nodes in the new order")
	}
	if creates := fakeCreatesCount(); creates != 0 {
		t.Errorf("creates = %d; want 0 (a pure reorder mounts nothing)", creates)
	}
	// The moves: c to the front, a after c. b is already in place.
	if ops := fakeOpsCount(); ops != 2 {
		t.Errorf("ops = %d; want 2 (two insertBefore moves)", ops)
	}
}

// TestPatchKeyedAddRemove verifies that keyed reconciliation mounts the
// new children, removes the dropped ones, and keeps the surviving
// nodes in place.
func TestPatchKeyedAddRemove(t *testing.T) {
	doc := fakeDocument()
	mk := func(keys ...string) *VNode {
		var children []*VNode
		for _, k := range keys {
			children = append(children, Keyed(El("li", nil, Text(k)), k))
		}
		return El("ul", nil, children...)
	}
	old := mk("a", "b", "c")
	container, ul := mountIn(doc, old)
	nodeB := ul.Get("children").Index(1)
	fakeOps = 0
	fakeCreates = 0

	patchChild(doc, old, mk("b", "d"), container)

	if got := fakeChildTags(ul); len(got) != 2 || got[0] != "li" || got[1] != "li" {
		t.Fatalf("children = %v; want [li li]", got)
	}
	if got := ul.Get("children").Index(0); got != nodeB {
		t.Error("the surviving node b was not reused in its new position")
	}
	if got := ul.Get("children").Index(1).Get("children").Index(0).Get("nodeValue").String(); got != "d" {
		t.Errorf("second text = %q; want %q", got, "d")
	}
	// The new node d is an li with a text child, so it creates two nodes.
	if creates := fakeCreatesCount(); creates != 2 {
		t.Errorf("creates = %d; want 2 (the new li and its text child)", creates)
	}
}

// TestPatchUnchangedTree verifies that re-rendering an identical tree
// costs no DOM writes and no node creation: the reconciliation's
// minimal-difference guarantee.
func TestPatchUnchangedTree(t *testing.T) {
	doc := fakeDocument()
	tree := func() *VNode {
		return El("div", nil,
			El("h1", nil, Text("Notes")),
			El("ul", nil,
				Keyed(El("li", nil, Text("a")), "1"),
				Keyed(El("li", nil, Text("b")), "2"),
			),
		)
	}
	old := tree()
	container, _ := mountIn(doc, old)
	fakeOps = 0
	fakeCreates = 0

	patchChild(doc, old, tree(), container)

	if ops := fakeOpsCount(); ops != 0 {
		t.Errorf("ops = %d; want 0 for an unchanged tree", ops)
	}
	if creates := fakeCreatesCount(); creates != 0 {
		t.Errorf("creates = %d; want 0 for an unchanged tree", creates)
	}
}

// TestPatchEventSwap verifies that a reused element swaps its listener
// when the render changes its role, so the element runs the handler its
// current render specifies.
func TestPatchEventSwap(t *testing.T) {
	doc := fakeDocument()
	mk := func(label string, calls *int) *VNode {
		btn := El("button", []Attr{{Name: "class", Value: label}}, Text(label))
		WithEvents(btn, Event{Name: "click", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
			*calls++
			return nil
		})})
		return btn
	}
	var editCalls, saveCalls int
	old := El("div", nil, mk("edit", &editCalls))
	container, _ := mountIn(doc, old)
	fakeOps = 0

	patchChild(doc, old, El("div", nil, mk("save", &saveCalls)), container)

	// The button is the div's first child, one level below the container.
	fakeClick(container.Get("children").Index(0).Get("children").Index(0))
	if editCalls != 0 || saveCalls != 1 {
		t.Errorf("calls = edit:%d save:%d; want edit:0 save:1 (the reused element must run the new handler)", editCalls, saveCalls)
	}
}
