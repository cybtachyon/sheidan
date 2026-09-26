package main

import (
	"fmt"
	"net/http"

	"github.com/gopherjs/gopherjs/js"
)

// installDeleteButtons injects the delete button's styles and adds a
// delete button to each note's title row. The button sends a DELETE
// request for the note and, on success, removes the note's rows from
// the page. There is one button per note, not per row, because a note's
// title and body rows share the note's ID and the delete acts on the
// whole note.
func installDeleteButtons(doc *js.Object) {
	installDeleteStyles(doc)
	list := doc.Call("querySelectorAll", "[data-field='title']")
	for i := 0; i < list.Length(); i++ {
		element := list.Index(i)
		noteID := element.Call("getAttribute", "data-note-id").Int()
		button := doc.Call("createElement", "button")
		button.Set("className", "sf-delete")
		button.Set("type", "button")
		button.Set("textContent", "🗑️")
		button.Call("addEventListener", "click", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
			deleteNote(doc, noteID)
			return nil
		}))
		element.Call("appendChild", button)
	}
}

// installDeleteStyles appends a style element with the delete button's
// CSS to the page's head.
func installDeleteStyles(doc *js.Object) {
	style := doc.Call("createElement", "style")
	style.Set("textContent", deleteStyles)
	doc.Get("head").Call("appendChild", style)
}

// deleteStyles is the CSS for the delete button. It follows the edit
// button, hidden until the field is hovered. It lives in the client so
// the button's markup and styles are a single unit, because the button
// is built entirely in the browser.
const deleteStyles = `.sf-field .sf-delete { display: none; margin-left: 0.5em; }
.sf-field:hover .sf-delete { display: inline-block; }`

// deleteNote sends a DELETE request for the note and, on success,
// removes the note's rows from the page. On the note detail page, where
// the note is the whole page, it then navigates back to the notes list,
// because removing the rows would leave an empty page.
func deleteNote(doc *js.Object, noteID int) {
	// The http client rejects relative URLs, so the path is resolved
	// against the page's origin.
	url := pageOrigin() + fmt.Sprintf("/note/%d", noteID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		logError(err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logError(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		logError(fmt.Errorf("delete /note/%d: status %d", noteID, resp.StatusCode))
		return
	}
	removeNoteRows(doc, noteID)
	if isDetailPage(doc) {
		js.Global.Get("location").Set("href", pageOrigin()+"/notes")
	}
}

// removeNoteRows removes every element that carries the note's
// data-note-id from the page, so all of the note's rows disappear
// together. querySelectorAll returns a static list, so removing elements
// while iterating is safe.
func removeNoteRows(doc *js.Object, noteID int) {
	list := doc.Call("querySelectorAll", fmt.Sprintf("[data-note-id='%d']", noteID))
	for i := 0; i < list.Length(); i++ {
		element := list.Index(i)
		parent := element.Get("parentNode")
		if parent == nil {
			continue
		}
		parent.Call("removeChild", element)
	}
}

// isDetailPage reports whether the page is a note detail page, not the
// notes list page. The list page has a plain heading, an h1 without the
// sf-field class, and the detail page's heading is a field.
func isDetailPage(doc *js.Object) bool {
	return doc.Call("querySelector", "h1:not(.sf-field)") == nil
}
