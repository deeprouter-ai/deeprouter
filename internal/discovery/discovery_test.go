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
