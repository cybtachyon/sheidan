package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cybtachyon/sheidan/internal/models"
)

// TestNotesView verifies that the list page links each note's title to
// its detail page, marks each note's title and body as editable fields,
// includes the shared field styles, and loads the GopherJS client.
func TestNotesView(t *testing.T) {
	var buf bytes.Buffer
	notes := []models.Note{
		{ID: 1, Title: "First", Body: "One"},
		{ID: 2, Title: "Second", Body: "Two"},
	}
	err := NotesView(notes).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Render error = %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`<h1>Notes</h1>`,
		`<h2 class="sf-field" data-note-id="1" data-field="title"><a class="sf-view" href="/note/1"><span class="sf-value">First</span></a></h2>`,
		`<p class="sf-field" data-note-id="1" data-field="body"><span class="sf-value">One</span></p>`,
		`<h2 class="sf-field" data-note-id="2" data-field="title"><a class="sf-view" href="/note/2"><span class="sf-value">Second</span></a></h2>`,
		`<p class="sf-field" data-note-id="2" data-field="body"><span class="sf-value">Two</span></p>`,
		`.sf-field .sf-edit { display: none; margin-left: 0.5em; }`,
		`.sf-view { color: inherit; text-decoration: none; }`,
		`<script src="/web/web.js"></script>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q; want it to contain %q", body, want)
		}
	}
}

// TestListNotes verifies that /notes content-negotiates: a request
// that names text/html in its Accept header gets the NotesView list
// page, and every other request gets the notes as JSON.
func TestListNotes(t *testing.T) {
	engine, _, note := newNoteServer(t, "list-notes")

	// HTML: a browser's Accept header.
	request := httptest.NewRequest(http.MethodGet, "/notes", nil)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /notes (html) status = %d; want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Errorf("html Content-Type = %q; want it to contain %q", got, "text/html")
	}
	for _, want := range []string{
		fmt.Sprintf(`<h2 class="sf-field" data-note-id="%d" data-field="title"><a class="sf-view" href="/note/%d">`, note.ID, note.ID),
		`<script src="/web/web.js"></script>`,
	} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("html body = %q; want it to contain %q", w.Body.String(), want)
		}
	}

	// JSON: no Accept header, the default for API clients.
	request = httptest.NewRequest(http.MethodGet, "/notes", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /notes (json) status = %d; want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("json Content-Type = %q; want it to contain %q", got, "application/json")
	}
	var got []models.Note
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	if len(got) != 1 || got[0].ID != note.ID || got[0].Title != note.Title || got[0].Body != note.Body {
		t.Errorf("notes = %+v; want one note with ID %d, Title %q, Body %q", got, note.ID, note.Title, note.Body)
	}

	// JSON: an explicit Accept: application/json.
	request = httptest.NewRequest(http.MethodGet, "/notes", nil)
	request.Header.Set("Accept", "application/json")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /notes (json explicit) status = %d; want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("json Content-Type = %q; want it to contain %q", got, "application/json")
	}
}
