package main

import (
	"encoding/json"
	"testing"

	"github.com/gopherjs/gopherjs/js"
)

// fakeElement builds a plain JS object that stands in for a DOM
// element, with no-op methods for the operations the client calls on
// it.
func fakeElement() *js.Object {
	el := js.Global.Get("Object").New()
	el.Set("appendChild", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return nil
	}))
	el.Set("addEventListener", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return nil
	}))
	el.Set("insertAdjacentElement", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return nil
	}))
	return el
}

// fakeDocument builds a plain JS object that stands in for the browser
// document, so the form construction can be exercised under Node.js.
func fakeDocument() *js.Object {
	doc := js.Global.Get("Object").New()
	doc.Set("head", fakeElement())
	doc.Set("createElement", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return fakeElement()
	}))
	doc.Set("querySelector", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return (*js.Object)(nil)
	}))
	return doc
}

// fakeNoteForm builds a NoteForm with plain JS objects standing in for
// the form's elements, so the payload logic can be exercised under
// Node.js.
func fakeNoteForm() *NoteForm {
	title := js.Global.Get("Object").New()
	title.Set("value", "new title")
	body := js.Global.Get("Object").New()
	body.Set("value", "new body")
	return &NoteForm{element: fakeElement(), title: title, body: body}
}

// TestNewNoteForm verifies that the form is built with a title input,
// a body input, and a submit button, and that the elements carry the
// styling class and placeholders.
func TestNewNoteForm(t *testing.T) {
	n := NewNoteForm(fakeDocument())
	if n.element == nil || n.title == nil || n.body == nil {
		t.Fatal("expected non-nil form, title, and body elements")
	}
	if got := n.element.Get("className").String(); got != "sf-new-note" {
		t.Errorf("form className = %q; want %q", got, "sf-new-note")
	}
	if got := n.title.Get("placeholder").String(); got != "Title" {
		t.Errorf("title placeholder = %q; want %q", got, "Title")
	}
	if got := n.body.Get("placeholder").String(); got != "Body" {
		t.Errorf("body placeholder = %q; want %q", got, "Body")
	}
}

// TestNoteFormPayload verifies that the create request's JSON body
// carries the form's title and body values under the keys the create
// endpoint expects.
func TestNoteFormPayload(t *testing.T) {
	n := fakeNoteForm()
	body, err := n.payload()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["Title"] != "new title" {
		t.Errorf("Title = %q; want %q", got["Title"], "new title")
	}
	if got["Body"] != "new body" {
		t.Errorf("Body = %q; want %q", got["Body"], "new body")
	}
}

// TestNullObjectCheck verifies that a *js.Object holding JavaScript's
// null compares equal to nil, the idiom installNewNoteForm uses to
// detect a missing heading.
func TestNullObjectCheck(t *testing.T) {
	fn := js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return (*js.Object)(nil)
	})
	if result := fn.Invoke(); result != nil {
		t.Errorf("result = %v; want nil (JavaScript null)", result)
	}
}
