package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cybtachyon/sheidan/db"
	"github.com/cybtachyon/sheidan/internal/models"
	"github.com/gin-gonic/gin"
)

// newDemoTestEngine builds the demo engine around an isolated
// in-memory database, mirroring the production wiring minus the web
// client bootstrap and the listening socket.
func newDemoTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := db.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
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
	return newDemoEngine(database)
}

func getPPROF(t *testing.T, engine *gin.Engine, path string, headers map[string]string) int {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	engine.ServeHTTP(recorder, request)
	return recorder.Result().StatusCode
}

// TestPprofIdleUnlessFlagged verifies the debug group stays dark by
// default: without SHEIDAN_DEBUG the profiler paths answer 404.
func TestPprofIdleUnlessFlagged(t *testing.T) {
	t.Setenv("SHEIDAN_DEBUG", "")
	engine := newDemoTestEngine(t)
	if got := getPPROF(t, engine, "/debug/pprof/heap", nil); got != http.StatusNotFound {
		t.Fatalf("GET /debug/pprof/heap status = %d; want 404 with the flag unset", got)
	}
}

// TestPprofAnswersWhenFlagged verifies SHEIDAN_DEBUG=1 lights the
// group: the index and a profile both answer 200 with content.
func TestPprofAnswersWhenFlagged(t *testing.T) {
	t.Setenv("SHEIDAN_DEBUG", "1")
	engine := newDemoTestEngine(t)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil))
	result := recorder.Result()
	if result.StatusCode != http.StatusOK {
		t.Fatalf("GET /debug/pprof/heap status = %d; want 200", result.StatusCode)
	}
	if result.ContentLength == 0 {
		t.Fatal("the heap profile body is empty")
	}
	index := getPPROF(t, engine, "/debug/pprof/", nil)
	if index != http.StatusOK {
		t.Fatalf("GET /debug/pprof/ status = %d; want 200", index)
	}
}

// TestPprofTokenGate verifies the bearer token guard: absent or wrong
// credentials refuse with 401, and the exact token admits.
func TestPprofTokenGate(t *testing.T) {
	t.Setenv("SHEIDAN_DEBUG", "1")
	t.Setenv("SHEIDAN_DEBUG_TOKEN", "sekrit")
	engine := newDemoTestEngine(t)

	if got := getPPROF(t, engine, "/debug/pprof/heap", nil); got != http.StatusUnauthorized {
		t.Fatalf("unticketed status = %d; want 401", got)
	}
	if got := getPPROF(t, engine, "/debug/pprof/heap", map[string]string{"Authorization": "Bearer wrong"}); got != http.StatusUnauthorized {
		t.Fatalf("wrong ticket status = %d; want 401", got)
	}
	if got := getPPROF(t, engine, "/debug/pprof/heap", map[string]string{"Authorization": "Bearer sekrit"}); got != http.StatusOK {
		t.Fatalf("bearer status = %d; want 200", got)
	}
}
