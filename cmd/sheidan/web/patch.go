package main

import (
	"strings"

	"github.com/gopherjs/gopherjs/js"
)

// mount creates the real DOM tree for v and returns the root node. It is
// used for the initial render and for nodes that reconciliation adds.
// A nil v mounts nothing, so a nil child in a Children slice is a no-op
// instead of a crash.
func mount(doc *js.Object, v *VNode) *js.Object {
	if v == nil {
		return nil
	}
	if v.Tag == "" {
		node := doc.Call("createTextNode", v.Text)
		v.node = node
		return node
	}
	node := doc.Call("createElement", v.Tag)
	for _, a := range v.Attrs {
		node.Call("setAttribute", a.Name, a.Value)
	}
	for _, e := range v.Events {
		node.Call("addEventListener", e.Name, e.Fn)
	}
	for _, child := range v.Children {
		if child == nil {
			continue
		}
		node.Call("appendChild", mount(doc, child))
	}
	v.node = node
	return node
}

// patchChild reconciles old and new, both children of parent, and returns
// the real node new represents. A nil old mounts new; a nil new removes
// old; a tag change replaces old with a fresh mount of new; otherwise the
// real node is reused and patched in place, so an unchanged subtree costs
// no DOM writes.
func patchChild(doc *js.Object, old, new *VNode, parent *js.Object) *js.Object {
	if new == nil {
		if old != nil && old.node != nil {
			parent.Call("removeChild", old.node)
		}
		return nil
	}
	if old == nil || old.node == nil {
		node := mount(doc, new)
		parent.Call("appendChild", node)
		return node
	}
	if old.Tag != new.Tag {
		node := mount(doc, new)
		parent.Call("replaceChild", node, old.node)
		return node
	}
	new.node = old.node
	if old.Tag == "" {
		updateTextChild(old, new)
		return old.node
	}
	patchAttrs(old, new)
	patchEvents(old, new)
	patchChildren(doc, old.Children, new.Children, old.node)
	return old.node
}

// updateTextChild brings the real text node up to date with new. Plain
// text writes the node's value directly. A textarea's content is its
// value property, so the write targets the element, and only while the
// value still equals the seed we last supplied: a store change must not
// clobber text the user has typed. The new node always records the
// seed it presents, so the next patch judges taint against the current
// store value.
func updateTextChild(old, new *VNode) {
	if old.Text == new.Text {
		new.lastSeeded = old.lastSeeded
		return
	}
	p := textareaParent(old.node)
	if p == nil {
		old.node.Set("nodeValue", new.Text)
		return
	}
	if p.Get("value").String() == old.lastSeeded {
		p.Set("value", new.Text)
		old.node.Set("nodeValue", new.Text)
	}
	new.lastSeeded = new.Text
}

// textareaParent returns the parent element of a text node when that
// parent is a textarea, or nil otherwise.
func textareaParent(node *js.Object) *js.Object {
	p := node.Get("parentNode")
	if p == nil || !strings.EqualFold(p.Get("tagName").String(), "textarea") {
		return nil
	}
	return p
}

// patchEvents replaces the real element's listeners with new's. A
// reused element keeps its node across renders, so its listeners must
// follow the render: an element that was an edit button in one render
// and a save button in the next must run the save handler. The
// handlers the client attaches read their state at call time, so
// swapping in the render's fresh closures is always correct.
func patchEvents(old, new *VNode) {
	for _, e := range old.Events {
		old.node.Call("removeEventListener", e.Name, e.Fn)
	}
	for _, e := range new.Events {
		old.node.Call("addEventListener", e.Name, e.Fn)
	}
}

// patchAttrs updates the real element's attributes to match new, changing
// attributes whose value changed and removing attributes that old had and
// new does not.
func patchAttrs(old, new *VNode) {
	oldMap := make(map[string]string, len(old.Attrs))
	for _, a := range old.Attrs {
		oldMap[a.Name] = a.Value
	}
	newMap := make(map[string]string, len(new.Attrs))
	for _, a := range new.Attrs {
		newMap[a.Name] = a.Value
	}
	for name, oldVal := range oldMap {
		if newVal, ok := newMap[name]; !ok {
			old.node.Call("removeAttribute", name)
		} else if oldVal != newVal {
			old.node.Call("setAttribute", name, newVal)
		}
	}
	for name, val := range newMap {
		if _, ok := oldMap[name]; !ok {
			old.node.Call("setAttribute", name, val)
		}
	}
}

// patchChildren reconciles the old and new child lists of parent.
// Children with a key are matched by key and moved in place; children
// without a key are matched by position.
func patchChildren(doc *js.Object, old, new []*VNode, parent *js.Object) {
	if hasKey(old) || hasKey(new) {
		patchKeyedChildren(doc, old, new, parent)
	} else {
		patchPositionalChildren(doc, old, new, parent)
	}
}

// hasKey reports whether any child carries a key.
func hasKey(children []*VNode) bool {
	for _, c := range children {
		if c != nil && c.Key != "" {
			return true
		}
	}
	return false
}

// patchPositionalChildren matches children by index: it patches the
// overlap in place, removes the surplus old children, and appends the
// surplus new children.
func patchPositionalChildren(doc *js.Object, old, new []*VNode, parent *js.Object) {
	n := len(old)
	if len(new) < n {
		n = len(new)
	}
	for i := 0; i < n; i++ {
		patchChild(doc, old[i], new[i], parent)
	}
	for i := n; i < len(old); i++ {
		if old[i] != nil && old[i].node != nil {
			parent.Call("removeChild", old[i].node)
		}
	}
	for i := n; i < len(new); i++ {
		patchChild(doc, nil, new[i], parent)
	}
}

// patchKeyedChildren matches children by key. A keyed new child that
// matches an old child is patched in place and moved to its new position;
// an unmatched new child is mounted; and every unmatched old child is
// removed. Moving a matched child is a no-op when it already sits in the
// right place, so an unchanged list costs no DOM writes.
func patchKeyedChildren(doc *js.Object, old, new []*VNode, parent *js.Object) {
	oldByKey := make(map[string]*VNode, len(old))
	for _, oc := range old {
		if oc != nil && oc.Key != "" {
			oldByKey[oc.Key] = oc
		}
	}
	var prev *js.Object
	for _, nc := range new {
		if nc == nil {
			continue
		}
		var matched *VNode
		if nc.Key != "" {
			matched = oldByKey[nc.Key]
			if matched != nil {
				delete(oldByKey, nc.Key)
			}
		}
		var node *js.Object
		if matched != nil {
			node = patchChild(doc, matched, nc, parent)
		} else {
			node = mount(doc, nc)
			parent.Call("appendChild", node)
		}
		// Move node to right after prev, unless it is already there.
		if prev == nil {
			if first := parent.Get("firstChild"); first != node && first != nil {
				parent.Call("insertBefore", node, first)
			}
		} else if next := prev.Get("nextSibling"); node != next {
			parent.Call("insertBefore", node, next)
		}
		prev = node
	}
	// Remove the old children that no new child matched.
	for _, oc := range oldByKey {
		if oc.node != nil {
			if p := oc.node.Get("parentNode"); p != nil {
				p.Call("removeChild", oc.node)
			}
		}
	}
}
