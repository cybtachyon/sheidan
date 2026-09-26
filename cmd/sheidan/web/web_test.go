package main

import (
	"strings"
	"testing"

	"github.com/gopherjs/gopherjs/js"
)

// TestMainNoDocument verifies that main is a no-op without a DOM, so
// the transpiled build can run under Node.js for tests.
func TestMainNoDocument(t *testing.T) {
	main()
}

// fakeField builds a Field with plain JS objects standing in for DOM
// elements, so the field logic can be exercised under Node.js.
func fakeField(name string) *Field {
	element := js.Global.Get("Object").New()
	element.Set("className", "sf-field")
	value := js.Global.Get("Object").New()
	value.Set("textContent", "old value")
	input := js.Global.Get("Object").New()
	input.Set("value", "")
	// enterEdit calls focus and select on the input.
	input.Set("focus", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return nil
	}))
	input.Set("select", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		return nil
	}))
	return &Field{element: element, value: value, input: input, name: name}
}

// TestEnterEdit verifies that edit mode prefills the input with the
// displayed value and toggles the element's class.
func TestEnterEdit(t *testing.T) {
	f := fakeField("title")
	f.enterEdit()
	if got := f.input.Get("value").String(); got != "old value" {
		t.Errorf("input value = %q, want %q", got, "old value")
	}
	if got := f.element.Get("className").String(); got != "sf-field editing" {
		t.Errorf("className = %q, want %q", got, "sf-field editing")
	}
}

// TestNewFieldWithViewAnchor verifies that NewField finds the value
// span when the list page wraps it in a view anchor, so the anchor does
// not break the field's edit mode.
func TestNewFieldWithViewAnchor(t *testing.T) {
	value := js.Global.Get("Object").New()
	value.Set("textContent", "linked title")
	element := js.Global.Get("Object").New()
	element.Set("querySelector", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		if args[0].String() == ".sf-value" {
			return value
		}
		return (*js.Object)(nil)
	}))
	element.Set("getAttribute", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		switch args[0].String() {
		case "data-note-id":
			return 7
		case "data-field":
			return "title"
		}
		return nil
	}))
	f := NewField(element)
	if got := f.value.Get("textContent").String(); got != "linked title" {
		t.Errorf("value textContent = %q; want %q", got, "linked title")
	}
	if f.noteID != 7 {
		t.Errorf("noteID = %d; want 7", f.noteID)
	}
	if f.name != "title" {
		t.Errorf("name = %q; want %q", f.name, "title")
	}
}

// TestApplyResponse verifies that the update response decodes and
// updates the field's displayed value, then exits edit mode.
func TestApplyResponse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		expect string
	}{
		{"title", `{"Title":"new title","Body":"body"}`, "new title"},
		{"body", `{"Title":"title","Body":"new body"}`, "new body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fakeField(tc.name)
			if err := f.applyResponse(strings.NewReader(tc.body)); err != nil {
				t.Fatal(err)
			}
			if got := f.value.Get("textContent").String(); got != tc.expect {
				t.Errorf("textContent = %q, want %q", got, tc.expect)
			}
			if got := f.element.Get("className").String(); got != "sf-field" {
				t.Errorf("className = %q, want %q", got, "sf-field")
			}
		})
	}
}
