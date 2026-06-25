package relay

// Integration test for the kids_mode strict output filter hook installed in
// GeminiHelper (relay/gemini_handler.go). Like ResponsesHelper, GeminiHelper
// goes straight through adaptor.DoRequest/adaptor.DoResponse
// (GeminiTextGenerationHandler in relay/channel/gemini/relay-gemini-native.go,
// which also calls service.IOCopyBytesGracefully) — no
// ChatCompletionsToResponsesPolicy mutation is needed.
//
// This test exercises the full chain:
//
//	policy.Decision{EnforceStrictOutputFilter: true} in gin context
//	  -> GeminiHelper -> wrapOutputFilterWriter -> adaptor.DoResponse
//	  -> mock Gemini upstream (returns a blocked keyword)
//	  -> outputFilterWriter.finalize() classifies + replaces the body
//	  -> client receives a kids.SafeFallbackText()-based Gemini body with NO
//	     trace of the blocked text
//	  -> constant.ContextKeyOutputFilterViolations recorded in gin context
//
// Run with:
//
//	go test ./relay/... -run TestGeminiHelper_OutputFilterInstalledBeforeDoResponse -v

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

	"github.com/gin-gonic/gin"
)

func TestGeminiHelper_OutputFilterInstalledBeforeDoResponse(t *testing.T) {
	service.InitHttpClient()
	withDBBypass(t)

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

	// Mock upstream: Gemini generateContent non-stream JSON response containing
	// the blocked keyword "murder" (category "violence" per
	// internal/kids/output_filter.go StrictOutputBlocklist).
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe.snapshot(c, spy)

		finishReason := "STOP"
		resp := dto.GeminiChatResponse{
			Candidates: []dto.GeminiChatCandidate{{
				Content: dto.GeminiChatContent{
					Role:  "model",
					Parts: []dto.GeminiPart{{Text: "I will murder this task for you."}},
				},
				FinishReason: &finishReason,
				Index:        0,
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(resp)
		_, _ = w.Write(body)
	}))
	t.Cleanup(upstream.Close)

	requestBody := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{
			Role:  "user",
			Parts: []dto.GeminiPart{{Text: "hello"}},
		}},
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-pro:generateContent", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-test")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gemini-pro")
	common.SetContextKey(c, constant.ContextKeyPolicyDecision, policy.Decision{EnforceStrictOutputFilter: true})

	info := relaycommon.GenRelayInfoGemini(c, requestBody)

	apiErr := GeminiHelper(c, info)
	if apiErr != nil {
		t.Fatalf("GeminiHelper returned error: %v", apiErr)
	}

	body := recorder.Body.Bytes()

	// (b) The original blocked text must never reach the client.
	if bytes.Contains(body, []byte("murder")) {
		t.Fatalf("response body must not contain the blocked text 'murder'; got: %s", body)
	}

	// (c) The body must parse as exactly one well-formed JSON object — no
	// concatenation/duplication.
	var resp dto.GeminiChatResponse
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body as dto.GeminiChatResponse: %v\nbody: %s", err, body)
	}
	if dec.More() {
		t.Fatalf("response body contains more than one JSON value (no leakage/duplication expected); body: %s", body)
	}

	// (a) The fallback body must contain the safe fallback text and finish
	// reason "SAFETY".
	if len(resp.Candidates) != 1 || len(resp.Candidates[0].Content.Parts) != 1 {
		t.Fatalf("Candidates: want 1 candidate with 1 part, got %+v (body: %s)", resp.Candidates, body)
	}
	if resp.Candidates[0].Content.Parts[0].Text != kids.SafeFallbackText() {
		t.Errorf("Candidates[0].Content.Parts[0].Text: want %q, got %q", kids.SafeFallbackText(), resp.Candidates[0].Content.Parts[0].Text)
	}
	if resp.Candidates[0].FinishReason == nil || *resp.Candidates[0].FinishReason != "SAFETY" {
		t.Errorf("Candidates[0].FinishReason: want %q, got %v", "SAFETY", resp.Candidates[0].FinishReason)
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

	// (a)/(b) wrapOutputFilterWriter installs outputFilterWriter before
	// adaptor.DoResponse's upstream call, and nothing leaks to the
	// underlying writer before restore() runs.
	assertOutputFilterWrapTiming(t, c, spy, &probe)
}
