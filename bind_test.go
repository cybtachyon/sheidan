package sheidan_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/cybtachyon/sheidan"
	"github.com/cybtachyon/sheidan/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// bindNote is a minimal GORM model for the Bind test.
type bindNote struct {
	gorm.Model
	Title string
}

// TestBind verifies that Bind loads the model from the database and
// renders the templ component with it, that it responds with status 404
// when the model is missing, and that it responds with status 500 when
// the load fails with another error.
func TestBind(t *testing.T) {
	database, err := db.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("DB error = %v", err)
	}
	// One connection keeps the shared in-memory database alive.
	sqlDB.SetMaxOpenConns(1)
	if err := database.AutoMigrate(&bindNote{}); err != nil {
		t.Fatalf("AutoMigrate error = %v", err)
	}
	note := bindNote{Title: "Hello"}
	if err := database.Create(&note).Error; err != nil {
		t.Fatalf("Create error = %v", err)
	}

	load := func(c *gin.Context) (bindNote, error) {
		// The "boom" id fails the load with a plain error, so the
		// handler responds with status 500.
		if c.Param("id") == "boom" {
			return bindNote{}, errors.New("boom")
		}
		var got bindNote
		if err := database.First(&got, c.Param("id")).Error; err != nil {
			return bindNote{}, err
		}
		return got, nil
	}
	render := func(n bindNote) templ.Component {
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			_, err := fmt.Fprintf(w, "<h1>%s</h1>", n.Title)
			return err
		})
	}

	engine := sheidan.New()
	engine.GET("/note/:id", sheidan.Bind(load, render))

	// Success path: the loaded note is rendered as HTML.
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/note/%d", note.ID), nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /note/%d status = %d; want %d", note.ID, w.Code, http.StatusOK)
	}
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q; want %q", contentType, "text/html; charset=utf-8")
	}
	if !strings.Contains(w.Body.String(), "<h1>Hello</h1>") {
		t.Errorf("body = %q; want it to contain %q", w.Body.String(), "<h1>Hello</h1>")
	}

	// Missing path: a note with no row is GORM's ErrRecordNotFound, so
	// the handler responds with status 404 and a JSON error body.
	request = httptest.NewRequest(http.MethodGet, "/note/999999", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /note/999999 status = %d; want %d", w.Code, http.StatusNotFound)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("404 Content-Type = %q; want it to contain %q", got, "application/json")
	}
	if !strings.Contains(w.Body.String(), "record not found") {
		t.Errorf("404 body = %q; want it to contain %q", w.Body.String(), "record not found")
	}

	// Failure path: a load error other than ErrRecordNotFound is a
	// server error, so the handler responds with status 500 and a JSON
	// error body.
	request = httptest.NewRequest(http.MethodGet, "/note/boom", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, request)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("GET /note/boom status = %d; want %d", w.Code, http.StatusInternalServerError)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Errorf("500 Content-Type = %q; want it to contain %q", got, "application/json")
	}
	if !strings.Contains(w.Body.String(), "boom") {
		t.Errorf("500 body = %q; want it to contain %q", w.Body.String(), "boom")
	}
}
