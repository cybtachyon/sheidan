// The sheidan command is the Sheidan demo app and its web toolchain.
// With no argument it serves the demo app: a templ-rendered home
// page, a reactive notes list and note detail page rendered by the
// GopherJS client, and note create, update, and delete endpoints, all
// backed by GORM. The list and detail pages content-negotiate between
// the reactive shell and JSON, so a browser gets the shell and an API
// client gets JSON.
// The web subcommand transpiles the GopherJS web client, and test-web
// runs the web client's tests.
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/cybtachyon/sheidan"
	"github.com/cybtachyon/sheidan/db"
	"github.com/cybtachyon/sheidan/internal/models"
	"github.com/cybtachyon/sheidan/webbuild"
	ginpprof "github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// webClientDir is the GopherJS web client module, relative to the repo
// root.
const webClientDir = "cmd/sheidan/web"

// webOut is the transpiled web client output, relative to the repo root.
const webOut = "web/web.js"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "web":
			if err := webbuild.Build(webClientDir, webOut); err != nil {
				log.Fatal(err)
			}
			return
		case "test-web":
			if err := webbuild.Test(webClientDir); err != nil {
				log.Fatal(err)
			}
			return
		case "run":
			// Fall through to run the app.
		case "help", "-h", "--help":
			usage()
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
			usage()
			os.Exit(2)
		}
	}
	runApp()
}

// usage prints the command's help text.
func usage() {
	fmt.Fprint(os.Stderr, `sheidan - the Sheidan demo app and web toolchain

Usage:
  sheidan            run the demo app
  sheidan web        transpile the GopherJS web client
  sheidan test-web   run the web client's tests
  sheidan help       show this help

Environment:
  SHEIDAN_DB             the GORM data source name (default file:./data/scheidan.db)
  SHEIDAN_DEBUG=1        mount the pprof routes under /debug/pprof
                         (feed them to go tool pprof, e.g.:
                          go tool pprof http://HOST:8080/debug/pprof/heap)
  SHEIDAN_DEBUG_TOKEN    bearer token guarding the pprof group
`)
}

// runApp serves the demo app. It bootstraps the web client first, so
// the app is self-contained: go run ./cmd/sheidan provisions the
// GopherJS toolchain on first use and transpiles the client when it is
// stale, then starts the server.
func runApp() {
	if err := webbuild.Build(webClientDir, webOut); err != nil {
		log.Fatal(err)
	}

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

	engine := newDemoEngine(database)
	if err := engine.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

// newDemoEngine wires the demo's routes onto a fresh engine, plus the
// environment-driven debug facility: when SHEIDAN_DEBUG is 1, the
// pprof routes mount beneath /debug/pprof, optionally gated by a
// bearer token drawn from SHEIDAN_DEBUG_TOKEN. Left unset, the group
// never mounts and its paths answer 404.
func newDemoEngine(database *gorm.DB) *gin.Engine {
	engine := sheidan.New()
	engine.GET("/", sheidan.Wrap(Home()))
	engine.GET("/notes", listNotes(database))
	engine.POST("/notes", createNote(database))
	engine.GET("/note/:id", showNote(database))
	engine.PATCH("/note/:id", updateNote(database))
	engine.DELETE("/note/:id", deleteNote(database))
	// The GopherJS build output of the web client, produced by the
	// bootstrap above.
	engine.StaticFile("/web/web.js", "./web/web.js")
	if os.Getenv("SHEIDAN_DEBUG") == "1" {
		attachPprof(engine)
	}
	return engine
}

// attachPprof mounts the standard pprof route family on a dedicated
// group, so it travels beside the default chain instead of joining it.
// A nonzero SHEIDAN_DEBUG_TOKEN switches the group to strict bearer
// authentication, protecting the profiles from casual probing.
func attachPprof(engine *gin.Engine) {
	group := engine.Group("/debug")
	token := os.Getenv("SHEIDAN_DEBUG_TOKEN")
	if token != "" {
		group.Use(func(c *gin.Context) {
			if c.GetHeader("Authorization") != "Bearer "+token {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Next()
		})
	}
	ginpprof.RouteRegister(group, "pprof")
}

// listNotes returns all notes, content-negotiating the representation.
// A request whose Accept header names text/html gets the reactive list
// shell, and every other request gets the notes as JSON. Browsers send
// text/html, and API clients usually do not, so the default
// representation stays JSON.
func listNotes(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if wantsHTML(c) {
			sheidan.Wrap(NoteListShell())(c)
			return
		}
		var notes []models.Note
		if err := database.Find(&notes).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, notes)
	}
}

// wantsHTML reports whether the request's Accept header names
// text/html.
func wantsHTML(c *gin.Context) bool {
	for _, part := range strings.Split(c.GetHeader("Accept"), ",") {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mediaType == "text/html" {
			return true
		}
	}
	return false
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

// showNote returns a note by its route parameter, content-negotiating
// the representation. A request whose Accept header names text/html
// gets the reactive detail shell, and every other request gets the
// note as JSON. A missing note responds with status 404, a malformed
// ID with status 400, and other errors with status 500, for either
// representation.
func showNote(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var note models.Note
		if err := database.First(&note, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(404, gin.H{"error": err.Error()})
				return
			}
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if wantsHTML(c) {
			sheidan.Wrap(NoteDetailShell())(c)
			return
		}
		c.JSON(200, note)
	}
}

// updateNote updates a note's title and/or body from a JSON body,
// through GORM. Fields absent from the body are left unchanged. A
// missing note responds with status 404.
func updateNote(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var note models.Note
		if err := database.First(&note, id).Error; err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}
		var update struct {
			Title *string
			Body  *string
		}
		if err := c.BindJSON(&update); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if update.Title != nil {
			note.Title = *update.Title
		}
		if update.Body != nil {
			note.Body = *update.Body
		}
		if err := database.Save(&note).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, note)
	}
}

// deleteNote deletes a note by its route parameter, through GORM. A
// successful deletion responds with status 204 and no body. A missing
// note responds with status 404, a malformed ID with status 400, and
// other errors with status 500.
func deleteNote(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		var note models.Note
		if err := database.First(&note, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(404, gin.H{"error": err.Error()})
				return
			}
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if err := database.Delete(&note).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.Status(204)
	}
}
