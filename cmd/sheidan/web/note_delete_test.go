package main

import (
	"testing"

	"github.com/gopherjs/gopherjs/js"
)

// fakeDeleteDocument builds a plain JS object that stands in for the
// browser document, so the delete logic can be exercised under Node.js.
// querySelectorAll answers the title-row selector with the given title
// element and every other selector with an empty list.
func fakeDeleteDocument(titleElement *js.Object) *js.Object {
	doc := js.Global.Get("Object").New()
	doc.Set("head", fakeElement())
	doc.Set("querySelectorAll", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		if args[0].String() == "[data-field='title']" {
			list := js.Global.Get("Object").New()
			list.Set("0", titleElement)
			list.Set("length", 1)
			return list
		}
		empty := js.Global.Get("Object").New()
		empty.Set("length", 0)
		return empty
	}))
	return doc
}

// TestInstallDeleteButtons verifies that a delete button with the
// styling class and a trash glyph is added to each note's title row,
// and that the button's styles are injected into the page's head.
func TestInstallDeleteButtons(t *testing.T) {
	title := fakeElement()
	title.Set("getAttribute", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		if args[0].String() == "data-note-id" {
			return 7
		}
		return nil
	}))
	var button, style *js.Object
	doc := fakeDeleteDocument(title)
	doc.Set("createElement", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		el := fakeElement()
		switch args[0].String() {
		case "button":
			button = el
		case "style":
			style = el
		}
		return el
	}))
	installDeleteButtons(doc)
	if button == nil {
		t.Fatal("no delete button was created")
	}
	if style == nil {
		t.Fatal("no style element was created")
	}
	if got := button.Get("className").String(); got != "sf-delete" {
		t.Errorf("className = %q; want %q", got, "sf-delete")
	}
	if got := button.Get("type").String(); got != "button" {
		t.Errorf("type = %q; want %q", got, "button")
	}
	if got := button.Get("textContent").String(); got != "🗑️" {
		t.Errorf("textContent = %q; want %q", got, "🗑️")
	}
}

// TestRemoveNoteRows verifies that removeNoteRows queries the note's
// data-note-id and detaches each matching element through its parent.
func TestRemoveNoteRows(t *testing.T) {
	var selector string
	removed := 0
	element := js.Global.Get("Object").New()
	parent := js.Global.Get("Object").New()
	parent.Set("removeChild", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		removed++
		return nil
	}))
	element.Set("parentNode", parent)
	doc := js.Global.Get("Object").New()
	doc.Set("querySelectorAll", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		selector = args[0].String()
		list := js.Global.Get("Object").New()
		list.Set("0", element)
		list.Set("length", 1)
		return list
	}))
	removeNoteRows(doc, 42)
	if selector != "[data-note-id='42']" {
		t.Errorf("selector = %q; want %q", selector, "[data-note-id='42']")
	}
	if removed != 1 {
		t.Errorf("removed = %d; want 1", removed)
	}
}

// TestIsDetailPage verifies that the detail page is detected by the
// absence of a plain heading: the list page has an h1 without the
// sf-field class, and the detail page's heading is a field.
func TestIsDetailPage(t *testing.T) {
	// Detail page: no plain h1.
	docDetail := js.Global.Get("Object").New()
	docDetail.Set("querySelector", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return (*js.Object)(nil)
	}))
	if !isDetailPage(docDetail) {
		t.Error("isDetailPage = false; want true (no plain h1)")
	}

	// List page: a plain h1 is present.
	h1 := js.Global.Get("Object").New()
	docList := js.Global.Get("Object").New()
	docList.Set("querySelector", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return h1
	}))
	if isDetailPage(docList) {
		t.Error("isDetailPage = true; want false (plain h1 present)")
	}
}
