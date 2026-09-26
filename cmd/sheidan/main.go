// The sheidan command serves the Sheidan demo app: a templ-rendered
// home page, a notes JSON API, and a note page rendered through the
// Bind data-binding layer, all backed by GORM.
package main

import (
	"log"
	"os"
	"strconv"

	"github.com/cybtachyon/sheidan"
	"github.com/cybtachyon/sheidan/db"
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
	engine.POST("/notes", createNote(database))
	engine.GET("/note/:id", showNote(database))
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

// createNote stores a new note from a JSON body and returns it.
func createNote(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var note models.Note
		if err := c.BindJSON(&note); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := database.Create(&note).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, note)
	}
}

// showNote loads a note by its route parameter and renders it with the
// NoteView templ component, demonstrating the Bind data-binding layer.
func showNote(database *gorm.DB) gin.HandlerFunc {
	return sheidan.Bind(
		func(c *gin.Context) (models.Note, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return models.Note{}, err
			}
			var note models.Note
			if err := database.First(&note, id).Error; err != nil {
				return models.Note{}, err
			}
			return note, nil
		},
		NoteView,
	)
}
