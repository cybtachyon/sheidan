package main

import "github.com/gopherjs/gopherjs/js"

// fakeOps counts the element-level DOM writes the fake performs
// (appendChild, insertBefore, removeChild, replaceChild, setAttribute,
// removeAttribute), so tests can verify the patch applies only the
// minimal changes. fakeCreates counts the nodes the fake builds, so
// tests can verify a re-render reuses existing nodes instead of
// rebuilding them. Both are shared across tests in the package, so a
// test that reads one resets it first.
var (
	fakeOps     int
	fakeCreates int
)

// fakeOpsCount returns and resets the operation counter.
func fakeOpsCount() int {
	n := fakeOps
	fakeOps = 0
	return n
}

// fakeCreatesCount returns and resets the node-creation counter.
func fakeCreatesCount() int {
	n := fakeCreates
	fakeCreates = 0
	return n
}

// The fake DOM is a small in-memory DOM built from plain JavaScript
// objects, so the patch and component logic can be exercised under
// Node.js, where there is no real document. It maintains a real child
// tree (children, firstChild, nextSibling, previousElementSibling,
// parentNode), so the reconciliation's moves and removals can be
// verified against the resulting structure.
//
// Pointer properties are always set to an explicit null, never left
// unset, because a never-set property is JavaScript's undefined, which
// does not compare equal to nil in the js package.

// fakeDocument builds a stand-in for the browser document.
func fakeDocument() *js.Object {
	doc := js.Global.Get("Object").New()
	doc.Set("head", fakeElement("head"))
	doc.Set("createElement", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return fakeElement(args[0].String())
	}))
	doc.Set("createTextNode", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return fakeTextNode(args[0].String())
	}))
	return doc
}

// fakeElement builds a stand-in for a DOM element with a child tree, an
// attribute map, an event-listener map, and a value property for inputs.
func fakeElement(tag string) *js.Object {
	fakeCreates++
	el := js.Global.Get("Object").New()
	el.Set("tagName", tag)
	el.Set("children", js.Global.Get("Array").New())
	el.Set("parentNode", (*js.Object)(nil))
	el.Set("firstChild", (*js.Object)(nil))
	el.Set("nextSibling", (*js.Object)(nil))
	el.Set("previousElementSibling", (*js.Object)(nil))
	el.Set("attrs", js.Global.Get("Object").New())
	el.Set("eventListeners", js.Global.Get("Object").New())
	el.Set("value", "")

	el.Set("appendChild", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return fakeAppend(el, args[0])
	}))
	el.Set("insertBefore", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return fakeInsertBefore(el, args[0], args[1])
	}))
	el.Set("removeChild", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return fakeRemoveChild(el, args[0])
	}))
	el.Set("replaceChild", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return fakeReplaceChild(el, args[0], args[1])
	}))
	el.Set("setAttribute", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		fakeOps++
		el.Get("attrs").Set(args[0].String(), args[1].String())
		// The value content attribute is the input's default value,
		// and on a fresh input the current value follows it.
		if args[0].String() == "value" {
			el.Set("value", args[1].String())
		}
		return nil
	}))
	el.Set("removeAttribute", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		fakeOps++
		el.Get("attrs").Delete(args[0].String())
		return nil
	}))
	el.Set("getAttribute", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		v := el.Get("attrs").Get(args[0].String())
		if v == nil || v == js.Undefined {
			return (*js.Object)(nil)
		}
		return v
	}))
	el.Set("addEventListener", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		el.Get("eventListeners").Set(args[0].String(), args[1])
		return nil
	}))
	el.Set("removeEventListener", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		el.Get("eventListeners").Delete(args[0].String())
		return nil
	}))
	el.Set("querySelector", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return fakeQuerySelector(el, args[0].String())
	}))
	return el
}

// fakeTextNode builds a stand-in for a DOM text node.
func fakeTextNode(text string) *js.Object {
	fakeCreates++
	node := js.Global.Get("Object").New()
	node.Set("tagName", "")
	node.Set("nodeValue", text)
	node.Set("parentNode", (*js.Object)(nil))
	node.Set("nextSibling", (*js.Object)(nil))
	return node
}

// fakeQuerySelector returns the first descendant element of el whose
// class attribute equals the class named by a ".class" selector, or
// null. It supports the class selectors the client uses and nothing
// more.
func fakeQuerySelector(el *js.Object, selector string) *js.Object {
	if len(selector) == 0 || selector[0] != '.' {
		return (*js.Object)(nil)
	}
	class := selector[1:]
	children := el.Get("children")
	for i := 0; i < children.Get("length").Int(); i++ {
		child := children.Index(i)
		if child.Get("tagName").String() == "" {
			continue
		}
		if v := child.Get("attrs").Get("class"); v != nil && v != js.Undefined && v.String() == class {
			return child
		}
		if found := fakeQuerySelector(child, selector); found != nil {
			return found
		}
	}
	return (*js.Object)(nil)
}

// relink rebuilds el's firstChild and the children's sibling chain from
// the children array.
func relink(el *js.Object) {
	children := el.Get("children")
	n := children.Get("length").Int()
	if n == 0 {
		el.Set("firstChild", (*js.Object)(nil))
		return
	}
	el.Set("firstChild", children.Index(0))
	var prevElement *js.Object
	for i := 0; i < n; i++ {
		child := children.Index(i)
		if i+1 < n {
			child.Set("nextSibling", children.Index(i+1))
		} else {
			child.Set("nextSibling", (*js.Object)(nil))
		}
		if child.Get("tagName").String() != "" {
			child.Set("previousElementSibling", prevElement)
			prevElement = child
		}
	}
}

// fakeAppend appends child to el, detaching it from any current parent.
// When a text child joins a textarea, the element's value is
// rederived from its text children, mirroring how a browser
// initializes a textarea's value from its content.
func fakeAppend(el, child *js.Object) *js.Object {
	fakeOps++
	if p := child.Get("parentNode"); p != nil {
		fakeRemoveChild(p, child)
	}
	el.Get("children").Call("push", child)
	child.Set("parentNode", el)
	if el.Get("tagName").String() == "textarea" && child.Get("tagName").String() == "" {
		el.Set("value", fakeTextView(el))
	}
	relink(el)
	return child
}

// fakeTextView joins the data of el's text children.
func fakeTextView(el *js.Object) string {
	out := ""
	kids := el.Get("children")
	for i := 0; i < kids.Get("length").Int(); i++ {
		kid := kids.Index(i)
		if kid.Get("tagName").String() == "" {
			out += kid.Get("nodeValue").String()
		}
	}
	return out
}

// fakeInsertBefore inserts child into el before ref, or appends it when
// ref is null. It detaches child from any current parent first.
func fakeInsertBefore(el, child, ref *js.Object) *js.Object {
	fakeOps++
	if p := child.Get("parentNode"); p != nil {
		fakeRemoveChild(p, child)
	}
	children := el.Get("children")
	if ref == nil {
		children.Call("push", child)
	} else {
		children.Call("splice", fakeIndexOf(el, ref), 0, child)
	}
	child.Set("parentNode", el)
	relink(el)
	return child
}

// fakeRemoveChild detaches child from el.
func fakeRemoveChild(el, child *js.Object) *js.Object {
	fakeOps++
	idx := fakeIndexOf(el, child)
	if idx >= 0 {
		el.Get("children").Call("splice", idx, 1)
	}
	child.Set("parentNode", (*js.Object)(nil))
	relink(el)
	return child
}

// fakeReplaceChild swaps oldChild for newChild in el, keeping
// newChild's position.
func fakeReplaceChild(el, newChild, oldChild *js.Object) *js.Object {
	fakeOps++
	idx := fakeIndexOf(el, oldChild)
	if idx >= 0 {
		el.Get("children").Call("splice", idx, 1, newChild)
	} else {
		el.Get("children").Call("push", newChild)
	}
	newChild.Set("parentNode", el)
	relink(el)
	return oldChild
}

// fakeIndexOf returns the index of child in el's children, or -1.
func fakeIndexOf(el, child *js.Object) int {
	children := el.Get("children")
	for i := 0; i < children.Get("length").Int(); i++ {
		if children.Index(i) == child {
			return i
		}
	}
	return -1
}

// fakeChildTags returns the child element tags of el in order, for
// assertions. Text nodes are reported by their nodeValue.
func fakeChildTags(el *js.Object) []string {
	children := el.Get("children")
	out := make([]string, children.Get("length").Int())
	for i := 0; i < len(out); i++ {
		child := children.Index(i)
		if child.Get("tagName").String() == "" {
			out[i] = "text:" + child.Get("nodeValue").String()
		} else {
			out[i] = child.Get("tagName").String()
		}
	}
	return out
}

// fakeEvent builds a stand-in for a browser event object: it names the
// element it was dispatched to and carries a preventDefault method,
// because handlers may call it.
func fakeEvent(target *js.Object) *js.Object {
	event := js.Global.Get("Object").New()
	event.Set("target", target)
	event.Set("key", "")
	event.Set("preventDefault", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		event.Set("defaultPrevented", true)
		return nil
	}))
	return event
}

// fakeEmit calls the listener el registered for name with this bound to
// el, simulating the browser dispatching the event to the element.
func fakeEmit(el *js.Object, name string, event *js.Object) {
	fn := el.Get("eventListeners").Get(name)
	if fn == nil || fn == js.Undefined {
		return
	}
	// fn.call(el, event) binds this to el, as the browser does.
	fn.Call("call", el, event)
}

// fakeClick simulates a click on el.
func fakeClick(el *js.Object) {
	fakeEmit(el, "click", fakeEvent(el))
}
