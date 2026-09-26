package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gopherjs/gopherjs/js"
)

// Field is an editable note field in the browser. It wraps a DOM
// element that displays a note's title or body value. The element
// shows an edit button on hover; clicking the button switches the
// element to edit mode, a text input with a submit button; submitting
// updates the note through the demo app's update endpoint.
type Field struct {
	element *js.Object // element with the data-field attribute
	value   *js.Object // span holding the field's displayed value
	input   *js.Object // text input, shown in edit mode
	noteID  int
	name    string // "title" or "body"
}

// NewField creates a Field from a DOM element with data-field and
// data-note-id attributes.
func NewField(element *js.Object) *Field {
	return &Field{
		element: element,
		value:   element.Call("querySelector", ".sf-value"),
		noteID:  element.Call("getAttribute", "data-note-id").Int(),
		name:    element.Call("getAttribute", "data-field").String(),
	}
}

// init appends the edit controls to the field's element and wires up
// their event handlers.
func (f *Field) init() {
	doc := js.Global.Get("document")

	edit := doc.Call("createElement", "button")
	edit.Set("className", "sf-edit")
	edit.Set("textContent", "✏️")
	edit.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		f.enterEdit()
		return nil
	}))
	f.element.Call("appendChild", edit)

	f.input = doc.Call("createElement", "input")
	f.input.Set("className", "sf-input")
	f.element.Call("appendChild", f.input)

	submit := doc.Call("createElement", "button")
	submit.Set("className", "sf-submit")
	submit.Set("textContent", "✔️")
	submit.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		f.submit()
		return nil
	}))
	f.element.Call("appendChild", submit)

	// Enter in the input submits, like a form.
	f.input.Call("addEventListener", "keydown", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		if args[0].Get("key").String() == "Enter" {
			f.submit()
		}
		return nil
	}))
}

// enterEdit switches the field to edit mode: the value and edit
// button hide, and the input and submit button show.
func (f *Field) enterEdit() {
	f.input.Set("value", f.value.Get("textContent").String())
	f.element.Set("className", "sf-field editing")
	f.input.Call("focus")
	f.input.Call("select")
}

// exitEdit switches the field back to display mode with the given
// value.
func (f *Field) exitEdit(value string) {
	f.value.Set("textContent", value)
	f.element.Set("className", "sf-field")
}

// submit sends the input's value to the demo app's update endpoint
// and, on success, switches the field back to display mode with the
// updated value.
func (f *Field) submit() {
	value := f.input.Get("value").String()
	if value == f.value.Get("textContent").String() {
		f.exitEdit(value)
		return
	}
	body, err := json.Marshal(map[string]string{f.name: value})
	if err != nil {
		f.logError(err)
		return
	}
	// The http client rejects relative URLs, so the path is resolved
	// against the page's origin.
	url := pageOrigin() + fmt.Sprintf("/note/%d", f.noteID)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		f.logError(err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.logError(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		f.logError(fmt.Errorf("update /note/%d: status %d", f.noteID, resp.StatusCode))
		return
	}
	if err := f.applyResponse(resp.Body); err != nil {
		f.logError(err)
	}
}

// applyResponse decodes the note returned by the update endpoint and
// switches the field back to display mode with the updated value.
func (f *Field) applyResponse(body io.Reader) error {
	var note struct {
		Title string
		Body  string
	}
	if err := json.NewDecoder(body).Decode(&note); err != nil {
		return err
	}
	if f.name == "title" {
		f.exitEdit(note.Title)
	} else {
		f.exitEdit(note.Body)
	}
	return nil
}

// pageOrigin returns the page's origin, the scheme and host part of
// the page's URL.
func pageOrigin() string {
	return js.Global.Get("location").Get("origin").String()
}

// logError reports an error to the browser console.
func (f *Field) logError(err error) {
	js.Global.Get("console").Call("error", err.Error())
}
