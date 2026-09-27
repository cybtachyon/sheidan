package stack

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"strings"
	"testing"
	"time"
)

// quietBadTargetEnvironment shields the tests from ambient blocklist
// variables.
func quietBadTargetEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(defaultBadTargetEnv, "")
}

// badTargetEngine builds an engine carrying the bad-target slot with
// the supplied bag and a single probe route.
func badTargetEngine(t *testing.T, params any) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(GateBadTarget, params)
	engine, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	engine.GET("/probe", func(c *gin.Context) { c.String(http.StatusOK, "alive") })
	return engine
}

// badTargetHit runs a GET from the supplied peer address.
func badTargetHit(t *testing.T, engine *gin.Engine, peer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	formatted := peer
	if strings.Contains(peer, ":") {
		formatted = "[" + peer + "]"
	}
	req.RemoteAddr = formatted + ":54321"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// TestBadTargetEnvBlocking verifies the environment roster kills its
// listed peers and spares everyone else, including the IPv4-mapped
// spelling of a listed quad.
func TestBadTargetEnvBlocking(t *testing.T) {
	quietBadTargetEnvironment(t)
	const env = "UNIT_DENY_ROSTER"
	t.Setenv(env, "203.0.113.7, 10.9.0.0/16")
	engine := badTargetEngine(t, &BadTargetParams{EnvVar: env})
	if rec := badTargetHit(t, engine, "203.0.113.7"); rec.Code != http.StatusForbidden {
		t.Fatalf("listed peer status = %d; want 403", rec.Code)
	}
	if rec := badTargetHit(t, engine, "::ffff:0a09:0102"); rec.Code != http.StatusForbidden {
		t.Fatalf("mapped peer status = %d; want 403", rec.Code)
	}
	if rec := badTargetHit(t, engine, "192.0.2.1"); rec.Code != http.StatusOK {
		t.Fatalf("innocent peer status = %d; want 200", rec.Code)
	}
}

// TestBadTargetAltDenialStatus verifies the optional denial status
// swap: a 404 hides the refusal from scanners instead of advertising
// it.
func TestBadTargetAltDenialStatus(t *testing.T) {
	quietBadTargetEnvironment(t)
	const env = "UNIT_DENY_ALT"
	t.Setenv(env, "203.0.113.7")
	engine := badTargetEngine(t, &BadTargetParams{EnvVar: env, DenyStatus: http.StatusNotFound})
	if rec := badTargetHit(t, engine, "203.0.113.7"); rec.Code != http.StatusNotFound {
		t.Fatalf("denial status = %d; want 404", rec.Code)
	}
}

// TestBadTargetFileRefresh verifies a file roster sampled on a short
// period picks up new entries as the file's modification time
// advances, and clears when the file withdraws.
func TestBadTargetFileRefresh(t *testing.T) {
	quietBadTargetEnvironment(t)
	dir := t.TempDir()
	roster := filepath.Join(dir, "roster")
	engine := badTargetEngine(t, &BadTargetParams{
		File:         roster,
		RefreshEvery: time.Millisecond,
	})
	if rec := badTargetHit(t, engine, "203.0.113.7"); rec.Code != http.StatusOK {
		t.Fatalf("absent roster status = %d; want 200", rec.Code)
	}
	when := time.Now().Add(time.Second)
	if err := os.WriteFile(roster, []byte("203.0.113.7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(roster, when, when); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if rec := badTargetHit(t, engine, "203.0.113.7"); rec.Code != http.StatusForbidden {
		t.Fatalf("published roster status = %d; want 403", rec.Code)
	}
	if err := os.RemoveAll(roster); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if rec := badTargetHit(t, engine, "203.0.113.7"); rec.Code != http.StatusOK {
		t.Fatalf("withdrawn roster status = %d; want 200", rec.Code)
	}
}

// TestBadTargetMalformedRosterKeepsTraffic verifies a broken roster
// sinks to an empty gate at boot rather than locking out every
// visitor, with the error announced to the log.
func TestBadTargetMalformedRosterKeepsTraffic(t *testing.T) {
	quietBadTargetEnvironment(t)
	const env = "UNIT_DENY_GARBAGE"
	t.Setenv(env, "banana/24")
	engine := badTargetEngine(t, &BadTargetParams{EnvVar: env})
	if rec := badTargetHit(t, engine, "203.0.113.7"); rec.Code != http.StatusOK {
		t.Fatalf("malformed roster status = %d; want 200", rec.Code)
	}
}

// TestBadTargetWrongBagRefused verifies a foreign bag breaks the
// build at compile time.
func TestBadTargetWrongBagRefused(t *testing.T) {
	quietBadTargetEnvironment(t)
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(GateBadTarget, &MaintenanceParams{})
	if _, err := b.Assemble(); err == nil {
		t.Fatal("assemble accepted a MaintenanceParams bag for gate.badiptarget; want a refusal")
	}
}
