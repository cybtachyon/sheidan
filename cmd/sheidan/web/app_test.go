package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gopherjs/gopherjs/js"
)

// testServer emulates the demo app's note endpoints for the client's
// tests. It keeps the notes in a map and returns the same JSON shapes
// the real server does, so the client's request and decode paths run
// against realistic data without a network.
type testServer struct {
	notes  map[int]Note
	nextID int
}

// newTestServer creates a server seeded with the given notes.
func newTestServer(notes ...Note) *testServer {
	s := &testServer{notes: map[int]Note{}}
	for _, n := range notes {
		s.notes[n.ID] = n
		if n.ID >= s.nextID {
			s.nextID = n.ID + 1
		}
	}
	return s
}

// send handles a request from the client, like the demo app would.
func (s *testServer) send(method, path string, body []byte) (*http.Response, error) {
	switch {
	case method == http.MethodGet && path == "/notes":
		return s.json(http.StatusOK, s.all()), nil
	case method == http.MethodPost && path == "/notes":
		var payload struct {
			Title string
			Body  string
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return s.json(http.StatusBadRequest, map[string]string{"error": err.Error()}), nil
		}
		s.nextID++
		note := Note{ID: s.nextID, Title: payload.Title, Body: payload.Body}
		s.notes[note.ID] = note
		return s.json(http.StatusCreated, note), nil
	case method == http.MethodGet && strings.HasPrefix(path, "/note/"):
		id, _ := strconv.Atoi(strings.TrimPrefix(path, "/note/"))
		note, ok := s.notes[id]
		if !ok {
			return s.json(http.StatusNotFound, map[string]string{"error": "not found"}), nil
		}
		return s.json(http.StatusOK, note), nil
	case method == http.MethodPatch && strings.HasPrefix(path, "/note/"):
		id, _ := strconv.Atoi(strings.TrimPrefix(path, "/note/"))
		note, ok := s.notes[id]
		if !ok {
			return s.json(http.StatusNotFound, map[string]string{"error": "not found"}), nil
		}
		var payload struct {
			Title *string
			Body  *string
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return s.json(http.StatusBadRequest, map[string]string{"error": err.Error()}), nil
		}
		if payload.Title != nil {
			note.Title = *payload.Title
		}
		if payload.Body != nil {
			note.Body = *payload.Body
		}
		s.notes[id] = note
		return s.json(http.StatusOK, note), nil
	case method == http.MethodDelete && strings.HasPrefix(path, "/note/"):
		id, _ := strconv.Atoi(strings.TrimPrefix(path, "/note/"))
		if _, ok := s.notes[id]; !ok {
			return s.json(http.StatusNotFound, map[string]string{"error": "not found"}), nil
		}
		delete(s.notes, id)
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}
	return s.json(http.StatusNotFound, map[string]string{"error": "not found"}), nil
}

// all returns the server's notes sorted by ID, like the list endpoint
// orders them.
func (s *testServer) all() []Note {
	ids := make([]int, 0, len(s.notes))
	for id := range s.notes {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	notes := make([]Note, 0, len(ids))
	for _, id := range ids {
		notes = append(notes, s.notes[id])
	}
	return notes
}

// json builds a JSON response with the given status code.
func (s *testServer) json(status int, v any) *http.Response {
	body, _ := json.Marshal(v)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{},
	}
}

// newTestApp builds an app bound to the fake DOM and the given server
// stub, with the browser's location stubbed to path. It returns the
// app, the page's container, and a flush function that runs the
// deferred re-renders the app scheduled.
func newTestApp(t *testing.T, path string, server *testServer) (*App, *js.Object, func()) {
	t.Helper()
	loc := js.Global.Get("Object").New()
	loc.Set("pathname", path)
	loc.Set("origin", "http://localhost")
	js.Global.Set("location", loc)
	doc := fakeDocument()
	container := fakeElement("div")
	app := NewApp(doc, container)
	app.send = server.send
	sched, flush := deferredScheduler()
	app.sched = sched
	return app, container, flush
}

// TestAppListPage verifies that the list page renders the heading, the
// form, and one row per loaded note, and that a note's title links to
// its detail page.
func TestAppListPage(t *testing.T) {
	server := newTestServer(Note{ID: 1, Title: "a", Body: "A"}, Note{ID: 2, Title: "b", Body: "B"})
	app, container, flush := newTestApp(t, "/notes", server)
	app.run()
	flush()

	if got := fakeChildTags(container); len(got) != 3 || got[0] != "h1" || got[1] != "form" || got[2] != "div" {
		t.Fatalf("children = %v; want [h1 form div]", got)
	}
	row1 := container.Get("children").Index(2).Get("children").Index(0)
	link := row1.Get("children").Index(0).Get("children").Index(0)
	if got := link.Get("attrs").Get("href").String(); got != "/note/1" {
		t.Errorf("link href = %q; want /note/1", got)
	}
	if got := link.Get("children").Index(0).Get("nodeValue").String(); got != "a" {
		t.Errorf("link text = %q; want a", got)
	}
}

// TestAppAddNote verifies that submitting the new-note form creates a
// note, adds it to the list, and clears the form.
func TestAppAddNote(t *testing.T) {
	server := newTestServer()
	app, container, flush := newTestApp(t, "/notes", server)
	app.run()
	flush()

	form := container.Get("children").Index(1)
	title := form.Call("querySelector", ".sf-new-title")
	body := form.Call("querySelector", ".sf-new-body")
	title.Set("value", "New")
	body.Set("value", "Note")
	fakeEmit(form, "submit", fakeEvent(form))
	flush()

	if got := fakeChildTags(container); len(got) != 3 {
		t.Fatalf("children = %v; want [h1 form div]", got)
	}
	row := container.Get("children").Index(2).Get("children").Index(0)
	link := row.Get("children").Index(0).Get("children").Index(0)
	if got := link.Get("children").Index(0).Get("nodeValue").String(); got != "New" {
		t.Errorf("new note title = %q; want New", got)
	}
	if got := title.Get("value").String(); got != "" {
		t.Errorf("title input after add = %q; want empty", got)
	}
	if got := body.Get("value").String(); got != "" {
		t.Errorf("body input after add = %q; want empty", got)
	}
}

// TestAppEditField verifies the list page's edit flow: clicking edit
// switches the field to an input prefilled with the current value,
// saving updates the server and the store, and the field returns to
// display mode.
func TestAppEditField(t *testing.T) {
	server := newTestServer(Note{ID: 1, Title: "a", Body: "A"})
	app, container, flush := newTestApp(t, "/notes", server)
	app.run()
	flush()

	row := container.Get("children").Index(2).Get("children").Index(0)
	h2 := row.Get("children").Index(0)
	fakeClick(h2.Get("children").Index(1))
	flush()

	h2 = row.Get("children").Index(0)
	if got := fakeChildTags(h2); len(got) != 3 || got[0] != "input" || got[1] != "button" || got[2] != "button" {
		t.Fatalf("edit mode children = %v; want [input button button]", got)
	}
	input := h2.Get("children").Index(0)
	if got := input.Get("value").String(); got != "a" {
		t.Errorf("input value = %q; want a (prefilled)", got)
	}

	input.Set("value", "edited")
	fakeClick(h2.Get("children").Index(1))
	flush()

	h2 = row.Get("children").Index(0)
	if got := fakeChildTags(h2); len(got) != 3 || got[0] != "a" {
		t.Fatalf("display mode children = %v; want [a button button]", got)
	}
	if got := h2.Get("children").Index(0).Get("children").Index(0).Get("nodeValue").String(); got != "edited" {
		t.Errorf("title = %q; want edited", got)
	}
	if got := server.notes[1].Title; got != "edited" {
		t.Errorf("server title = %q; want edited", got)
	}
}

// TestAppEditFieldCancel verifies that cancelling an edit leaves the
// value unchanged on the server.
func TestAppEditFieldCancel(t *testing.T) {
	server := newTestServer(Note{ID: 1, Title: "a", Body: "A"})
	app, container, flush := newTestApp(t, "/notes", server)
	app.run()
	flush()

	row := container.Get("children").Index(2).Get("children").Index(0)
	h2 := row.Get("children").Index(0)
	fakeClick(h2.Get("children").Index(1))
	flush()

	h2 = row.Get("children").Index(0)
	h2.Get("children").Index(0).Set("value", "changed")
	fakeClick(h2.Get("children").Index(2))
	flush()

	if got := server.notes[1].Title; got != "a" {
		t.Errorf("server title = %q; want a (unchanged)", got)
	}
	if got := h2.Get("children").Index(0).Get("children").Index(0).Get("nodeValue").String(); got != "a" {
		t.Errorf("title = %q; want a (unchanged)", got)
	}
}

// TestAppDeleteNote verifies that deleting a note from the list
// removes it from the server and the list.
func TestAppDeleteNote(t *testing.T) {
	server := newTestServer(Note{ID: 1, Title: "a", Body: "A"}, Note{ID: 2, Title: "b", Body: "B"})
	app, container, flush := newTestApp(t, "/notes", server)
	app.run()
	flush()

	row1 := container.Get("children").Index(2).Get("children").Index(0)
	fakeClick(row1.Get("children").Index(0).Get("children").Index(2))
	flush()

	if got := fakeChildTags(container); len(got) != 3 || got[2] != "div" {
		t.Fatalf("children = %v; want [h1 form div]", got)
	}
	row := container.Get("children").Index(2).Get("children").Index(0)
	link := row.Get("children").Index(0).Get("children").Index(0)
	if got := link.Get("children").Index(0).Get("nodeValue").String(); got != "b" {
		t.Errorf("remaining title = %q; want b", got)
	}
	if _, ok := server.notes[1]; ok {
		t.Error("note 1 was not deleted from the server")
	}
}

// TestAppDetailPage verifies that the detail page renders a back link,
// the note's title and body, and a delete button.
func TestAppDetailPage(t *testing.T) {
	server := newTestServer(Note{ID: 1, Title: "a", Body: "A"})
	app, container, flush := newTestApp(t, "/note/1", server)
	app.run()
	flush()

	if got := fakeChildTags(container); len(got) != 2 || got[0] != "p" || got[1] != "div" {
		t.Fatalf("children = %v; want [p div]", got)
	}
	back := container.Get("children").Index(0).Get("children").Index(0)
	if got := back.Get("attrs").Get("href").String(); got != "/notes" {
		t.Errorf("back href = %q; want /notes", got)
	}
	note := container.Get("children").Index(1)
	if got := note.Get("children").Index(0).Get("children").Index(0).Get("nodeValue").String(); got != "a" {
		t.Errorf("title = %q; want a", got)
	}
	if got := note.Get("children").Index(1).Get("children").Index(0).Get("nodeValue").String(); got != "A" {
		t.Errorf("body = %q; want A", got)
	}
}

// TestAppDetailEdit verifies the detail page's edit flow: editing the
// title and saving updates the server and the rendered title.
func TestAppDetailEdit(t *testing.T) {
	server := newTestServer(Note{ID: 1, Title: "a", Body: "A"})
	app, container, flush := newTestApp(t, "/note/1", server)
	app.run()
	flush()

	note := container.Get("children").Index(1)
	h1 := note.Get("children").Index(0)
	fakeClick(h1.Get("children").Index(1))
	flush()

	h1 = note.Get("children").Index(0)
	h1.Get("children").Index(0).Set("value", "edited")
	fakeClick(h1.Get("children").Index(1))
	flush()

	h1 = note.Get("children").Index(0)
	if got := h1.Get("children").Index(0).Get("nodeValue").String(); got != "edited" {
		t.Errorf("title = %q; want edited", got)
	}
	if got := server.notes[1].Title; got != "edited" {
		t.Errorf("server title = %q; want edited", got)
	}
}

// TestAppDetailDelete verifies that deleting the note from the detail
// page removes it from the server and navigates back to the list.
func TestAppDetailDelete(t *testing.T) {
	server := newTestServer(Note{ID: 1, Title: "a", Body: "A"})
	app, container, flush := newTestApp(t, "/note/1", server)
	app.run()
	flush()

	note := container.Get("children").Index(1)
	fakeClick(note.Get("children").Index(2))
	flush()

	if _, ok := server.notes[1]; ok {
		t.Error("note 1 was not deleted from the server")
	}
	if got := js.Global.Get("location").Get("href").String(); got != "/notes" {
		t.Errorf("location.href = %q; want /notes", got)
	}
}

// TestAppDetailLoadError verifies that a failed detail load shows the
// not-found page.
func TestAppDetailLoadError(t *testing.T) {
	server := newTestServer()
	app, container, flush := newTestApp(t, "/note/99", server)
	app.run()
	flush()

	if got := fakeChildTags(container); len(got) != 2 || got[0] != "p" || got[1] != "h1" {
		t.Fatalf("children = %v; want [p h1]", got)
	}
	if got := container.Get("children").Index(1).Get("children").Index(0).Get("nodeValue").String(); got != "Note not found" {
		t.Errorf("heading = %q; want Note not found", got)
	}
}
