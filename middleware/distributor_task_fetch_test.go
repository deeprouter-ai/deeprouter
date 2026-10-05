package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/i18n"

	"github.com/gin-gonic/gin"
)

// Regression: a by-ID video status fetch names no model, so the token
// whitelist check matched "" and 403'd "该令牌无权访问模型 " for every
// model-limited key — i.e. every Simple-mode video key — on its first poll
// (measured on the local stack 2026-10-05). Videos could be submitted but
// never polled.

// distributeAs runs Distribute for one request made with a key whitelisted to
// the given models, and reports the status the request ended with.
func distributeAs(t *testing.T, method, path, body string, whitelist ...string) int {
	t.Helper()
	// The 403 path renders a translated message; main.go normally loads it.
	if err := i18n.Init(); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		setTokenWhitelist(c, whitelist...)
		c.Next()
	})
	r.Use(Distribute())
	r.Any("/*path", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestDistributeTaskFetch_ModelLimitedKeyMayPollItsVideo(t *testing.T) {
	for _, path := range []string{
		"/v1/videos/task_abc",            // OpenAI-format status
		"/v1/video/generations/task_abc", // unified-format status
	} {
		if got := distributeAs(t, http.MethodGet, path, "", "MiniMax-H3"); got != http.StatusOK {
			t.Errorf("GET %s with a video key = %d, want it to reach the handler", path, got)
		}
	}
}

func TestDistributeTaskFetch_SubmitStillChecksTheWhitelist(t *testing.T) {
	// The fetch exemption must not widen what a key may generate with.
	got := distributeAs(t, http.MethodPost, "/v1/video/generations",
		`{"model":"sora-2","prompt":"a cat"}`, "MiniMax-H3")
	if got != http.StatusForbidden {
		t.Errorf("submitting a model outside the whitelist = %d, want 403", got)
	}
}
