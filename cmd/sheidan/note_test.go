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

	"github.com/cybtachyon/sheidan/db"
	"github.com/cybtachyon/sheidan/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// newNoteServer builds the demo app's note routes around an in-memory
// database with one note, and returns the engine, the database, and
// the note. name names the in-memory database, so each test gets its
// own.
func newNoteServer(t *testing.T, name string) (*gin.Engine, *gorm.DB, models.Note) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := db.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}
	// One connection keeps the shared in-memory database alive.
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("DB error = %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := database.AutoMigrate(&models.Note{}); err != nil {
		t.Fatalf("AutoMigrate error = %v", err)
	}
	note := models.Note{Title: "Hello", Body: "World"}
	if err := database.Create(&note).Error; err != nil {
		t.Fatalf("Create error = %v", err)
	}
	engine := gin.New()
	engine.GET("/notes", listNotes(database))
	engine.POST("/notes", createNote(database))
	engine.PATCH("/note/:id", updateNote(database))
	engine.DELETE("/note/:id", deleteNote(database))
	return engine, database, note
}

// TestUpdateNote verifies that updateNote changes the note's fields
// through GORM, leaves absent fields unchanged, and responds with
// status 404 for a missing note and status 400 for a malformed body.
func TestUpdateNote(t *testing.T) {
	engine, database, note := newNoteServer(t, "update-note")

	// Title only: the body is left unchanged.
	request := httptest.NewRequest(http.MethodPatch, "/note/1", strings.NewReader(`{"title":"Goodbye"}`))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /note/1 status = %d; want %d", w.Code, http.StatusOK)
	}
	var got models.Note
	if err := database.First(&got, note.ID).Error; err != nil {
		t.Fatalf("First error = %v", err)
	}
	if got.Title != "Goodbye" || got.Body != "World" {
		t.Errorf("note = %+v; want Title %q, Body %q", got, "Goodbye", "World")
	}

	// Body only: the title is left unchanged.
	request = httptest.NewRequest(http.MethodPatch, "/note/1", strings.NewReader(`{"body":"Universe"}`))
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /note/1 status = %d; want %d", w.Code, http.StatusOK)
	}
	if err := database.First(&got, note.ID).Error; err != nil {
		t.Fatalf("First error = %v", err)
	}
	if got.Title != "Goodbye" || got.Body != "Universe" {
		t.Errorf("note = %+v; want Title %q, Body %q", got, "Goodbye", "Universe")
	}

	// Missing note: status 404 with a JSON error body.
	request = httptest.NewRequest(http.MethodPatch, "/note/999999", strings.NewReader(`{"title":"Ghost"}`))
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusNotFound {
		t.Fatalf("PATCH /note/999999 status = %d; want %d", w.Code, http.StatusNotFound)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("404 Content-Type = %q; want it to contain %q", got, "application/json")
	}

	// Malformed body: status 400 with a JSON error body.
	request = httptest.NewRequest(http.MethodPatch, "/note/1", strings.NewReader(`not json`))
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("PATCH /note/1 malformed status = %d; want %d", w.Code, http.StatusBadRequest)
	}
}

// TestCreateNote verifies that createNote stores a new note from a JSON
// body, responds with status 201 and the created note, and leaves the
// new note in the list. This is the contract the web client's new-note
// form relies on: it posts the title and body, expects status 201, and
// reloads the page to show the created note. A malformed body responds
// with status 400.
func TestCreateNote(t *testing.T) {
	engine, database, _ := newNoteServer(t, "create-note")

	// Valid body: status 201 with the created note, including the
	// generated ID. The keys match the canonical JSON the client's
	// payload builds and the server's responses use.
	request := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(`{"Title":"Created","Body":"By the form"}`))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /notes status = %d; want %d", w.Code, http.StatusCreated)
	}
	var created models.Note
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	if created.ID == 0 {
		t.Errorf("created.ID = 0; want a generated ID")
	}
	if created.Title != "Created" || created.Body != "By the form" {
		t.Errorf("created = %+v; want Title %q, Body %q", created, "Created", "By the form")
	}

	// The new note is persisted and appears in the list.
	var notes []models.Note
	if err := database.Find(&notes).Error; err != nil {
		t.Fatalf("Find error = %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("len(notes) = %d; want 2", len(notes))
	}

	// Malformed body: status 400 with a JSON error body.
	request = httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(`not json`))
	request.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /notes malformed status = %d; want %d", w.Code, http.StatusBadRequest)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("400 Content-Type = %q; want it to contain %q", got, "application/json")
	}
}

// TestDeleteNote verifies that deleteNote removes the note through
// GORM and responds with status 204 and no body on success, status 404
// for a missing note, and status 400 for a malformed ID. This is the
// contract the web client's delete button relies on: it sends a DELETE
// request, expects status 204, and removes the note's rows on success.
func TestDeleteNote(t *testing.T) {
	engine, database, note := newNoteServer(t, "delete-note")

	// Missing note: status 404 with a JSON error body.
	request := httptest.NewRequest(http.MethodDelete, "/note/999999", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusNotFound {
		t.Fatalf("DELETE /note/999999 status = %d; want %d", w.Code, http.StatusNotFound)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("404 Content-Type = %q; want it to contain %q", got, "application/json")
	}

	// Malformed ID: status 400 with a JSON error body.
	request = httptest.NewRequest(http.MethodDelete, "/note/notanumber", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("DELETE /note/notanumber status = %d; want %d", w.Code, http.StatusBadRequest)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("400 Content-Type = %q; want it to contain %q", got, "application/json")
	}

	// Valid deletion: status 204 with no body, and the note is gone.
	request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/note/%d", note.ID), nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE /note/%d status = %d; want %d", note.ID, w.Code, http.StatusNoContent)
	}
	if w.Body.Len() != 0 {
		t.Errorf("204 body = %q; want it empty", w.Body.String())
	}
	var got models.Note
	err := database.First(&got, note.ID).Error
	if err == nil {
		t.Fatalf("First after delete succeeded; want the note gone")
	}

	// A second delete of the same note reports it missing: status 404.
	request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/note/%d", note.ID), nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusNotFound {
		t.Fatalf("second DELETE /note/%d status = %d; want %d", note.ID, w.Code, http.StatusNotFound)
	}
}

// TestNoteView verifies that the note page marks its title and body
// as editable fields and loads the GopherJS client.
func TestNoteView(t *testing.T) {
	var buf bytes.Buffer
	err := NoteView(models.Note{ID: 7, Title: "Hello", Body: "World"}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Render error = %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`<h1 class="sf-field" data-note-id="7" data-field="title"><span class="sf-value">Hello</span></h1>`,
		`<p class="sf-field" data-note-id="7" data-field="body"><span class="sf-value">World</span></p>`,
		`<script src="/web/web.js"></script>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q; want it to contain %q", body, want)
		}
	}
}
