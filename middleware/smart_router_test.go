package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/internal/smart_router_client"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// stubServer builds an httptest server returning the given handler for
// POST /route. Returns the URL and a cleanup func.
func stubServer(t *testing.T, handler http.HandlerFunc) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/route" {
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}))
	return srv.URL, srv.Close
}

// newCtxForResolve builds a gin.Context wired up so resolveAutoModel can:
//   - read the request body via common.UnmarshalBodyReusable
//   - set headers on the response
//   - read ContextKeyUserId
func newCtxForResolve(t *testing.T, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(buf))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID > 0 {
		c.Set(string(constant.ContextKeyUserId), userID)
	}
	return c, w
}

func TestResolveAutoModel_NotAutoModel(t *testing.T) {
	c, w := newCtxForResolve(t, map[string]any{"messages": []any{}}, 1)
	client := smart_router_client.NewClient("http://unused", time.Second)

	got := resolveAutoModel(c, "gpt-4o", client)

	if got != "" {
		t.Errorf("non-auto model should return empty, got %q", got)
	}
	// No headers should be touched for non-auto models.
	if v := w.Header().Get("X-DeepRouter-Routed-Model"); v != "" {
		t.Errorf("should not set headers for non-auto, got %q", v)
	}
}

func TestResolveAutoModel_DisabledClient(t *testing.T) {
	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}, 42)
	disabled := smart_router_client.NewClient("", time.Second) // empty URL = disabled

	got := resolveAutoModel(c, VirtualModelAuto, disabled)

	if got != DefaultAutoFallbackModel {
		t.Errorf("disabled client should return fallback, got %q", got)
	}
	if reason := w.Header().Get("X-DeepRouter-Routed-Reason"); reason != "smart_router_disabled" {
		t.Errorf("reason header = %q, want smart_router_disabled", reason)
	}
	if model := w.Header().Get("X-DeepRouter-Routed-Model"); model != DefaultAutoFallbackModel {
		t.Errorf("model header = %q, want %s", model, DefaultAutoFallbackModel)
	}
}

func TestResolveAutoModel_NoMessages(t *testing.T) {
	// Smart-router can't decide without prompt content — code path falls
	// back to default + records the reason for debugging.
	c, w := newCtxForResolve(t, map[string]any{"messages": []any{}}, 1)
	url, cleanup := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("smart-router should NOT be called when messages are empty")
	})
	defer cleanup()
	client := smart_router_client.NewClient(url, time.Second)

	got := resolveAutoModel(c, VirtualModelAuto, client)

	if got != DefaultAutoFallbackModel {
		t.Errorf("no messages → fallback, got %q", got)
	}
	if reason := w.Header().Get("X-DeepRouter-Routed-Reason"); reason != "smart_router_no_messages" {
		t.Errorf("reason header = %q, want smart_router_no_messages", reason)
	}
}

func TestResolveAutoModel_Success(t *testing.T) {
	url, cleanup := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify the request we send is shaped right.
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"tenant_id":"42"`) {
			t.Errorf("smart-router got body without tenant_id=42: %s", body)
		}
		if !strings.Contains(string(body), `"role":"user"`) {
			t.Errorf("smart-router got body without messages: %s", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"primary":          "claude-haiku-4-5",
			"fallback_chain":   []string{"gpt-4o-mini"},
			"reason":           "short_question",
			"strategy_version": "heuristic-v1-test",
		})
	})
	defer cleanup()

	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   false,
	}, 42)
	client := smart_router_client.NewClient(url, time.Second)

	got := resolveAutoModel(c, VirtualModelAuto, client)

	if got != "claude-haiku-4-5" {
		t.Errorf("got primary %q, want claude-haiku-4-5", got)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Model"); v != "claude-haiku-4-5" {
		t.Errorf("model header = %q", v)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Reason"); v != "short_question" {
		t.Errorf("reason header = %q", v)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Strategy"); v != "heuristic-v1-test" {
		t.Errorf("strategy header = %q", v)
	}

	// Context keys for cross-model fallback + audit.
	if fc, ok := c.Get(string(constant.ContextKeySmartRouterFallback)); !ok {
		t.Error("ContextKeySmartRouterFallback not set")
	} else if chain, ok := fc.([]string); !ok || len(chain) != 1 || chain[0] != "gpt-4o-mini" {
		t.Errorf("fallback chain = %+v", fc)
	}
	if v, _ := c.Get(string(constant.ContextKeyAliasResolvedFrom)); v != VirtualModelAuto {
		t.Errorf("alias_resolved_from = %v", v)
	}
	if v, _ := c.Get(string(constant.ContextKeySmartRouterReason)); v != "short_question" {
		t.Errorf("reason ctx = %v", v)
	}
}

func TestResolveAutoModel_SmartRouterError(t *testing.T) {
	url, cleanup := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	defer cleanup()

	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}, 1)
	client := smart_router_client.NewClient(url, time.Second)

	got := resolveAutoModel(c, VirtualModelAuto, client)

	if got != DefaultAutoFallbackModel {
		t.Errorf("upstream 500 → fallback, got %q", got)
	}
	if reason := w.Header().Get("X-DeepRouter-Routed-Reason"); reason != "smart_router_error" {
		t.Errorf("reason = %q", reason)
	}
}

func TestResolveAutoModel_NoDecision(t *testing.T) {
	// Smart-router answered but didn't pick anything (e.g. constraints filtered
	// out every model). Our client treats that as nil decision.
	url, cleanup := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":               "no_model_matches_constraints",
			"fallback_to_default": "gpt-4o-mini",
		})
	})
	defer cleanup()

	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}, 1)
	client := smart_router_client.NewClient(url, time.Second)

	got := resolveAutoModel(c, VirtualModelAuto, client)

	if got != DefaultAutoFallbackModel {
		t.Errorf("no decision → fallback, got %q", got)
	}
	if reason := w.Header().Get("X-DeepRouter-Routed-Reason"); reason != "smart_router_no_decision" {
		t.Errorf("reason = %q", reason)
	}
}

func TestResolveAutoModel_HeadersAlwaysSet(t *testing.T) {
	// Even on failure paths the routing observability headers must surface,
	// otherwise customers can't debug "why didn't my auto request route".
	tests := []struct {
		name      string
		client    *smart_router_client.Client
		body      any
		wantModel string
	}{
		{
			name:      "disabled",
			client:    smart_router_client.NewClient("", time.Second),
			body:      map[string]any{"messages": []map[string]string{{"role": "user", "content": "hi"}}},
			wantModel: DefaultAutoFallbackModel,
		},
		{
			name:      "no_messages",
			client:    smart_router_client.NewClient("http://127.0.0.1:1", time.Second),
			body:      map[string]any{"messages": []any{}},
			wantModel: DefaultAutoFallbackModel,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newCtxForResolve(t, tc.body, 1)
			_ = resolveAutoModel(c, VirtualModelAuto, tc.client)
			if got := w.Header().Get("X-DeepRouter-Routed-Model"); got != tc.wantModel {
				t.Errorf("X-DeepRouter-Routed-Model = %q want %q", got, tc.wantModel)
			}
			if got := w.Header().Get("X-DeepRouter-Routed-Reason"); got == "" {
				t.Errorf("X-DeepRouter-Routed-Reason must be set on every code path")
			}
		})
	}
}

// --- token-whitelist re-filter (docs/adr/0007-auto-model-token-whitelist.md) ---
//
// Regression tests for the production bug where deeprouter-auto resolved to a
// model outside the token's model whitelist and every request 403'd with
// "该令牌无权访问模型 X" on a model the user never typed.

// setTokenWhitelist wires the context keys the auth middleware sets for a
// token that has model_limits enabled.
func setTokenWhitelist(c *gin.Context, entries ...string) {
	limits := map[string]bool{}
	for _, e := range entries {
		limits[e] = true
	}
	c.Set(string(constant.ContextKeyTokenModelLimitEnabled), true)
	c.Set(string(constant.ContextKeyTokenModelLimit), limits)
}

// stubGroupModels swaps the DB-backed group-model lookup for a fixed list,
// restoring the original on cleanup.
func stubGroupModels(t *testing.T, models ...string) {
	t.Helper()
	orig := groupModelsForWhitelistFallback
	groupModelsForWhitelistFallback = func(*gin.Context) []string { return models }
	t.Cleanup(func() { groupModelsForWhitelistFallback = orig })
}

func TestResolveAutoModel_WhitelistFallsToChain(t *testing.T) {
	// smart-router's primary is outside the whitelist but its fallback chain
	// has an allowed entry — that entry must win, silently, before the 403.
	url, cleanup := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"primary":        "claude-haiku-4-5",
			"fallback_chain": []string{"gpt-4o-mini"},
			"reason":         "short_question",
		})
	})
	defer cleanup()

	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}, 42)
	setTokenWhitelist(c, "gpt-4o-mini")
	client := smart_router_client.NewClient(url, time.Second)

	got := resolveAutoModel(c, VirtualModelAuto, client)

	if got != "gpt-4o-mini" {
		t.Errorf("whitelisted fallback should win, got %q", got)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Model"); v != "gpt-4o-mini" {
		t.Errorf("model header = %q", v)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Reason"); v != "short_question+token_whitelist" {
		t.Errorf("reason header = %q, want short_question+token_whitelist", v)
	}
	// The one allowed entry was promoted to primary; nothing is left to retry
	// through, so the cross-model fallback key must not be set.
	if _, ok := c.Get(string(constant.ContextKeySmartRouterFallback)); ok {
		t.Error("fallback chain should be empty after promotion")
	}
}

func TestResolveAutoModel_WhitelistWildcardMatchesChain(t *testing.T) {
	// Wildcard entries ("claude-*") must match the same way the later
	// Distribute check does.
	url, cleanup := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"primary":        "deepseek-v4-flash",
			"fallback_chain": []string{"claude-haiku-4-5", "gpt-4o-mini"},
			"reason":         "chinese_short",
		})
	})
	defer cleanup()

	c, _ := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "你好"}},
	}, 42)
	setTokenWhitelist(c, "claude-*")
	client := smart_router_client.NewClient(url, time.Second)

	if got := resolveAutoModel(c, VirtualModelAuto, client); got != "claude-haiku-4-5" {
		t.Errorf("wildcard whitelist should keep claude-haiku-4-5, got %q", got)
	}
}

func TestResolveAutoModel_WhitelistDegradesToCheapestGroupModel(t *testing.T) {
	// Neither primary nor chain passes the whitelist → degrade to the
	// cheapest whitelisted model the group can serve. Per-call-priced models
	// (dall-e-3) must be skipped even when whitelisted: a chat request routed
	// there fails.
	ratio_setting.InitRatioSettings() // gpt-4o=1.25, gpt-4=15, dall-e-3=per-call

	url, cleanup := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"primary":        "deepseek-v4-flash",
			"fallback_chain": []string{"claude-haiku-4-5"},
			"reason":         "chinese_short",
		})
	})
	defer cleanup()

	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "你好"}},
	}, 42)
	setTokenWhitelist(c, "gpt-4", "gpt-4o", "dall-e-3")
	stubGroupModels(t, "gpt-4", "dall-e-3", "gpt-4o")
	client := smart_router_client.NewClient(url, time.Second)

	if got := resolveAutoModel(c, VirtualModelAuto, client); got != "gpt-4o" {
		t.Errorf("should degrade to cheapest whitelisted chat model gpt-4o, got %q", got)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Reason"); v != "chinese_short+token_whitelist_degraded" {
		t.Errorf("reason header = %q", v)
	}
}

func TestResolveAutoModel_DisabledClientRespectsWhitelist(t *testing.T) {
	// Sidecar down + whitelist that excludes DefaultAutoFallbackModel: the
	// graceful-degradation pick must ALSO honour the whitelist, or the outage
	// path 403s for limited tokens.
	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}, 42)
	setTokenWhitelist(c, "claude-sonnet-4-6")
	stubGroupModels(t, "claude-sonnet-4-6")
	disabled := smart_router_client.NewClient("", time.Second)

	if got := resolveAutoModel(c, VirtualModelAuto, disabled); got != "claude-sonnet-4-6" {
		t.Errorf("degradation should stay inside the whitelist, got %q", got)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Reason"); v != "smart_router_disabled+token_whitelist_degraded" {
		t.Errorf("reason header = %q", v)
	}
}

func TestResolveAutoModel_WhitelistNothingUsableLeavesPick(t *testing.T) {
	// Whitelist matches nothing the group can serve. Keep the smart-router
	// pick and let Distribute's own 403 fire — the token can use nothing, and
	// masking that would only move the error somewhere less honest.
	url, cleanup := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"primary":        "claude-haiku-4-5",
			"fallback_chain": []string{"gpt-4o-mini"},
			"reason":         "short_question",
		})
	})
	defer cleanup()

	c, w := newCtxForResolve(t, map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}, 42)
	setTokenWhitelist(c, "no-such-model")
	stubGroupModels(t) // group serves nothing whitelisted
	client := smart_router_client.NewClient(url, time.Second)

	if got := resolveAutoModel(c, VirtualModelAuto, client); got != "claude-haiku-4-5" {
		t.Errorf("nothing usable → keep the pick unchanged, got %q", got)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Reason"); v != "short_question" {
		t.Errorf("reason must stay unchanged, got %q", v)
	}
}

// --- Gemini (`contents`) and Responses (`input`) protocol translation ---
//
// These two protocols carry the conversation under a different top-level key
// than OpenAI/Anthropic, so before the translators existed they parsed to zero
// messages and silently fell back to DefaultAutoFallbackModel — a 200 with the
// wrong model and no log line. Every test here asserts smart-router was
// actually CALLED, because "returned something sensible" is exactly what the
// bug already did.

// newCtxForRawBody is newCtxForResolve for bodies that must be sent as exact
// JSON — Responses' `input` is a raw message that is legitimately either a
// string or an array, and marshalling a Go value can't express both.
func newCtxForRawBody(t *testing.T, path, body string, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID > 0 {
		c.Set(string(constant.ContextKeyUserId), userID)
	}
	return c, w
}

// captureRouteBody stubs smart-router and records the raw body it received,
// so a test can assert on what the gateway actually translated.
func captureRouteBody(t *testing.T, got *string) (string, func()) {
	t.Helper()
	return stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*got = string(body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"primary":          "claude-haiku-4-5",
			"fallback_chain":   []string{},
			"reason":           "short_question",
			"strategy_version": "heuristic-v1-test",
		})
	})
}

func TestResolveAutoModel_GeminiContentsReachSmartRouter(t *testing.T) {
	var body string
	url, cleanup := captureRouteBody(t, &body)
	defer cleanup()

	c, w := newCtxForRawBody(t, "/v1beta/models/deeprouter-auto:generateContent",
		`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`, 42)

	got := resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, time.Second))

	if body == "" {
		t.Fatal("smart-router was NOT called for a Gemini-format request")
	}
	if got != "claude-haiku-4-5" {
		t.Errorf("got %q, want claude-haiku-4-5", got)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Reason"); v == "smart_router_no_messages" {
		t.Error("reason is still smart_router_no_messages — contents were not translated")
	}
	if !strings.Contains(body, `"hi"`) {
		t.Errorf("prompt text missing from smart-router body: %s", body)
	}
}

func TestResolveAutoModel_GeminiMultiPartTextJoined(t *testing.T) {
	var body string
	url, cleanup := captureRouteBody(t, &body)
	defer cleanup()

	// One Gemini message may carry several parts; taking only the first
	// loses most of the prompt and skews every heuristic downstream.
	c, _ := newCtxForRawBody(t, "/v1beta/models/deeprouter-auto:generateContent",
		`{"contents":[{"role":"user","parts":[{"text":"first"},{"text":"second"}]}]}`, 42)

	resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, time.Second))

	if !strings.Contains(body, "first") || !strings.Contains(body, "second") {
		t.Errorf("both parts should be joined, got: %s", body)
	}
}

func TestResolveAutoModel_GeminiRoleModelBecomesAssistant(t *testing.T) {
	var body string
	url, cleanup := captureRouteBody(t, &body)
	defer cleanup()

	c, _ := newCtxForRawBody(t, "/v1beta/models/deeprouter-auto:generateContent",
		`{"contents":[{"role":"user","parts":[{"text":"q"}]},`+
			`{"role":"model","parts":[{"text":"a"}]}]}`, 42)

	resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, time.Second))

	if strings.Contains(body, `"role":"model"`) {
		t.Errorf("Gemini's 'model' role must be mapped to 'assistant', got: %s", body)
	}
	if !strings.Contains(body, `"role":"assistant"`) {
		t.Errorf("no assistant role in translated body: %s", body)
	}
}

func TestResolveAutoModel_GeminiSystemInstruction(t *testing.T) {
	var body string
	url, cleanup := captureRouteBody(t, &body)
	defer cleanup()

	// systemInstruction sits OUTSIDE contents — dropping it loses the system
	// prompt, which is often the longest and most routing-relevant text.
	c, _ := newCtxForRawBody(t, "/v1beta/models/deeprouter-auto:generateContent",
		`{"systemInstruction":{"parts":[{"text":"you are a translator"}]},`+
			`"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`, 42)

	resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, time.Second))

	if !strings.Contains(body, "you are a translator") {
		t.Errorf("systemInstruction missing from smart-router body: %s", body)
	}
	if !strings.Contains(body, `"role":"system"`) {
		t.Errorf("systemInstruction should map to a system message: %s", body)
	}
}

func TestResolveAutoModel_ResponsesInputStringReachesSmartRouter(t *testing.T) {
	var body string
	url, cleanup := captureRouteBody(t, &body)
	defer cleanup()

	// Single-turn Responses requests send `input` as a bare string.
	c, w := newCtxForRawBody(t, "/v1/responses", `{"input":"hi"}`, 42)

	got := resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, time.Second))

	if body == "" {
		t.Fatal("smart-router was NOT called for a Responses-format request")
	}
	if got != "claude-haiku-4-5" {
		t.Errorf("got %q, want claude-haiku-4-5", got)
	}
	if v := w.Header().Get("X-DeepRouter-Routed-Reason"); v == "smart_router_no_messages" {
		t.Error("reason is still smart_router_no_messages — input was not translated")
	}
	if !strings.Contains(body, `"hi"`) {
		t.Errorf("prompt text missing from smart-router body: %s", body)
	}
}

func TestResolveAutoModel_ResponsesInputArray(t *testing.T) {
	var body string
	url, cleanup := captureRouteBody(t, &body)
	defer cleanup()

	// Multi-turn sends an array; each item's content is itself either a
	// string or an array of typed parts. Codex CLI sends this shape.
	c, _ := newCtxForRawBody(t, "/v1/responses",
		`{"input":[{"role":"user","content":"first"},`+
			`{"role":"assistant","content":[{"type":"output_text","text":"second"}]}]}`, 42)

	resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, time.Second))

	if !strings.Contains(body, "first") || !strings.Contains(body, "second") {
		t.Errorf("both input items should be translated, got: %s", body)
	}
	if !strings.Contains(body, `"role":"assistant"`) {
		t.Errorf("roles should survive translation: %s", body)
	}
}

func TestResolveAutoModel_ResponsesInstructions(t *testing.T) {
	var body string
	url, cleanup := captureRouteBody(t, &body)
	defer cleanup()

	// `instructions` is Responses' system prompt — same shape of trap as
	// Gemini's systemInstruction: a top-level field outside the turn list.
	c, _ := newCtxForRawBody(t, "/v1/responses",
		`{"instructions":"you are a translator","input":"hi"}`, 42)

	resolveAutoModel(c, VirtualModelAuto, smart_router_client.NewClient(url, time.Second))

	if !strings.Contains(body, "you are a translator") {
		t.Errorf("instructions missing from smart-router body: %s", body)
	}
	if !strings.Contains(body, `"role":"system"`) {
		t.Errorf("instructions should map to a system message: %s", body)
	}
}
