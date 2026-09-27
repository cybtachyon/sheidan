package stack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// quietMaintenanceEnvironment shields the tests from ambient
// maintenance variables, so the gates react only to what the test
// sets.
func quietMaintenanceEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(defaultMaintenanceEnv, "")
	t.Setenv(defaultMaintenanceFlagVar, "")
}

// maintenanceEngine builds an engine carrying the maintenance slot
// with the supplied bag (nil selects the shipped defaults) and
// registers probe routes: the root reports the intake-bypass stamp,
// and a ping route doubles as a live-path admission witness.
func maintenanceEngine(t *testing.T, params any) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(GateMaintenance, params)
	engine, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	stamped := func(c *gin.Context) {
		ok := c.GetBool(intakeBypass)
		c.JSON(http.StatusOK, gin.H{"stamp": ok})
	}
	engine.GET("/", stamped)
	engine.GET("/ping", stamped)
	return engine
}

// maintenanceHit runs one GET against the engine and hands back the
// response for inspection.
func maintenanceHit(t *testing.T, engine *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// TestMaintenanceDisengagedByDefault verifies the fresh chain admits
// traffic when no trigger fires, the zero-cost resting state.
func TestMaintenanceDisengagedByDefault(t *testing.T) {
	quietMaintenanceEnvironment(t)
	rec := maintenanceHit(t, maintenanceEngine(t, nil), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 while disengaged", rec.Code)
	}
}

// TestMaintenanceEnvFlip verifies the environment trigger engages and
// disengages between successive requests, the mid-life flip the slice
// promises without a restart.
func TestMaintenanceEnvFlip(t *testing.T) {
	quietMaintenanceEnvironment(t)
	const env = "UNIT_MAINT_ENGAGED"
	engine := maintenanceEngine(t, &MaintenanceParams{
		EnvVars:      []string{env},
		FlagVars:     []string{""},
		RetrySeconds: 7,
	})
	t.Setenv(env, "")
	if rec := maintenanceHit(t, engine, "/"); rec.Code != http.StatusOK {
		t.Fatalf("pre-engagement status = %d; want 200", rec.Code)
	}
	t.Setenv(env, "1")
	rec := maintenanceHit(t, engine, "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("engaged status = %d; want 503", rec.Code)
	}
	if got := rec.Result().Header.Get("Retry-After"); got != "7" {
		t.Errorf("Retry-After = %q; want 7", got)
	}
	var body struct {
		Error      string `json:"error"`
		RetrySecs  int    `json:"retry_seconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body = %q: %v", rec.Body.String(), err)
	}
	if body.RetrySecs != 7 {
		t.Errorf("body retry_seconds = %d; want 7", body.RetrySecs)
	}
	t.Setenv(env, "0")
	if rec = maintenanceHit(t, engine, "/"); rec.Code != http.StatusOK {
		t.Fatalf("post-disengage status = %d; want 200", rec.Code)
	}
}

// TestMaintenanceFlagFile verifies the flag-file trigger, resolved
// indirectly through its variable, toggles the gate as the file
// appears and vanishes.
func TestMaintenanceFlagFile(t *testing.T) {
	quietMaintenanceEnvironment(t)
	dir := t.TempDir()
	flag := filepath.Join(dir, "halt.flag")
	const varName = "UNIT_MAINT_FLAG_VAR"
	t.Setenv(varName, flag)
	engine := maintenanceEngine(t, &MaintenanceParams{
		EnvVars:  []string{""},
		FlagVars: []string{varName},
	})
	if rec := maintenanceHit(t, engine, "/"); rec.Code != http.StatusOK {
		t.Fatalf("absent flag status = %d; want 200", rec.Code)
	}
	if err := os.WriteFile(flag, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if rec := maintenanceHit(t, engine, "/"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("present flag status = %d; want 503", rec.Code)
	}
	if err := os.Remove(flag); err != nil {
		t.Fatal(err)
	}
	if rec := maintenanceHit(t, engine, "/"); rec.Code != http.StatusOK {
		t.Fatalf("removed flag status = %d; want 200", rec.Code)
	}
}

// TestMaintenanceLiveAdmission verifies admitted paths sail through
// the engaged gate wearing the intake-bypass stamp, while the rest
// meet the 503 wall.
func TestMaintenanceLiveAdmission(t *testing.T) {
	quietMaintenanceEnvironment(t)
	const env = "UNIT_MAINT_LIVE"
	t.Setenv(env, "1")
	engine := maintenanceEngine(t, &MaintenanceParams{
		EnvVars:   []string{env},
		FlagVars:  []string{""},
		LivePaths: []string{"/ping"},
	})
	root := maintenanceHit(t, engine, "/")
	if root.Code != http.StatusServiceUnavailable {
		t.Fatalf("non-live path status = %d; want 503", root.Code)
	}
	ping := maintenanceHit(t, engine, "/ping")
	if ping.Code != http.StatusOK {
		t.Fatalf("live path status = %d; want 200", ping.Code)
	}
	var witness struct{ Stamp bool }
	if err := json.Unmarshal(ping.Body.Bytes(), &witness); err != nil {
		t.Fatal(err)
	}
	if !witness.Stamp {
		t.Error("admitted request lacks the intake-bypass stamp")
	}
}

// TestMaintenanceWrongBagRefused verifies a foreign parameter bag
// breaks the build at compile time instead of silently shaping the
// wrong slot.
func TestMaintenanceWrongBagRefused(t *testing.T) {
	quietMaintenanceEnvironment(t)
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(GateMaintenance, &SlogParams{})
	if _, err := b.Assemble(); err == nil {
		t.Fatal("assemble accepted a SlogParams bag for gate.maintenance; want a refusal")
	}
}
