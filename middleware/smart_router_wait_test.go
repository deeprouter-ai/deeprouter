package middleware

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/internal/smart_router_client"
)

// slowSmartRouter answers after delay with a fixed decision, or gives up when the caller does.
func slowSmartRouter(t *testing.T, delay time.Duration) (string, func()) {
	t.Helper()
	return stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"primary":          "claude-sonnet-5",
			"fallback_chain":   []string{"gpt-5.6-terra"},
			"reason":           "jev:code_hard",
			"strategy_version": "heuristic-jev-v2-test",
		})
	})
}

// A decision that takes longer than the old fixed 150ms (smart-router asking
// Jev takes ~400ms) must still be used when SMART_ROUTER_TIMEOUT_MS allows it.
func TestResolveAutoModel_WaitsAsLongAsTheClientTimeout(t *testing.T) {
	url, cleanup := slowSmartRouter(t, 300*time.Millisecond)
	defer cleanup()

	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "fix my regex"}},
	}, 1)
	got := resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, time.Second))

	if got != "claude-sonnet-5" {
		t.Errorf("got %q; a 300ms decision within a 1s timeout must be used", got)
	}
	if reason := w.Header().Get("X-DeepRouter-Routed-Reason"); reason != "jev:code_hard" {
		t.Errorf("reason = %q", reason)
	}
}

// The same knob still caps the wait: past it the gateway falls back and stays up.
func TestResolveAutoModel_GivesUpAtTheClientTimeout(t *testing.T) {
	url, cleanup := slowSmartRouter(t, time.Second)
	defer cleanup()

	c, _ := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "fix my regex"}},
	}, 1)
	start := time.Now()
	got := resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, 50*time.Millisecond))

	if got != DefaultAutoFallbackModel {
		t.Errorf("got %q, want the default model after the timeout", got)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("waited %v; the 50ms timeout should have cut the wait short", elapsed)
	}
}
