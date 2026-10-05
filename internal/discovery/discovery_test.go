package discovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsAgentProbe(t *testing.T) {
	for _, path := range []string{
		"/openapi.json", "/openapi.yaml", "/swagger.json", "/mcp", "/mcp/", "/sse",
		"/llms-full.txt", "/.well-known/ai-plugin.json", "/.well-known/mcp.json",
		"/openapi.json?x=1",
	} {
		if !IsAgentProbe(path) {
			t.Errorf("%s should be an agent probe", path)
		}
	}
	// App pages and the real guide stay with the web app / static files.
	for _, path := range []string{"/", "/simple", "/dashboard", "/keys", "/llms.txt", "/docs/integrations/codex.md", "/mcp-servers"} {
		if IsAgentProbe(path) {
			t.Errorf("%s must not be treated as a probe", path)
		}
	}
}

func TestWithGuide(t *testing.T) {
	got := WithGuide("Invalid token")
	if !strings.HasPrefix(got, "Invalid token") || !strings.Contains(got, GuideURL) {
		t.Fatalf("got %q", got)
	}
	// A trailing period is not doubled.
	if got := WithGuide("Bad thing."); strings.Contains(got, "..") {
		t.Fatalf("doubled period: %q", got)
	}
}

func TestNotFoundAnswersJSON404WithDocs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(NotFound)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type %q", ct)
	}
	var body struct {
		Docs  string `json:"docs"`
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Docs != GuideURL || body.Error.Type != "invalid_request_error" || !strings.Contains(body.Error.Message, "/openapi.json") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestRootForAgents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RootForAgents())
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "<html>app</html>") })
	r.GET("/keys", func(c *gin.Context) { c.String(http.StatusOK, "keys page") })

	get := func(path, accept string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// curl / SDKs / agents: JSON pointing at the guide.
	for _, accept := range []string{"", "*/*", "application/json"} {
		w := get("/", accept)
		if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("accept %q: %d %s", accept, w.Code, w.Header().Get("Content-Type"))
		}
		var body struct {
			Docs string `json:"docs"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Docs != GuideURL {
			t.Fatalf("accept %q: body %s", accept, w.Body.String())
		}
		if w.Header().Get("Vary") != "Accept" {
			t.Fatalf("accept %q: missing Vary: Accept", accept)
		}
	}

	// A browser still gets the web app, and caches know the response varies.
	w := get("/", "text/html,application/xhtml+xml,*/*;q=0.8")
	if w.Body.String() != "<html>app</html>" || w.Header().Get("Vary") != "Accept" {
		t.Fatalf("browser got %q (Vary %q)", w.Body.String(), w.Header().Get("Vary"))
	}

	// Other paths are untouched, whatever the Accept header.
	if w := get("/keys", "*/*"); w.Body.String() != "keys page" {
		t.Fatalf("non-root path intercepted: %q", w.Body.String())
	}
}
