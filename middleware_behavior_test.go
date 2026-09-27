package sheidan

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequestIDStampedAndPropagated verifies the request.id slot
// stamps a generated identifier on anonymous requests and reflects a
// caller-supplied one unchanged.
func TestRequestIDStampedAndPropagated(t *testing.T) {
	engine := New()
	engine.GET("/probe", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/probe", nil))
	generated := response.Result().Header.Get("X-Request-ID")
	if generated == "" {
		t.Fatal("anonymous request got no X-Request-ID response header")
	}

	response = httptest.NewRecorder()
	incoming := httptest.NewRequest(http.MethodGet, "/probe", nil)
	incoming.Header.Set("X-Request-ID", "caller-supplied-42")
	engine.ServeHTTP(response, incoming)
	if got := response.Result().Header.Get("X-Request-ID"); got != "caller-supplied-42" {
		t.Fatalf("reflected ID = %q; want the caller-supplied value unchanged", got)
	}
}

// TestSlogLevelLadder verifies the logging.slog slot grades records by
// outcome: info below 400, warn for client errors, error for server
// errors, with the request identifier stitched into each record.
func TestSlogLevelLadder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var sink bytes.Buffer
	stack := Stack()
	stack.Configure(LoggingSlog, &SlogParams{Writer: &sink})
	engine, err := stack.Apply()
	if err != nil {
		t.Fatalf("Apply error = %v", err)
	}
	engine.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "fine") })
	engine.GET("/absent", func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "gone"}) })
	engine.GET("/boom", func(c *gin.Context) { panic("deliberate") })

	fire := func(id string, path string) string {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-Request-ID", id)
		engine.ServeHTTP(response, request)
		return response.Result().Header.Get("X-Request-ID")
	}

	reflectedOK := fire("alpha-id", "/ok")
	reflectedAbsent := fire("beta-id", "/absent")
	reflectedBoom := fire("gamma-id", "/boom")
	text := sink.String()

	check := func(recordFragment, idWant, level string) {
		line := ""
		for _, candidate := range strings.Split(text, "\n") {
			if strings.Contains(candidate, recordFragment) {
				line = candidate
			}
		}
		if line == "" {
			t.Fatalf("no record fragment %q in sink:\n%s", recordFragment, text)
		}
		if !strings.Contains(line, level) {
			t.Errorf("record for %s lacks %s:\n%s", recordFragment, level, line)
		}
		if !strings.Contains(line, "request_id="+idWant) {
			t.Errorf("record for %s lacks request_id=%s:\n%s", recordFragment, idWant, line)
		}
	}
	check("request_id=alpha-id", reflectedOK, "level=INFO")
	check("request_id=beta-id", reflectedAbsent, "level=WARN")
	check("request_id=gamma-id", reflectedBoom, "level=ERROR")
}

// TestSlogSilencesNoise verifies the default skip discipline:
// preflight exchanges and statically served asset families produce no
// records.
func TestSlogSilencesNoise(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var sink bytes.Buffer
	stack := Stack()
	stack.Configure(LoggingSlog, &SlogParams{Writer: &sink})
	engine, err := stack.Apply()
	if err != nil {
		t.Fatalf("Apply error = %v", err)
	}
	engine.OPTIONS("/anything", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	engine.GET("/web/web.js", func(c *gin.Context) { c.String(http.StatusOK, "stub") })
	engine.GET("/logged", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodOptions, "/anything", nil),
		httptest.NewRequest(http.MethodGet, "/web/web.js", nil),
	} {
		engine.ServeHTTP(httptest.NewRecorder(), request)
	}
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/logged", nil))

	lines := strings.FieldsFunc(sink.String(), func(r rune) bool { return r == '\n' })
	for _, line := range lines {
		if strings.Contains(line, "/web/") || strings.Contains(line, "OPTIONS") {
			t.Errorf("silent families leaked a record:\n%s", line)
		}
	}
	if !strings.Contains(sink.String(), "/logged") {
		t.Error("the non-skipped route produced no record")
	}
}
