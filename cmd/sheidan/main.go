// The sheidan command serves the Sheidan demo app: a templ-rendered
// home page and a notes API backed by GORM.
package main

import (
	"log"
	"os"

	"github.com/cybtachyon/sheidan"
	"github.com/cybtachyon/sheidan/internal/db"
	"github.com/cybtachyon/sheidan/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("SHEIDAN_DB")
	if dsn == "" {
		dsn = "file:./data/sheidan.db"
	}
	database, err := db.Open(dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err := database.AutoMigrate(&models.Note{}); err != nil {
		log.Fatal(err)
	}

	engine := sheidan.New()
	engine.GET("/", sheidan.Wrap(Home()))
	engine.GET("/notes", listNotes(database))
	if err := engine.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

// listNotes returns all notes as JSON.
func listNotes(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var notes []models.Note
		if err := database.Find(&notes).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, notes)
	}
}
