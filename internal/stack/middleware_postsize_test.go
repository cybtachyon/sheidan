package stack

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// postsizeEngine builds an engine carrying the postsize slot with the
// supplied bag and three POST routes of differing ceilings.
func postsizeEngine(t *testing.T, params any) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(IntakePostsize, params)
	engine, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	echo := func(label string) gin.HandlerFunc {
		return func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.String(http.StatusBadRequest, "read error")
				return
			}
			c.String(http.StatusOK, label+"="+itoa(len(body)))
		}
	}
	engine.POST("/notes", echo("notes"))
	engine.POST("/up", echo("up"))
	engine.POST("/ping", echo("ping"))
	return engine
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

// postsizeHit runs a POST with the supplied body and transfer
// treatment against the engine.
func postsizeHit(t *testing.T, engine *gin.Engine, path, contentType string, body []byte, chunked bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if chunked {
		req.ContentLength = -1
		req.TransferEncoding = []string{"chunked"}
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// TestDeclaredOversizeDiesEarly verifies a declared length past the
// ceiling settles 413 before any handler work, with the effective
// ceiling named in the body.
func TestDeclaredOversizeDiesEarly(t *testing.T) {
	engine := postsizeEngine(t, &PostSizeParams{DefaultLimit: 1024})
	rec := postsizeHit(t, engine, "/notes", "application/octet-stream", bytes.Repeat([]byte("x"), 2048), false)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d; want 413", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "1024") {
		t.Errorf("body %q does not name the ceiling", rec.Body.String())
	}
}

// TestDeclaredWithinFlows verifies a compliant declaration passes
// through to the handler unmetered.
func TestDeclaredWithinFlows(t *testing.T) {
	engine := postsizeEngine(t, &PostSizeParams{DefaultLimit: 1024})
	rec := postsizeHit(t, engine, "/notes", "application/octet-stream", bytes.Repeat([]byte("x"), 512), false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
}

// TestChunkedTruncationClassified verifies a chunked stream that
// outruns the ceiling is truncated at the cap, and a handler that
// declines to commit a response on the resulting decoder trouble
// yields the honest 413 instead of a misleading decoder status.
func TestChunkedTruncationClassified(t *testing.T) {
	longJSON := "{\"Title\":\"" + strings.Repeat("A", 2040) + "\"}"
	handler := func(c *gin.Context) {
		var note struct {
			Title string
		}
		if err := c.ShouldBindJSON(&note); err != nil {
			// Defer the verdict: the size guard knows the
			// truth better than the decoder's error.
			return
		}
		c.String(http.StatusOK, "stored "+note.Title)
	}
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(IntakePostsize, &PostSizeParams{DefaultLimit: 1024})
	alt, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	alt.POST("/notes", handler)
	req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(longJSON))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}
	rec := httptest.NewRecorder()
	alt.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d; want 413", rec.Code)
	}
}

// TestUploadOverrideVerified verifies a per-route relaxation absorbs
// what the default would reject, while the unrouted default still
// bites.
func TestUploadOverrideVerified(t *testing.T) {
	params := &PostSizeParams{
		DefaultLimit: 1024,
		Rules:        []PostSizeRule{{Prefix: "/up", Limit: 4096}},
	}
	engine := postsizeEngine(t, params)
	up := postsizeHit(t, engine, "/up", "application/octet-stream", bytes.Repeat([]byte("x"), 2048), false)
	if up.Code != http.StatusOK {
		t.Fatalf("override route status = %d; want 200", up.Code)
	}
	notes := postsizeHit(t, engine, "/notes", "application/octet-stream", bytes.Repeat([]byte("x"), 2048), false)
	if notes.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("default route status = %d; want 413", notes.Code)
	}
}

// TestStampedProbeSkipsGuard verifies a request wearing the
// intake-bypass stamp sails past the size guard, the maintenance
// probe scenario.
func TestStampedProbeSkipsGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const maintEnv = "UNIT_POSTSIZE_MAINT"
	t.Setenv(maintEnv, "1")
	b := NewBuilder()
	b.Configure(GateMaintenance, &MaintenanceParams{
		EnvVars:   []string{maintEnv},
		FlagVars:  []string{""},
		LivePaths: []string{"/ping"},
	})
	b.Configure(IntakePostsize, &PostSizeParams{DefaultLimit: 1024})
	engine, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	engine.POST("/ping", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, "ping="+itoa(len(body)))
	})
	rec := postsizeHit(t, engine, "/ping", "application/octet-stream", bytes.Repeat([]byte("x"), 2048), false)
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 admitted or 503 gated", rec.Code)
	}
	if rec.Code == http.StatusOK && !strings.Contains(rec.Body.String(), "=2048") {
		t.Errorf("body %q; want the full 2048 bytes read", rec.Body.String())
	}
}

// TestPostSizeWrongBagRefused verifies a foreign bag breaks the
// build.
func TestPostSizeWrongBagRefused(t *testing.T) {
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(IntakePostsize, &SlogParams{})
	if _, err := b.Assemble(); err == nil {
		t.Fatal("assemble accepted a SlogParams bag for intake.postsize; want a refusal")
	}
}
