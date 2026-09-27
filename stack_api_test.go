package sheidan

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestNewInstallsFullChain verifies New returns an engine whose
// handler list is Recovery plus all twenty-one slots, holders
// included, so the chain length and ordering stay stable before any
// slot is filled.
func TestNewInstallsFullChain(t *testing.T) {
	engine := New()
	gin.SetMode(gin.TestMode)
	if got, want := len(engine.Handlers), 22; got != want {
		t.Fatalf("installed handlers = %d; want %d (recovery plus 21 slots)", got, want)
	}
}

// TestNewServesTraffic verifies the rebuilt New still serves ordinary
// routes end to end.
func TestNewServesTraffic(t *testing.T) {
	engine := New()
	engine.GET("/probe", func(c *gin.Context) { c.String(http.StatusTeapot, "teapot") })
	responder := httptest.NewRecorder()
	engine.ServeHTTP(responder, httptest.NewRequest(http.MethodGet, "/probe", nil))
	if responder.Code != http.StatusTeapot {
		t.Fatalf("status = %d; want %d", responder.Code, http.StatusTeapot)
	}
	if got := responder.Body.String(); got != "teapot" {
		t.Errorf("body = %q; want teapot", got)
	}
}

// TestStackComposition verifies the builder composes an adjustable
// engine: disabling a slot shrinks the chain, inserting a custom
// handler grows it, and the custom handler runs.
func TestStackComposition(t *testing.T) {
	called := false
	s := Stack()
	s.Disable(SessionFlashError)
	s.Insert("after:logging.slog", func(c *gin.Context) {
		called = true
		c.Next()
	})
	engine, err := s.Apply()
	if err != nil {
		t.Fatalf("Apply error = %v", err)
	}
	engine.GET("/probe", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	responder := httptest.NewRecorder()
	engine.ServeHTTP(responder, httptest.NewRequest(http.MethodGet, "/probe", nil))
	if !called {
		t.Error("inserted handler did not run")
	}
	if got, want := len(engine.Handlers), 22; got != want {
		t.Fatalf("installed handlers = %d; want %d (one disabled, one added)", got, want)
	}
}

// TestApplyReportsBrokenOperations verifies misnamed operations are
// refused at build time with the offender named, and the engine is
// not handed out.
func TestApplyReportsBrokenOperations(t *testing.T) {
	s := Stack()
	s.Disable("made.up.slot")
	_, err := s.Apply()
	if err == nil {
		t.Fatal("Apply succeeded; want an error naming made.up.slot")
	}
	if !strings.Contains(err.Error(), "made.up.slot") {
		t.Errorf("error %q does not name the bad slot", err)
	}
}

// TestChainExclusionsStayLocal verifies per-group exclusions filter
// one branch's copy only: the excluded group loses the slot, a second
// group from the same base keeps it, and the base engine is
// untouched.
func TestChainExclusionsStayLocal(t *testing.T) {
	base := New()
	admin, err := Chain(base).Without(SafetyCsrf, IntakePostsize).Handlers()
	if err != nil {
		t.Fatalf("admin Handlers error = %v", err)
	}
	public, err := Chain(base).Handlers()
	if err != nil {
		t.Fatalf("public Handlers error = %v", err)
	}
	if len(admin) != 19 {
		t.Errorf("admin handlers = %d; want 19 (two excluded)", len(admin))
	}
	if len(public) != 21 {
		t.Errorf("public handlers = %d; want 21 (none excluded)", len(public))
	}
}

// TestChainRejectsUnknownSlots verifies an unrecognized exclusion is
// surfaced by Handlers instead of silently dropping nothing.
func TestChainRejectsUnknownSlots(t *testing.T) {
	base := New()
	_, err := Chain(base).Without("ghost.branch").Handlers()
	if err == nil {
		t.Fatal("Handlers succeeded; want an error naming ghost.branch")
	}
}

// TestChainRequiresABase verifies a nil engine is refused loudly.
func TestChainRequiresABase(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Chain(nil) did not panic")
		}
	}()
	Chain(nil)
}

// TestChainSilencesNewIntakeSlots verifies the group exclusion list
// silences a new intake slot for one branch while a sibling group from
// the same base keeps it armed: a group that drops requestheaders
// admits a fifty-five-line request, and a sibling group still rejects
// the same flood with a 431.
func TestChainSilencesNewIntakeSlots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := New()
	engine := gin.New()

	probe := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	api := engine.Group("/api")
	apiHandlers, err := Chain(base).Without(IntakeReqHeaders).Handlers()
	if err != nil {
		t.Fatalf("api Handlers error = %v", err)
	}
	api.Use(apiHandlers...)
	api.GET("/probe", probe)

	public := engine.Group("/public")
	publicHandlers, err := Chain(base).Handlers()
	if err != nil {
		t.Fatalf("public Handlers error = %v", err)
	}
	public.Use(publicHandlers...)
	public.GET("/probe", probe)

	flood := func(path string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for i := 0; i < 55; i++ {
			req.Header.Set(fmt.Sprintf("X-Extra-%d", i), "v")
		}
		return req
	}

	// The api group drops the slot, so the flood sails through.
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, flood("/api/probe"))
	if rec.Code != http.StatusOK {
		t.Fatalf("api (silenced) status = %d; want 200", rec.Code)
	}

	// The public group keeps the slot, so the same flood is refused.
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, flood("/public/probe"))
	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("public (armed) status = %d; want 431", rec.Code)
	}
}
