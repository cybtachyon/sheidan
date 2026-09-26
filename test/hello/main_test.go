package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cybtachyon/sheidan"
	"github.com/gin-gonic/gin"
)

// TestHelloServed verifies that the Hello World app built on the
// sheidan framework serves the expected page.
func TestHelloServed(t *testing.T) {
	engine := sheidan.New()
	engine.GET("/", sheidan.Wrap(Hello()))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, request)

	if w.Code != http.StatusOK {
		t.Fatalf("GET / status = %d; want %d", w.Code, http.StatusOK)
	}
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("GET / Content-Type = %q; want %q", contentType, "text/html; charset=utf-8")
	}
	body := w.Body.String()
	if !strings.Contains(body, "<h1>Hello World</h1>") {
		t.Errorf("GET / body = %q; want it to contain %q", body, "<h1>Hello World</h1>")
	}
}

// TestNewIgnoresForwardedFor verifies that New disables trusted
// proxy parsing, so a forged X-Forwarded-For header does not change
// ClientIP.
func TestNewIgnoresForwardedFor(t *testing.T) {
	engine := sheidan.New()
	var got string
	engine.GET("/who", func(c *gin.Context) {
		got = c.ClientIP()
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/who", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, request)

	want := "192.0.2.10"
	if got != want {
		t.Errorf("ClientIP = %q; want %q", got, want)
	}
}
