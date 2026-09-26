package main

import (
	"testing"

	"github.com/cybtachyon/sheidan/db"
	"gorm.io/gorm"
)

// greeting is a minimal GORM model for the consumer db test. It
// mirrors the model pattern a consumer app would define for itself.
type greeting struct {
	gorm.Model
	Word string
}

// TestOpenDB verifies that a consumer module can open a GORM
// connection through the public sheidan/db package and run a basic
// create and read against it.
func TestOpenDB(t *testing.T) {
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

	if err := database.AutoMigrate(&greeting{}); err != nil {
		t.Fatalf("AutoMigrate error = %v", err)
	}

	in := greeting{Word: "hello"}
	if err := database.Create(&in).Error; err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if in.ID == 0 {
		t.Fatalf("Create left ID = 0; want a generated ID")
	}

	var got greeting
	if err := database.First(&got, in.ID).Error; err != nil {
		t.Fatalf("First error = %v", err)
	}
	if got.Word != "hello" {
		t.Errorf("First = %q; want %q", got.Word, "hello")
	}
}
