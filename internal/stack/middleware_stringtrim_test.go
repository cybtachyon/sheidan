package stack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// trimEngine builds an engine carrying the strings.trim slot with the
// supplied bag and one route per input channel.
func trimEngine(t *testing.T, params any) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(IntakeStringTrim, params)
	engine, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	// The JSON route echoes the decoded title and password, so a test
	// can see exactly what the binder received.
	engine.POST("/json", func(c *gin.Context) {
		var in struct {
			Title    string `json:"title"`
			Password string `json:"password"`
			Count    int    `json:"count"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.String(http.StatusBadRequest, "bind error")
			return
		}
		c.JSON(http.StatusOK, gin.H{"title": in.Title, "password": in.Password, "count": in.Count})
	})
	// The query route echoes the trimmed query value.
	engine.GET("/query", func(c *gin.Context) {
		c.String(http.StatusOK, "title="+c.Query("title"))
	})
	// The form route echoes the trimmed form value.
	engine.POST("/form", func(c *gin.Context) {
		c.String(http.StatusOK, "title="+c.PostForm("title"))
	})
	return engine
}

// TestTrimJSONBody verifies a JSON body's string leaves arrive trimmed
// at the binder, an excluded field stays verbatim, and a number is
// preserved exactly.
func TestTrimJSONBody(t *testing.T) {
	engine := trimEngine(t, nil)
	body := `{"title":"  hello  ","password":"  keep  ","count":42}`
	req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	var got struct {
		Title    string `json:"title"`
		Password string `json:"password"`
		Count    int    `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal = %v", err)
	}
	if got.Title != "hello" {
		t.Errorf("title = %q; want trimmed", got.Title)
	}
	if got.Password != "  keep  " {
		t.Errorf("password = %q; want verbatim", got.Password)
	}
	if got.Count != 42 {
		t.Errorf("count = %d; want preserved", got.Count)
	}
}

// TestTrimJSONRawBodyPreserved verifies the raw original body is
// parked in the context for auditors after the swap.
func TestTrimJSONRawBodyPreserved(t *testing.T) {
	body := `{"title":"  hello  "}`
	rawSeen := ""
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(IntakeStringTrim, nil)
	alt, err := b.Assemble()
	if err != nil {
		t.Fatalf("assemble = %v", err)
	}
	alt.POST("/json", func(c *gin.Context) {
		if raw, ok := c.Get(rawBodyKey); ok {
			rawSeen = string(raw.([]byte))
		}
		c.String(http.StatusOK, "ok")
	})
	req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	alt.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if rawSeen != body {
		t.Errorf("raw body = %q; want the original %q", rawSeen, body)
	}
}

// TestTrimQuery verifies a query parameter arrives trimmed at the
// handler. Excluded keys are covered by the transformer's own tests.
func TestTrimQuery(t *testing.T) {
	engine := trimEngine(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/query?title=%20%20hello%20%20", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if rec.Body.String() != "title=hello" {
		t.Errorf("body = %q; want title=hello", rec.Body.String())
	}
}

// TestTrimForm verifies a urlencoded form field arrives trimmed at the
// handler.
func TestTrimForm(t *testing.T) {
	engine := trimEngine(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("title=%20%20hello%20%20"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if rec.Body.String() != "title=hello" {
		t.Errorf("body = %q; want title=hello", rec.Body.String())
	}
}

// TestTrimExcludedConfig verifies a custom exclusion list overrides the
// shipped defaults.
func TestTrimExcludedConfig(t *testing.T) {
	engine := trimEngine(t, &StringTrimParams{Exclude: []string{"title"}})
	body := `{"title":"  hello  "}`
	req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	var got struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal = %v", err)
	}
	if got.Title != "  hello  " {
		t.Errorf("title = %q; want verbatim under the custom exclusion", got.Title)
	}
}

// TestStringTrimWrongBagRefused verifies a foreign bag breaks the
// build.
func TestStringTrimWrongBagRefused(t *testing.T) {
	gin.SetMode(gin.TestMode)
	b := NewBuilder()
	b.Configure(IntakeStringTrim, &SlogParams{})
	if _, err := b.Assemble(); err == nil {
		t.Fatal("assemble accepted a SlogParams bag for intake.strings.trim; want a refusal")
	}
}

// TestTrimNoBodyIsNoop verifies a GET with no body and no query sails
// through untouched.
func TestTrimNoBodyIsNoop(t *testing.T) {
	engine := trimEngine(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/query", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if rec.Body.String() != "title=" {
		t.Errorf("body = %q; want an empty title", rec.Body.String())
	}
}
