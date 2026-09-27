package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gopherjs/gopherjs/js"
)

// App is the reactive front-end of the demo app. It owns the note
// store, the scheduler, and the page component. The page is chosen by
// the browser's path: /notes renders the list page, and /note/<id>
// renders the detail page. Every mutation goes through the store, and
// the store schedules the page's re-render, batched by the scheduler.
type App struct {
	doc       *js.Object
	container *js.Object
	store     *NoteStore
	sched     *Scheduler
	page      *Component
	noteID    int // the note the detail page shows; 0 on the list page
	editing   map[string]bool
	loadErr   string
	// send sends an HTTP request to the demo app. The browser uses the
	// real request, and tests inject a stub.
	send func(method, path string, body []byte) (*http.Response, error)
}

// NewApp creates an app bound to the page's document and container.
func NewApp(doc, container *js.Object) *App {
	return &App{
		doc:       doc,
		container: container,
		store:     NewNoteStore(),
		sched:     NewScheduler(),
		editing:   map[string]bool{},
		send:      request,
	}
}

// run installs the app's styles, picks the page from the browser's
// path, subscribes the page to the store, renders it, and loads its
// data.
func (a *App) run() {
	a.installStyles()
	a.noteID = a.noteIDFromPath()
	a.page = NewComponent(a.sched, a.doc, a.container, a.renderPage)
	a.store.Subscribe(a.page.schedule)
	a.page.update()
	if a.noteID != 0 {
		a.loadNote()
	} else {
		a.loadNotes()
	}
}

// noteIDFromPath returns the note ID from the browser's path, or 0
// when the path is not a note detail page.
func (a *App) noteIDFromPath() int {
	path := js.Global.Get("location").Get("pathname").String()
	if !strings.HasPrefix(path, "/note/") {
		return 0
	}
	id, _ := strconv.Atoi(strings.TrimPrefix(path, "/note/"))
	return id
}

// noteURL returns the path of a note's detail page.
func noteURL(id int) string {
	return fmt.Sprintf("/note/%d", id)
}

// loadNotes fetches the note list and loads it into the store.
func (a *App) loadNotes() {
	resp, err := a.send(http.MethodGet, "/notes", nil)
	if err != nil {
		logError(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logError(fmt.Errorf("list notes: status %d", resp.StatusCode))
		return
	}
	notes, err := decodeNotes(resp.Body)
	if err != nil {
		logError(err)
		return
	}
	a.store.SetAll(notes)
}

// loadNote fetches the detail page's note and upserts it into the
// store. A failed load sets the page's load error, which the render
// shows.
func (a *App) loadNote() {
	resp, err := a.send(http.MethodGet, noteURL(a.noteID), nil)
	if err != nil {
		a.setLoadError(err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		a.setLoadError(fmt.Sprintf("load %s: status %d", noteURL(a.noteID), resp.StatusCode))
		return
	}
	note, err := decodeNote(resp.Body)
	if err != nil {
		a.setLoadError(err.Error())
		return
	}
	a.store.Upsert(note)
}

// setLoadError records the detail page's load failure and schedules
// the re-render that shows it.
func (a *App) setLoadError(msg string) {
	a.loadErr = msg
	a.page.schedule()
}

// addNote posts a new note to the create endpoint and, on success,
// stores it and calls onSuccess.
func (a *App) addNote(title, body string, onSuccess func()) {
	payload, err := json.Marshal(map[string]string{"Title": title, "Body": body})
	if err != nil {
		logError(err)
		return
	}
	resp, err := a.send(http.MethodPost, "/notes", payload)
	if err != nil {
		logError(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		logError(fmt.Errorf("create note: status %d", resp.StatusCode))
		return
	}
	note, err := decodeNote(resp.Body)
	if err != nil {
		logError(err)
		return
	}
	a.store.Add(note)
	if onSuccess != nil {
		onSuccess()
	}
}

// saveField sends the field's value to the update endpoint and, on
// success, stores the updated note and leaves edit mode.
func (a *App) saveField(noteID int, field, value string) {
	name := "Title"
	if field == "body" {
		name = "Body"
	}
	payload, err := json.Marshal(map[string]string{name: value})
	if err != nil {
		logError(err)
		return
	}
	resp, err := a.send(http.MethodPatch, noteURL(noteID), payload)
	if err != nil {
		logError(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logError(fmt.Errorf("update %s: status %d", noteURL(noteID), resp.StatusCode))
		return
	}
	note, err := decodeNote(resp.Body)
	if err != nil {
		logError(err)
		return
	}
	a.store.Upsert(note)
	a.setEditing(noteID, field, false)
}

// deleteNote sends a delete request for the note and, on success,
// removes it from the store. On the detail page it navigates back to
// the list.
func (a *App) deleteNote(noteID int) {
	resp, err := a.send(http.MethodDelete, noteURL(noteID), nil)
	if err != nil {
		logError(err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		logError(fmt.Errorf("delete %s: status %d", noteURL(noteID), resp.StatusCode))
		return
	}
	a.store.Remove(noteID)
	if a.noteID == noteID {
		js.Global.Get("location").Set("href", "/notes")
	}
}

// editKey keys the editing state by note ID and field name.
func editKey(noteID int, field string) string {
	return fmt.Sprintf("%d:%s", noteID, field)
}

// isEditing reports whether the note's field is in edit mode.
func (a *App) isEditing(noteID int, field string) bool {
	return a.editing[editKey(noteID, field)]
}

// setEditing switches the note's field into or out of edit mode and
// schedules the page's re-render.
func (a *App) setEditing(noteID int, field string, on bool) {
	a.editing[editKey(noteID, field)] = on
	a.page.schedule()
}

// renderPage renders the page the app's path selects.
func (a *App) renderPage() []*VNode {
	if a.noteID != 0 {
		return a.renderDetail()
	}
	return a.renderList()
}

// renderList renders the notes list page: the heading, the new-note
// form, and a container holding one row per note. The rows are keyed by
// note ID so a note keeps its DOM node across re-renders, and they live
// in their own container because the keyed reconciliation cannot mix
// keyed rows with the unkeyed heading and form.
func (a *App) renderList() []*VNode {
	notes := a.store.Notes()
	rows := make([]*VNode, 0, len(notes))
	for _, note := range notes {
		rows = append(rows, a.listRow(note))
	}
	return []*VNode{
		El("h1", nil, Text("Notes")),
		a.newNoteForm(),
		El("div", []Attr{{Name: "class", Value: "sf-note-list"}}, rows...),
	}
}

// listRow renders one note of the list: its title as a link to the
// detail page, its body, and the edit and delete controls.
func (a *App) listRow(note Note) *VNode {
	return Keyed(
		El("div", []Attr{{Name: "class", Value: "sf-note"}},
			a.listTitleRow(note),
			a.listBodyRow(note),
		),
		strconv.Itoa(note.ID))
}

// listTitleRow renders a list note's title: a link to the detail
// page, an edit button, and a delete button, or the edit controls
// when the title is in edit mode.
func (a *App) listTitleRow(note Note) *VNode {
	if a.isEditing(note.ID, "title") {
		return a.editRow("h2", note, "title")
	}
	link := El("a", []Attr{{Name: "href", Value: noteURL(note.ID)}}, Text(note.Title))
	return El("h2", nil, link, a.editButton(note.ID, "title"), a.deleteButton(note.ID))
}

// listBodyRow renders a list note's body with an edit button, or the
// edit controls when the body is in edit mode.
func (a *App) listBodyRow(note Note) *VNode {
	if a.isEditing(note.ID, "body") {
		return a.editRow("p", note, "body")
	}
	return El("p", []Attr{{Name: "class", Value: "sf-body"}}, Text(note.Body), a.editButton(note.ID, "body"))
}

// renderDetail renders the note detail page: a back link, the note's
// title and body, and a delete button. Before the store has loaded
// the note it shows a placeholder, and a failed load shows an error.
func (a *App) renderDetail() []*VNode {
	if a.loadErr != "" {
		return []*VNode{
			a.backLink(),
			El("h1", nil, Text("Note not found")),
		}
	}
	note := a.note()
	if note == nil {
		return []*VNode{Text("Loading...")}
	}
	return []*VNode{
		a.backLink(),
		El("div", []Attr{{Name: "class", Value: "sf-note"}},
			a.detailTitleRow(*note),
			a.detailBodyRow(*note),
			a.detailDeleteButton(*note),
		),
	}
}

// note returns the detail page's note from the store, or nil before
// the store has loaded it.
func (a *App) note() *Note {
	for i := range a.store.Notes() {
		if a.store.Notes()[i].ID == a.noteID {
			return &a.store.Notes()[i]
		}
	}
	return nil
}

// backLink renders the link back to the notes list.
func (a *App) backLink() *VNode {
	return El("p", nil, El("a", []Attr{{Name: "href", Value: "/notes"}}, Text("Back to notes")))
}

// detailTitleRow renders the detail page's title with an edit button,
// or the edit controls when the title is in edit mode.
func (a *App) detailTitleRow(note Note) *VNode {
	if a.isEditing(note.ID, "title") {
		return a.editRow("h1", note, "title")
	}
	return El("h1", nil, Text(note.Title), a.editButton(note.ID, "title"))
}

// detailBodyRow renders the detail page's body with an edit button,
// or the edit controls when the body is in edit mode.
func (a *App) detailBodyRow(note Note) *VNode {
	if a.isEditing(note.ID, "body") {
		return a.editRow("p", note, "body")
	}
	return El("p", []Attr{{Name: "class", Value: "sf-body"}}, Text(note.Body), a.editButton(note.ID, "body"))
}

// detailDeleteButton renders the detail page's delete button.
func (a *App) detailDeleteButton(note Note) *VNode {
	btn := El("button", []Attr{{Name: "class", Value: "sf-delete"}}, Text("Delete"))
	WithEvents(btn, Event{Name: "click", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		a.deleteNote(note.ID)
		return nil
	})})
	return btn
}

// editRow renders a note field in edit mode and the save and cancel
// buttons beside it. The value is read from the real element at submit
// time, so a re-render never clobbers what the user has typed.
func (a *App) editRow(tag string, note Note, field string) *VNode {
	save := El("button", []Attr{{Name: "class", Value: "sf-save"}}, Text("Save"))
	WithEvents(save, Event{Name: "click", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		a.saveField(note.ID, field, this.Get("previousElementSibling").Get("value").String())
		return nil
	})})
	cancel := El("button", []Attr{{Name: "class", Value: "sf-cancel"}}, Text("Cancel"))
	WithEvents(cancel, Event{Name: "click", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		a.setEditing(note.ID, field, false)
		return nil
	})})
	return El(tag, nil, a.fieldControl(note, field), save, cancel)
}

// fieldControl builds the control for editing a note field. The title
// control is a single-line input; Enter in it saves, like a form. The
// body control is a textarea seeded with the current body; Enter in a
// textarea inserts a line break, so the body saves through the save
// button alone.
func (a *App) fieldControl(note Note, field string) *VNode {
	if field == "body" {
		return TextArea([]Attr{{Name: "class", Value: "sf-input"}}, note.Body)
	}
	input := El("input", []Attr{
		{Name: "class", Value: "sf-input"},
		{Name: "type", Value: "text"},
		{Name: "value", Value: note.Title},
	})
	// Enter in the input saves, like a form.
	WithEvents(input, Event{Name: "keydown", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		if args[0].Get("key").String() == "Enter" {
			a.saveField(note.ID, field, this.Get("value").String())
		}
		return nil
	})})
	return input
}

// editButton renders the button that switches a note field into edit
// mode.
func (a *App) editButton(noteID int, field string) *VNode {
	btn := El("button", []Attr{{Name: "class", Value: "sf-edit"}}, Text("Edit"))
	WithEvents(btn, Event{Name: "click", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		a.setEditing(noteID, field, true)
		return nil
	})})
	return btn
}

// deleteButton renders the button that deletes a note from the list.
func (a *App) deleteButton(noteID int) *VNode {
	btn := El("button", []Attr{{Name: "class", Value: "sf-delete"}}, Text("Delete"))
	WithEvents(btn, Event{Name: "click", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		a.deleteNote(noteID)
		return nil
	})})
	return btn
}

// newNoteForm renders the new-note form: a title input, a body textarea,
// and an add button. Submitting posts the note to the create endpoint.
func (a *App) newNoteForm() *VNode {
	title := El("input", []Attr{
		{Name: "class", Value: "sf-new-title"},
		{Name: "type", Value: "text"},
		{Name: "placeholder", Value: "Title"},
	})
	// The body is a textarea without a seed child: the form's content
	// is scratch, it never projects store state, so a re-render cannot
	// touch what the user has typed.
	body := El("textarea", []Attr{
		{Name: "class", Value: "sf-new-body"},
		{Name: "placeholder", Value: "Body"},
	})
	add := El("button", []Attr{
		{Name: "class", Value: "sf-new-submit"},
		{Name: "type", Value: "submit"},
	}, Text("Add"))
	form := El("form", []Attr{{Name: "class", Value: "sf-new-note"}}, title, body, add)
	WithEvents(form, Event{Name: "submit", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		args[0].Call("preventDefault")
		titleEl := this.Call("querySelector", ".sf-new-title")
		bodyEl := this.Call("querySelector", ".sf-new-body")
		a.addNote(titleEl.Get("value").String(), bodyEl.Get("value").String(), func() {
			titleEl.Set("value", "")
			bodyEl.Set("value", "")
		})
		return nil
	})})
	return form
}

// installStyles appends the app's CSS to the page's head.
func (a *App) installStyles() {
	style := a.doc.Call("createElement", "style")
	style.Set("textContent", appStyles)
	a.doc.Get("head").Call("appendChild", style)
}

// appStyles is the CSS for the app's interactive elements. It lives in
// the client, because the client owns the markup it renders.
const appStyles = `.sf-note { margin: 0 0 1em; }
.sf-note .sf-edit, .sf-note .sf-delete { display: none; margin-left: 0.5em; }
.sf-note:hover .sf-edit, .sf-note:hover .sf-delete { display: inline-block; }
.sf-note .sf-input { font: inherit; }
.sf-body { white-space: pre-wrap; overflow-wrap: break-word; }
.sf-note .sf-save, .sf-note .sf-cancel { margin-left: 0.25em; }
.sf-new-note { margin: 0 0 1em; }
.sf-new-note input, .sf-new-note textarea { font: inherit; padding: 0.25em 0.5em; margin-right: 0.5em; height: 3em; vertical-align: top; }
.sf-new-note button { font: inherit; padding: 0.25em 0.75em; }`
