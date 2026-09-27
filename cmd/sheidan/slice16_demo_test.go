package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cybtachyon/sheidan"
	"github.com/gin-gonic/gin"
)

// newProbeEngine builds an engine carrying the full default middleware
// chain (via sheidan.New) with a few probe routes, so the slice 16
// demonstrations reproduce without a database.
func newProbeEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := sheidan.New()
	engine.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "alive")
	})
	engine.GET("/probe", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	// The note route echoes the title it decoded, so the trim
	// demonstration is visible in the response.
	engine.POST("/notes", func(c *gin.Context) {
		var in struct {
			Title string `json:"title"`
		}
		c.ShouldBindJSON(&in)
		c.JSON(http.StatusCreated, gin.H{"title": in.Title})
	})
	return engine
}

// TestDemoMaintenance reproduces the maintenance demonstration: with
// SHEIDAN_MAINTENANCE set, every route answers 503 with a Retry-After
// hint while /healthz stays alive, and clearing the variable restores
// traffic without a restart.
func TestDemoMaintenance(t *testing.T) {
	engine := newProbeEngine(t)
	t.Setenv("SHEIDAN_MAINTENANCE", "1")

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("maintained /probe status = %d; want 503", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("503 response lacks a Retry-After hint")
	}

	// The health probe stays live during maintenance.
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("maintained /healthz status = %d; want 200", rec.Code)
	}

	// Clearing the variable restores traffic without a restart.
	t.Setenv("SHEIDAN_MAINTENANCE", "")
	req = httptest.NewRequest(http.MethodGet, "/probe", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("restored /probe status = %d; want 200", rec.Code)
	}
}

// TestDemoPostSize reproduces the postsize demonstration: a 2 MiB POST
// to /notes answers 413 under the default one megabyte cap.
func TestDemoPostSize(t *testing.T) {
	engine := newProbeEngine(t)
	body := strings.Repeat("x", 2<<20)
	req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d; want 413", rec.Code)
	}
}

// TestDemoHeaderCap reproduces the header demonstration: a request with
// fifty-five extra lines (fifty-six seen, counting the request.id
// stamp) answers 431, while a modest request passes.
func TestDemoHeaderCap(t *testing.T) {
	engine := newProbeEngine(t)
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	for i := 0; i < 55; i++ {
		req.Header.Set(fmt.Sprintf("X-Extra-%d", i), "v")
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status = %d; want 431", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/probe", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("modest request status = %d; want 200", rec.Code)
	}
}

// TestDemoTrim reproduces the trim demonstration: a title arriving as
// "  hello  " is stored trimmed, while a password-shaped field keeps
// its padding verbatim.
func TestDemoTrim(t *testing.T) {
	engine := newProbeEngine(t)
	req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(`{"title":"  hello  ","password":"  keep  "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want 201", rec.Code)
	}
	var got struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal = %v", err)
	}
	if got.Title != "hello" {
		t.Errorf("title = %q; want trimmed to %q", got.Title, "hello")
	}
}

// TestDemoCORS reproduces the CORS demonstration: a cross-origin
// preflight from a granted origin answers 204 with the granted headers,
// while a same-origin request needs no CORS exchange.
func TestDemoCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	b := sheidan.Stack()
	b.Configure(sheidan.ApiCors, sheidan.CorsOptions{
		AllowOrigins: []string{"https://foreign.example"},
		AllowMethods: []string{http.MethodGet, http.MethodPost},
		AllowHeaders: []string{"Content-Type"},
	})
	engine, err := b.Apply()
	if err != nil {
		t.Fatalf("apply = %v", err)
	}
	engine.GET("/probe", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// A cross-origin preflight from the granted origin answers 204
	// with the granted headers.
	req := httptest.NewRequest(http.MethodOptions, "/probe", nil)
	req.Header.Set("Origin", "https://foreign.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d; want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://foreign.example" {
		t.Errorf("Allow-Origin = %q; want the granted origin", got)
	}

	// A same-origin request (no Origin header) needs no CORS exchange.
	req = httptest.NewRequest(http.MethodGet, "/probe", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-origin status = %d; want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("same-origin Allow-Origin = %q; want none", got)
	}
}
