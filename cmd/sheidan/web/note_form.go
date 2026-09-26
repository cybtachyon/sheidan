package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gopherjs/gopherjs/js"
)

// NoteForm is the form for creating a new note on the notes list page.
// The form is built in the browser, because the create feature needs
// JavaScript: the create endpoint accepts only a JSON body, which a
// plain HTML form cannot send. Submitting the form POSTs the title and
// body to the demo app's create endpoint and, on success, reloads the
// page, so the server template stays the single source of truth for the
// note rows.
type NoteForm struct {
	element *js.Object // the form element
	title   *js.Object // the title input
	body    *js.Object // the body input
}

// NewNoteForm builds the new-note form's elements from the document and
// wires the form's submit handler.
func NewNoteForm(doc *js.Object) *NoteForm {
	form := doc.Call("createElement", "form")
	form.Set("className", "sf-new-note")

	title := doc.Call("createElement", "input")
	title.Set("className", "sf-new-title")
	title.Set("type", "text")
	title.Set("placeholder", "Title")
	form.Call("appendChild", title)

	body := doc.Call("createElement", "input")
	body.Set("className", "sf-new-body")
	body.Set("type", "text")
	body.Set("placeholder", "Body")
	form.Call("appendChild", body)

	submit := doc.Call("createElement", "button")
	submit.Set("className", "sf-new-submit")
	submit.Set("type", "submit")
	submit.Set("textContent", "Add")
	form.Call("appendChild", submit)

	n := &NoteForm{element: form, title: title, body: body}

	// The default form submission sends an HTML-encoded body, which the
	// create endpoint rejects, so the handler sends JSON instead.
	form.Call("addEventListener", "submit", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		args[0].Call("preventDefault")
		n.submit()
		return nil
	}))
	return n
}

// installNewNoteForm injects the form's styles, builds the form, and
// inserts it after the page's heading, so it sits above the notes list.
func installNewNoteForm(doc *js.Object) {
	installFormStyles(doc)
	form := NewNoteForm(doc)
	heading := doc.Call("querySelector", "h1")
	if heading == nil {
		return
	}
	heading.Call("insertAdjacentElement", "afterend", form.element)
}

// installFormStyles appends a style element with the form's CSS to the
// page's head.
func installFormStyles(doc *js.Object) {
	style := doc.Call("createElement", "style")
	style.Set("textContent", formStyles)
	doc.Get("head").Call("appendChild", style)
}

// formStyles is the CSS for the new-note form. It lives in the client
// so the form's markup and styles are a single unit, because the form
// is built entirely in the browser.
const formStyles = `.sf-new-note { margin: 0 0 1em; }
.sf-new-note input { font: inherit; padding: 0.25em 0.5em; margin-right: 0.5em; }
.sf-new-note button { font: inherit; padding: 0.25em 0.75em; }`

// payload builds the JSON body for the create request from the form's
// inputs.
func (n *NoteForm) payload() ([]byte, error) {
	return json.Marshal(map[string]string{
		"Title": n.title.Get("value").String(),
		"Body":  n.body.Get("value").String(),
	})
}

// submit sends the form's title and body to the create endpoint and,
// on success, reloads the page so the server re-renders the list with
// the new note.
func (n *NoteForm) submit() {
	body, err := n.payload()
	if err != nil {
		logError(err)
		return
	}
	url := pageOrigin() + "/notes"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logError(err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logError(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		logError(fmt.Errorf("create note: status %d", resp.StatusCode))
		return
	}
	n.reload()
}

// reload reloads the page, so the server re-renders the notes list with
// the created note.
func (n *NoteForm) reload() {
	js.Global.Get("location").Call("reload")
}
