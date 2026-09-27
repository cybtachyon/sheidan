package stack

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ccors "github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// corsEngine builds an engine carrying the cors slot with the
// supplied bag and a single GET route doubling as the preflight
// target.
func corsEngine(t *testing.T, params any) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(ApiCors, params)
	engine, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	engine.GET("/thing", func(c *gin.Context) { c.String(http.StatusOK, "thing") })
	return engine
}

// corsHit runs a request with the supplied origin against the
// engine's sole route.
func corsHit(t *testing.T, engine *gin.Engine, method, origin string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/thing", nil)
	req.Host = "srv.local"
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// TestCorsClosedByDefault verifies a stock slot admits no cross-origin
// traffic, the restricted default, while same-origin traffic stays
// invisible to the exchange.
func TestCorsClosedByDefault(t *testing.T) {
	rec := corsHit(t, corsEngine(t, nil), http.MethodGet, "http://evil.example")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status = %d; want 403", rec.Code)
	}
	same := corsHit(t, corsEngine(t, nil), http.MethodGet, "http://srv.local")
	if same.Code != http.StatusOK {
		t.Fatalf("same-origin status = %d; want 200", same.Code)
	}
	if got := same.Result().Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("same-origin response advertises %q; want none", got)
	}
}

// TestCorsGrantedExchange verifies a granted origin earns the preflight
// settlement with its allowance echoed back, and a stranger still
// meets the 403 wall, including for a method the route table never
// registered, where the slot must intercept before the 405 fallback.
func TestCorsGrantedExchange(t *testing.T) {
	params := &ccors.Config{
		AllowOrigins: []string{"http://friend.example"},
		MaxAge:       time.Minute,
	}
	engine := corsEngine(t, params)
	pre := corsHit(t, engine, http.MethodOptions, "http://friend.example")
	if pre.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d; want 204", pre.Code)
	}
	if got := pre.Result().Header.Get("Access-Control-Allow-Origin"); got != "http://friend.example" {
		t.Errorf("allow-origin = %q; want the granted origin", got)
	}
	if got := pre.Result().Header.Get("Access-Control-Max-Age"); got == "" {
		t.Error("preflight lacks the max-age hint")
	}
	stranger := corsHit(t, engine, http.MethodOptions, "http://intruder.example")
	if stranger.Code != http.StatusForbidden {
		t.Fatalf("stranger preflight status = %d; want 403", stranger.Code)
	}
}

// TestCorsWildcardCredentialConflict verifies the builder refuses a
// wildcard grant married to credentials at build time, where it
// bites, instead of letting the pairing limp into browsers.
func TestCorsWildcardCredentialConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(ApiCors, &ccors.Config{AllowAllOrigins: true, AllowCredentials: true})
	if _, err := b.Assemble(); err == nil {
		t.Fatal("assemble accepted wildcard origins with credentials; want a refusal")
	}
}

// TestCorsAllOriginsPlusListDemotes verifies a configuration the
// library itself rejects (all-origins alongside an explicit list)
// demotes to the closed posture with the build intact, rather than
// panicking the bootstrap inside the library's validator.
func TestCorsAllOriginsPlusListDemotes(t *testing.T) {
	conflicted := &ccors.Config{
		AllowAllOrigins: true,
		AllowOrigins:    []string{"http://a.example"},
	}
	engine := corsEngine(t, conflicted)
	rec := corsHit(t, engine, http.MethodGet, "http://any.example")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("demoted slot status = %d; want 403 closed", rec.Code)
	}
}

// TestCorsPointerBagAccepted verifies both bag polarities pass the
// acceptance check, so applications need not chase pointers.
func TestCorsPointerBagAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(ApiCors, ccors.Config{AllowOrigins: []string{"http://friend.example"}})
	if _, err := b.Assemble(); err != nil {
		t.Fatalf("assemble refused a value bag: %v", err)
	}
}
