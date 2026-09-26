package db_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cybtachyon/sheidan/db"
	"github.com/cybtachyon/sheidan/internal/models"
	"gorm.io/gorm"
)

// TestOpenFile verifies that a file: DSN opens a local SQLite
// database and that GORM can create, read, update, and delete
// records in it.
func TestOpenFile(t *testing.T) {
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "test.db")
	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", dsn, err)
	}

	if err := database.AutoMigrate(&models.Note{}); err != nil {
		t.Fatalf("AutoMigrate error = %v", err)
	}

	note := models.Note{Title: "hello", Body: "world"}
	if err := database.Create(&note).Error; err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if note.ID == 0 {
		t.Fatalf("Create left ID = 0; want a generated ID")
	}

	var got models.Note
	if err := database.First(&got, note.ID).Error; err != nil {
		t.Fatalf("First error = %v", err)
	}
	if got.Title != note.Title || got.Body != note.Body {
		t.Errorf("First = {%q %q}; want {%q %q}", got.Title, got.Body, note.Title, note.Body)
	}

	if err := database.Model(&note).Update("Title", "updated").Error; err != nil {
		t.Fatalf("Update error = %v", err)
	}
	if err := database.First(&got, note.ID).Error; err != nil {
		t.Fatalf("First after Update error = %v", err)
	}
	if got.Title != "updated" {
		t.Errorf("First after Update = %q; want %q", got.Title, "updated")
	}

	if err := database.Delete(&note).Error; err != nil {
		t.Fatalf("Delete error = %v", err)
	}
	if err := database.First(&got, note.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("First after Delete error = %v; want gorm.ErrRecordNotFound", err)
	}

	// The mattn driver creates the file on first use.
	if _, err := os.Stat(filepath.Join(dir, "test.db")); err != nil {
		t.Errorf("database file missing: %v", err)
	}
}

// TestOpenMemory verifies that a shared in-memory file: DSN serves a
// working database while one connection stays open.
func TestOpenMemory(t *testing.T) {
	database, err := db.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}
	// One connection keeps the shared in-memory database alive for
	// the whole test.
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("DB error = %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := database.AutoMigrate(&models.Note{}); err != nil {
		t.Fatalf("AutoMigrate error = %v", err)
	}
	note := models.Note{Title: "memory"}
	if err := database.Create(&note).Error; err != nil {
		t.Fatalf("Create error = %v", err)
	}
	var got models.Note
	if err := database.First(&got, note.ID).Error; err != nil {
		t.Fatalf("First error = %v", err)
	}
	if got.Title != note.Title {
		t.Errorf("First = %q; want %q", got.Title, note.Title)
	}
}

// TestOpenFileWithAuthToken verifies that the connector path used
// for remote auth also opens a local file: database. The token is
// ignored for local connections.
func TestOpenFileWithAuthToken(t *testing.T) {
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "test.db")
	database, err := db.Open(dsn, db.WithAuthToken("token"))
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}
	if err := database.AutoMigrate(&models.Note{}); err != nil {
		t.Fatalf("AutoMigrate error = %v", err)
	}
}

// TestOpenRejectsBadDSN verifies that an unsupported DSN scheme
// fails at Open time, not on the first query.
func TestOpenRejectsBadDSN(t *testing.T) {
	if _, err := db.Open("postgres://localhost/x"); err == nil {
		t.Fatal("Open error = nil; want an error for an unsupported scheme")
	}
}
