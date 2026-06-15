package relay

// Integration test for the kids_mode strict output filter hook installed in
// ClaudeHelper (relay/claude_handler.go), specifically the
// chatCompletionsViaResponses early-return branch (same branch shape as
// TextHelper, but produces a Claude Messages wire response via
// service.ResponseOpenAI2Claude / OaiResponsesToChatHandler dispatched on
// info.RelayFormat == types.RelayFormatClaude).
//
// This test exercises the full chain:
//
//	policy.Decision{EnforceStrictOutputFilter: true} in gin context
//	  -> ClaudeHelper -> wrapOutputFilterWriter -> chatCompletionsViaResponses
//	  -> mock OpenAI Responses upstream (returns a blocked keyword)
//	  -> outputFilterWriter.finalize() classifies + replaces the body
//	  -> client receives a kids.SafeFallbackText()-based Claude Messages body
//	     with NO trace of the blocked text
//	  -> constant.ContextKeyOutputFilterViolations recorded in gin context
//
// Run with:
//
//	go test ./relay/... -run TestClaudeHelper_ChatCompletionsViaResponses_OutputFilterAppliedBeforeEarlyReturn -v

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/internal/kids"
	"github.com/QuantumNous/new-api/internal/policy"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

func TestClaudeHelper_ChatCompletionsViaResponses_OutputFilterAppliedBeforeEarlyReturn(t *testing.T) {
	service.InitHttpClient()
	withDBBypass(t)

	// Force ShouldChatCompletionsUseResponsesGlobal(...) == true so
	// ClaudeHelper takes the chatCompletionsViaResponses early-return branch.
	gs := model_setting.GetGlobalSettings()
	prev := gs.ChatCompletionsToResponsesPolicy
	gs.ChatCompletionsToResponsesPolicy = model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   true,
		ModelPatterns: []string{".*"},
	}
	t.Cleanup(func() { gs.ChatCompletionsToResponsesPolicy = prev })

	// Build the gin test context BEFORE the mock upstream server, so the
	// upstream handler closure can probe c.Writer / the spy writer at
	// HTTP-call time — strictly before wrapOutputFilterWriter's
	// restore()/finalize() can have run.
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	spy := &outputFilterWriterSpy{ResponseWriter: c.Writer}
	c.Writer = spy

	var probe outputFilterWrapProbe

	// Mock upstream: OpenAI Responses API non-stream JSON response containing
	// the blocked keyword "murder" (category "violence" per
	// internal/kids/output_filter.go StrictOutputBlocklist).
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe.snapshot(c, spy)

		resp := dto.OpenAIResponsesResponse{
			Object: "response",
			Status: json.RawMessage(`"completed"`),
			Output: []dto.ResponsesOutput{{
				Type:   "message",
				Role:   "assistant",
				Status: "completed",
				Content: []dto.ResponsesOutputContent{{
					Type:        "output_text",
					Text:        "I will murder this task for you.",
					Annotations: []interface{}{},
				}},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(resp)
		_, _ = w.Write(body)
	}))
	t.Cleanup(upstream.Close)

	maxTokens := uint(100)
	requestBody := &dto.ClaudeRequest{
		Model:     "gpt-4o-mini",
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: "hello"}},
		MaxTokens: &maxTokens,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-test")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o-mini")
	common.SetContextKey(c, constant.ContextKeyPolicyDecision, policy.Decision{EnforceStrictOutputFilter: true})

	info := relaycommon.GenRelayInfoClaude(c, requestBody)

	apiErr := ClaudeHelper(c, info)
	if apiErr != nil {
		t.Fatalf("ClaudeHelper returned error: %v", apiErr)
	}

	body := recorder.Body.Bytes()

	// (b) The original blocked text must never reach the client.
	if bytes.Contains(body, []byte("murder")) {
		t.Fatalf("response body must not contain the blocked text 'murder'; got: %s", body)
	}

	// (c) The body must parse as exactly one well-formed JSON object — no
	// concatenation/duplication.
	var resp dto.ClaudeResponse
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body as dto.ClaudeResponse: %v\nbody: %s", err, body)
	}
	if dec.More() {
		t.Fatalf("response body contains more than one JSON value (no leakage/duplication expected); body: %s", body)
	}

	// (a) The fallback body must be in the Claude Messages shape with the
	// safe fallback text and stop_reason "end_turn".
	if resp.Type != "message" {
		t.Errorf("Type: want %q, got %q", "message", resp.Type)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("Content: want len 1, got %d (body: %s)", len(resp.Content), body)
	}
	if resp.Content[0].Type != "text" {
		t.Errorf("Content[0].Type: want %q, got %q", "text", resp.Content[0].Type)
	}
	if resp.Content[0].Text == nil || *resp.Content[0].Text != kids.SafeFallbackText() {
		t.Errorf("Content[0].Text: want %q, got %v", kids.SafeFallbackText(), resp.Content[0].Text)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason: want %q, got %q", "end_turn", resp.StopReason)
	}

	// (d) ContextKeyOutputFilterViolations must be recorded and contain
	// "violence" (the category for "murder").
	raw, ok := c.Get(string(constant.ContextKeyOutputFilterViolations))
	if !ok {
		t.Fatal("ContextKeyOutputFilterViolations not set in gin context")
	}
	categories, ok := raw.([]string)
	if !ok {
		t.Fatalf("ContextKeyOutputFilterViolations has unexpected type %T", raw)
	}
	found := false
	for _, cat := range categories {
		if cat == kids.OutputCategoryViolence {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ContextKeyOutputFilterViolations: want to contain %q, got %v", kids.OutputCategoryViolence, categories)
	}

	// (a)/(b) wrapOutputFilterWriter installs outputFilterWriter before the
	// chatCompletionsViaResponses early-return branch's upstream call, and
	// nothing leaks to the underlying writer before restore() runs.
	assertOutputFilterWrapTiming(t, c, spy, &probe)
}
