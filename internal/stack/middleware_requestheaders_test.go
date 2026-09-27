package stack

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// headerEngine builds an engine carrying the requestheaders slot with
// the supplied bag and one GET route.
func headerEngine(t *testing.T, params any) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(IntakeReqHeaders, params)
	engine, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	engine.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return engine
}

// headerHit runs a GET against the engine, adding the supplied
// headers to the request.
func headerHit(t *testing.T, engine *gin.Engine, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// TestHeaderCountCap verifies the line cap. The request.id slot
// stamps one X-Request-ID header onto the request, so the guard sees
// one more line than the test supplies. With a configured cap of
// fifty-four, fifty-five supplied lines (fifty-six seen) trip a 431
// naming the dimension, while fifty (fifty-one seen) pass; the
// default cap of fifty rejects fifty-five supplied (fifty-six seen)
// and admits forty (forty-one seen).
func TestHeaderCountCap(t *testing.T) {
	engine := headerEngine(t, &RequestHeaderParams{MaxHeaders: 54})
	over := make(map[string]string)
	for i := 0; i < 55; i++ {
		over["X-Extra-"+itoa(i)] = "v"
	}
	rec := headerHit(t, engine, over)
	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status = %d; want 431", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "header lines") {
		t.Errorf("body %q does not name the line dimension", rec.Body.String())
	}

	under := make(map[string]string)
	for i := 0; i < 50; i++ {
		under["X-Extra-"+itoa(i)] = "v"
	}
	rec = headerHit(t, engine, under)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}

	defEngine := headerEngine(t, nil)
	overDef := make(map[string]string)
	for i := 0; i < 55; i++ {
		overDef["X-Extra-"+itoa(i)] = "v"
	}
	rec = headerHit(t, defEngine, overDef)
	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("default cap status = %d; want 431", rec.Code)
	}
	underDef := make(map[string]string)
	for i := 0; i < 40; i++ {
		underDef["X-Extra-"+itoa(i)] = "v"
	}
	rec = headerHit(t, defEngine, underDef)
	if rec.Code != http.StatusOK {
		t.Fatalf("default cap status = %d; want 200", rec.Code)
	}
}

// TestHeaderValueCap verifies one oversized value trips the per-value
// cap with a 431 naming the dimension.
func TestHeaderValueCap(t *testing.T) {
	engine := headerEngine(t, nil)
	rec := headerHit(t, engine, map[string]string{"X-Big": strings.Repeat("a", 8<<10+1)})
	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status = %d; want 431", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "header value") {
		t.Errorf("body %q does not name the value dimension", rec.Body.String())
	}
}

// TestHeaderTotalCap verifies the aggregate block trips the total cap
// even when no single value is oversized.
func TestHeaderTotalCap(t *testing.T) {
	engine := headerEngine(t, nil)
	// Forty headers of 1 KiB each: 40 KiB total, under the line cap
	// and the per-value cap, over the 32 KiB total cap.
	headers := make(map[string]string)
	for i := 0; i < 40; i++ {
		headers["X-Pad-"+itoa(i)] = strings.Repeat("p", 1<<10)
	}
	rec := headerHit(t, engine, headers)
	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status = %d; want 431", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "header block") {
		t.Errorf("body %q does not name the total dimension", rec.Body.String())
	}
}

// TestHeaderControlSequence verifies a value carrying a CR/LF sequence
// trips the ban with a 431.
func TestHeaderControlSequence(t *testing.T) {
	engine := headerEngine(t, nil)
	rec := headerHit(t, engine, map[string]string{"X-Smuggled": "value\r\nInjected: 1"})
	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("status = %d; want 431", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "control sequence") {
		t.Errorf("body %q does not name the control-sequence ban", rec.Body.String())
	}
}

// TestHeaderControlBanDisabled verifies an explicit false lifts the
// control-sequence ban while the caps stay armed.
func TestHeaderControlBanDisabled(t *testing.T) {
	disabled := false
	engine := headerEngine(t, &RequestHeaderParams{BanControlChars: &disabled})
	rec := headerHit(t, engine, map[string]string{"X-Smuggled": "value\r\nInjected: 1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 with the ban off", rec.Code)
	}
}

// TestHeaderBypassedProbeSkipsGuard verifies a request wearing the
// intake-bypass stamp sails past the header guard.
func TestHeaderBypassedProbeSkipsGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const maintEnv = "UNIT_REQHEADERS_MAINT"
	t.Setenv(maintEnv, "1")
	b := NewBuilder()
	b.Configure(GateMaintenance, &MaintenanceParams{
		EnvVars:   []string{maintEnv},
		FlagVars:  []string{""},
		LivePaths: []string{"/ping"},
	})
	b.Configure(IntakeReqHeaders, nil)
	engine, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	engine.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	over := make(map[string]string)
	for i := 0; i < 55; i++ {
		over["X-Extra-"+itoa(i)] = "v"
	}
	rec := headerHit(t, engine, over)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 for a bypassed probe", rec.Code)
	}
}

// TestRequestHeadersWrongBagRefused verifies a foreign bag breaks the
// build.
func TestRequestHeadersWrongBagRefused(t *testing.T) {
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(IntakeReqHeaders, &SlogParams{})
	if _, err := b.Assemble(); err == nil {
		t.Fatal("assemble accepted a SlogParams bag for intake.requestheaders; want a refusal")
	}
}
