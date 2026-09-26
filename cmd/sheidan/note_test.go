package main

import (
	"bytes"
	"context"
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
// the note.
func newNoteServer(t *testing.T) (*gin.Engine, *gorm.DB, models.Note) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := db.Open("file::memory:?cache=shared")
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
	engine.PATCH("/note/:id", updateNote(database))
	return engine, database, note
}

// TestUpdateNote verifies that updateNote changes the note's fields
// through GORM, leaves absent fields unchanged, and responds with
// status 404 for a missing note and status 400 for a malformed body.
func TestUpdateNote(t *testing.T) {
	engine, database, note := newNoteServer(t)

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
