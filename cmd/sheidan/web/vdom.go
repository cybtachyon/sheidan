package main

import "github.com/gopherjs/gopherjs/js"

// Attr is a named attribute on a virtual element.
type Attr struct {
	Name  string
	Value string
}

// Event is a named event listener on a virtual element. The handler is a
// native JavaScript function. It is attached when the element is mounted
// and re-attached whenever a later render reuses the element, so the
// element's behavior always matches its current render.
type Event struct {
	Name string
	Fn   *js.Object
}

// VNode is a node in a virtual DOM tree. An element node has a Tag and
// children; a text node has an empty Tag and a Text. Key is an optional
// identity used to match a child across renders, so a list item keeps its
// real DOM node when it moves. node holds the real DOM node the vdom node
// is mounted to; patching sets it, and it stays nil until then.
type VNode struct {
	Key      string
	Tag      string
	Text     string
	Attrs    []Attr
	Children []*VNode
	Events   []Event
	node     *js.Object
}

// El builds an element vdom node with the given tag, attributes, and
// children.
func El(tag string, attrs []Attr, children ...*VNode) *VNode {
	return &VNode{Tag: tag, Attrs: attrs, Children: children}
}

// Text builds a text vdom node.
func Text(text string) *VNode {
	return &VNode{Text: text}
}

// Keyed sets v's key and returns v. Use it when building a list of vdom
// children that should be matched by identity, not position, across
// renders.
func Keyed(v *VNode, key string) *VNode {
	v.Key = key
	return v
}

// WithEvents sets the event listeners on v and returns v.
func WithEvents(v *VNode, events ...Event) *VNode {
	v.Events = append(v.Events, events...)
	return v
}
