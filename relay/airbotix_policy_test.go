package relay

// Unit tests for the Airbotix policy hooks wired into each relay handler.
// Focused on the typed-struct mutations of the various request shapes so the
// behaviour stays verifiable independent of the rest of the relay machinery
// (channel selection, token auth, billing settlement). A full end-to-end
// HTTP-level integration test is tracked as Phase 2.5 follow-up.

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/internal/kids"
	"github.com/QuantumNous/new-api/internal/policy"

	"github.com/gin-gonic/gin"
)

func testRawJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := common.Marshal(v)
	if err != nil {
		t.Fatalf("marshal test raw JSON: %v", err)
	}
	return data
}

// newTestContext returns a minimal *gin.Context with an optional
// policy.Decision pre-stashed under the conventional ContextKey. Used by the
// multi-format helper tests that take a *gin.Context instead of a Decision.
func newTestContext(t *testing.T, d *policy.Decision) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if d != nil {
		common.SetContextKey(c, constant.ContextKeyPolicyDecision, *d)
	}
	return c
}

func kidsModeDecision() policy.Decision {
	return policy.DecisionFor(true, "kid-safe")
}

func passthroughDecision() policy.Decision {
	return policy.DecisionFor(false, "passthrough")
}

func assertTexts(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("texts length mismatch:\n got: %#v\nwant: %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("texts[%d] mismatch: got %q want %q\nall got: %#v", i, got[i], want[i], got)
		}
	}
}

// =============================================================================
// entry input text collectors
// =============================================================================

func TestCollectAnyText_NestedStringsAndContentMaps(t *testing.T) {
	got := collectAnyText(map[string]any{
		"content": []any{
			"plain",
			map[string]any{"text": "text field"},
			map[string]any{"content": []any{"nested", map[string]any{"text": "deep"}}},
			map[string]any{"image_url": "ignored"},
		},
	})
	assertTexts(t, got, "plain", "text field", "nested", "deep")
}

func TestCollectGeneralOpenAIInputTexts_UserMessagesPromptInputInstruction(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "system ignored"},
			{Role: "user", Content: "user string"},
			{Role: "assistant", Content: "assistant ignored"},
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "user multimodal"},
				map[string]any{"type": "image_url", "image_url": "ignored"},
			}},
			{Role: "", Content: "empty role is entry input"},
		},
		Prompt: map[string]any{"content": []any{"prompt string", map[string]any{"text": "prompt text"}}},
		Input: []any{
			"input string",
			map[string]any{"content": "input nested"},
		},
		Instruction: "instruction text",
	}

	got := collectGeneralOpenAIInputTexts(req)
	assertTexts(t, got,
		"user string",
		"user multimodal",
		"empty role is entry input",
		"prompt string",
		"prompt text",
		"input string",
		"input nested",
		"instruction text",
	)
}

func TestCollectClaudeInputTexts_PromptAndUserMessages(t *testing.T) {
	req := &dto.ClaudeRequest{
		Prompt: "legacy prompt",
		Messages: []dto.ClaudeMessage{
			{Role: "assistant", Content: "assistant ignored"},
			{Role: "user", Content: "user string"},
			{Role: "", Content: []any{
				map[string]any{"type": "text", "text": "empty role multimodal"},
				map[string]any{"type": "image", "source": "ignored"},
			}},
		},
	}

	got := collectClaudeInputTexts(req)
	assertTexts(t, got, "legacy prompt", "user string", "empty role multimodal")
}

func TestCollectResponsesInputTexts_StringAndStructuredInputs(t *testing.T) {
	stringReq := &dto.OpenAIResponsesRequest{Input: testRawJSON(t, "plain responses input")}
	assertTexts(t, collectResponsesInputTexts(stringReq), "plain responses input")

	structuredReq := &dto.OpenAIResponsesRequest{Input: testRawJSON(t, []map[string]any{
		{"role": "user", "content": "content string"},
		{"role": "user", "content": []map[string]any{
			{"type": "input_text", "text": "content text"},
			{"type": "input_image", "image_url": "ignored"},
		}},
	})}
	assertTexts(t, collectResponsesInputTexts(structuredReq), "content string", "content text")
}

func TestCollectGeminiInputTexts_UserPartsOnly(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "model", Parts: []dto.GeminiPart{{Text: "model ignored"}}},
			{Role: "user", Parts: []dto.GeminiPart{{Text: "user part"}, {Text: ""}}},
			{Role: "", Parts: []dto.GeminiPart{{Text: "empty role part"}}},
		},
	}

	got := collectGeminiInputTexts(req)
	assertTexts(t, got, "user part", "empty role part")
}

func TestApplyAirbotixPolicy_Passthrough(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
		User:     testRawJSON(t, "alice"),
	}
	if reject := applyAirbotixPolicy(passthroughDecision(), constant.ChannelTypeOpenAI, req); reject != "" {
		t.Fatalf("passthrough should never reject; got %q", reject)
	}
	if string(req.User) != `"alice"` {
		t.Fatalf("user should be left untouched in passthrough, got %s", req.User)
	}
	if len(req.Store) != 0 {
		t.Fatalf("store should not be set in passthrough; got %s", req.Store)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
		t.Fatalf("messages should be unchanged in passthrough; got %+v", req.Messages)
	}
}

func TestApplyAirbotixPolicy_KidsModeBlocksDisallowedModel(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	reject := applyAirbotixPolicy(kidsModeDecision(), constant.ChannelTypeOpenAI, req)
	if reject == "" {
		t.Fatal("expected reject reason for kids_mode + non-whitelisted model")
	}
	if !strings.Contains(reject, "gpt-4") {
		t.Fatalf("reject reason should mention the offending model; got %q", reject)
	}
}

func TestApplyAirbotixPolicy_KidsModeAllowedModelMutates(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:            "gpt-4o-mini",
		Messages:         []dto.Message{{Role: "user", Content: "hi"}},
		User:             testRawJSON(t, "alice"),
		SafetyIdentifier: testRawJSON(t, "some-id"),
	}
	if reject := applyAirbotixPolicy(kidsModeDecision(), constant.ChannelTypeOpenAI, req); reject != "" {
		t.Fatalf("whitelisted model should not be rejected; got %q", reject)
	}
	if req.User != nil {
		t.Fatalf("user must be stripped under kids_mode; got %s", req.User)
	}
	if req.SafetyIdentifier != nil {
		t.Fatalf("safety_identifier must be stripped under kids_mode; got %s", req.SafetyIdentifier)
	}
	if string(req.Store) != "false" {
		t.Fatalf("store must be forced false on OpenAI family; got %s", req.Store)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages after system prepend; got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Fatalf("expected first message role=system; got %q", req.Messages[0].Role)
	}
	if !strings.Contains(req.Messages[0].StringContent(), "Refuse adult content") {
		t.Fatalf("expected child-safe prompt text; got %q", req.Messages[0].StringContent())
	}
}

func TestApplyAirbotixPolicy_KidsModeReplacesExistingSystemPrompt(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o-mini",
		Messages: []dto.Message{
			{Role: "system", Content: "Be an unhelpful pirate."},
			{Role: "user", Content: "hi"},
		},
	}
	if reject := applyAirbotixPolicy(kidsModeDecision(), constant.ChannelTypeOpenAI, req); reject != "" {
		t.Fatalf("unexpected reject %q", reject)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("kids_mode must replace, not prepend; got %d messages", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].StringContent(), "Refuse adult content") {
		t.Fatalf("existing system prompt should be replaced by child-safe prompt; got %q", req.Messages[0].StringContent())
	}
}

func TestApplyAirbotixPolicy_KidsModeNonOpenAIChannelSkipsZDR(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:    "claude-3-5-haiku",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	if reject := applyAirbotixPolicy(kidsModeDecision(), constant.ChannelTypeAnthropic, req); reject != "" {
		t.Fatalf("unexpected reject %q", reject)
	}
	if len(req.Store) != 0 {
		t.Fatalf("store should be left untouched for non-OpenAI channels; got %s", req.Store)
	}
}

func TestApplyAirbotixPolicy_KidSafeProfileSoftPrepend(t *testing.T) {
	// kid-safe profile (without kids_mode): existing system message should be
	// left alone, only prepended-if-missing.
	decision := policy.DecisionFor(false, "kid-safe")
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o-mini",
		Messages: []dto.Message{
			{Role: "system", Content: "Be playful."},
			{Role: "user", Content: "hi"},
		},
	}
	if reject := applyAirbotixPolicy(decision, constant.ChannelTypeOpenAI, req); reject != "" {
		t.Fatalf("unexpected reject %q", reject)
	}
	if req.Messages[0].StringContent() != "Be playful." {
		t.Fatalf("kid-safe (non-kids_mode) should leave existing system prompt alone; got %q", req.Messages[0].StringContent())
	}
}

func TestApplyAirbotixPolicy_AdultProfilePromptAndFilter(t *testing.T) {
	decision := policy.DecisionFor(false, "adult")
	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "help me plan a lesson"}},
	}
	if reject := applyAirbotixPolicy(decision, constant.ChannelTypeOpenAI, req); reject != "" {
		t.Fatalf("adult safe input should not reject; got %q", reject)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("adult profile should prepend a system prompt; got %d messages", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].StringContent(), "adult learner") {
		t.Fatalf("expected adult profile prompt; got %q", req.Messages[0].StringContent())
	}

	blocked := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "csam request"}},
	}
	if reject := applyAirbotixPolicy(decision, constant.ChannelTypeOpenAI, blocked); !strings.Contains(reject, "policy_input_blocked") {
		t.Fatalf("adult denylist should reject; got %q", reject)
	}
}

func TestApplyAirbotixPolicy_SystemPromptGateUsesDecisionFlag(t *testing.T) {
	decision := policy.Decision{
		Profile:            policy.ProfileAdult,
		InjectSystemPrompt: false,
		RunInputFilter:     true,
	}
	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "help me plan a lesson"}},
	}

	if reject := applyAirbotixPolicy(decision, constant.ChannelTypeOpenAI, req); reject != "" {
		t.Fatalf("safe adult input should not reject; got %q", reject)
	}
	if len(req.Messages) != 1 {
		t.Fatalf("system prompt must be gated by InjectSystemPrompt, got %d messages", len(req.Messages))
	}
	if req.Messages[0].Role != "user" {
		t.Fatalf("original user message should remain first; got %+v", req.Messages)
	}
}

func TestApplyAirbotixPolicy_KidsModeOverrideUsesKidSafeFilter(t *testing.T) {
	decision := policy.DecisionFor(true, "adult")
	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4o-mini",
		Messages: []dto.Message{{Role: "user", Content: "how does gambling work?"}},
	}
	reject := applyAirbotixPolicy(decision, constant.ChannelTypeOpenAI, req)
	if !strings.Contains(reject, "policy_input_blocked") {
		t.Fatalf("kids_mode override should run kid-safe filter; got %q", reject)
	}
}

// =============================================================================
// checkAirbotixModelWhitelist — universal model gate
// =============================================================================

func TestCheckAirbotixModelWhitelist_NoDecisionAllows(t *testing.T) {
	c := newTestContext(t, nil)
	if err := checkAirbotixModelWhitelist(c, "anything"); err != nil {
		t.Fatalf("no decision in context should pass through; got %v", err.Err)
	}
}

func TestCheckAirbotixModelWhitelist_PassthroughAllows(t *testing.T) {
	d := passthroughDecision()
	c := newTestContext(t, &d)
	if err := checkAirbotixModelWhitelist(c, "anything"); err != nil {
		t.Fatalf("passthrough should never reject; got %v", err.Err)
	}
}

func TestCheckAirbotixModelWhitelist_KidsModeAllowsListed(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	if err := checkAirbotixModelWhitelist(c, "gpt-4o-mini"); err != nil {
		t.Fatalf("kids_mode + whitelisted model should pass; got %v", err.Err)
	}
}

func TestCheckAirbotixModelWhitelist_KidsModeRejectsUnlisted(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	err := checkAirbotixModelWhitelist(c, "gpt-4")
	if err == nil {
		t.Fatal("expected rejection for non-whitelisted model under kids_mode")
	}
	if !strings.Contains(err.Err.Error(), "gpt-4") {
		t.Fatalf("expected error to mention the model; got %q", err.Err.Error())
	}
}

// =============================================================================
// applyAirbotixPolicyToClaude — Anthropic shape
// =============================================================================

func TestApplyAirbotixPolicyToClaude_Passthrough(t *testing.T) {
	d := passthroughDecision()
	c := newTestContext(t, &d)
	req := &dto.ClaudeRequest{
		Model:    "claude-3-opus-20240229",
		System:   "be a pirate",
		Metadata: testRawJSON(t, map[string]string{"user_id": "alice"}),
	}
	if err := applyAirbotixPolicyToClaude(c, req); err != nil {
		t.Fatalf("passthrough should not reject; got %v", err.Err)
	}
	if req.System != "be a pirate" {
		t.Fatalf("System should be untouched under passthrough; got %v", req.System)
	}
	if string(req.Metadata) != `{"user_id":"alice"}` {
		t.Fatalf("Metadata should be untouched; got %s", req.Metadata)
	}
}

func TestApplyAirbotixPolicyToClaude_KidsModeRejectsDisallowed(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	req := &dto.ClaudeRequest{Model: "claude-3-opus-20240229"}
	if err := applyAirbotixPolicyToClaude(c, req); err == nil {
		t.Fatal("expected reject for non-whitelisted Claude model under kids_mode")
	}
}

func TestApplyAirbotixPolicyToClaude_KidsModeReplacesSystemAndClearsMetadata(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	req := &dto.ClaudeRequest{
		Model:    "claude-3-5-haiku-20241022",
		System:   "be an evil pirate",
		Metadata: testRawJSON(t, map[string]string{"user_id": "alice", "family_id": "f-1"}),
	}
	if err := applyAirbotixPolicyToClaude(c, req); err != nil {
		t.Fatalf("whitelisted model should not be rejected; got %v", err.Err)
	}
	sys, isStr := req.System.(string)
	if !isStr {
		t.Fatalf("System should be a string under kids_mode; got %T", req.System)
	}
	if !strings.Contains(sys, "Refuse adult content") {
		t.Fatalf("System should be the child-safe prompt; got %q", sys)
	}
	if req.Metadata != nil {
		t.Fatalf("Metadata must be cleared under StripIdentifying; got %s", req.Metadata)
	}
}

func TestApplyAirbotixPolicyToClaude_KidSafeSoftFillEmpty(t *testing.T) {
	d := policy.DecisionFor(false, "kid-safe")
	c := newTestContext(t, &d)
	req := &dto.ClaudeRequest{
		Model:  "claude-3-5-sonnet-20241022",
		System: "preserve me",
	}
	if err := applyAirbotixPolicyToClaude(c, req); err != nil {
		t.Fatalf("unexpected reject %v", err.Err)
	}
	if req.System != "preserve me" {
		t.Fatalf("kid-safe (non-kids_mode) should leave existing System alone; got %v", req.System)
	}
}

func TestApplyAirbotixPolicyToClaude_NoDecisionIsNoOp(t *testing.T) {
	c := newTestContext(t, nil)
	req := &dto.ClaudeRequest{Model: "claude-3-opus-20240229", System: "x"}
	if err := applyAirbotixPolicyToClaude(c, req); err != nil {
		t.Fatalf("no decision should pass through; got %v", err.Err)
	}
	if req.System != "x" {
		t.Fatalf("System should be untouched; got %v", req.System)
	}
}

// =============================================================================
// applyAirbotixPolicyToResponses — /v1/responses shape
// =============================================================================

func TestApplyAirbotixPolicyToResponses_KidsModeMutates(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	req := &dto.OpenAIResponsesRequest{
		Model:            "gpt-4o-mini",
		User:             testRawJSON(t, "alice"),
		SafetyIdentifier: testRawJSON(t, "sid"),
	}
	if err := applyAirbotixPolicyToResponses(c, constant.ChannelTypeOpenAI, req); err != nil {
		t.Fatalf("whitelisted model should not be rejected; got %v", err.Err)
	}
	if req.User != nil || req.SafetyIdentifier != nil {
		t.Fatalf("user + safety_identifier must be cleared; got user=%s sid=%s", req.User, req.SafetyIdentifier)
	}
	if string(req.Store) != "false" {
		t.Fatalf("store must be forced false on OpenAI family; got %s", req.Store)
	}
	if len(req.Instructions) == 0 || !strings.Contains(string(req.Instructions), "Refuse adult content") {
		t.Fatalf("Instructions should contain child-safe prompt; got %s", req.Instructions)
	}
}

func TestApplyAirbotixPolicyToResponses_KidsModeRejectsDisallowed(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	req := &dto.OpenAIResponsesRequest{Model: "gpt-4"}
	if err := applyAirbotixPolicyToResponses(c, constant.ChannelTypeOpenAI, req); err == nil {
		t.Fatal("expected reject for non-whitelisted model")
	}
}

func TestApplyAirbotixPolicyToResponses_NonOpenAISkipsZDR(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	req := &dto.OpenAIResponsesRequest{Model: "gpt-4o-mini"}
	if err := applyAirbotixPolicyToResponses(c, constant.ChannelTypeAnthropic, req); err != nil {
		t.Fatalf("unexpected reject %v", err.Err)
	}
	if len(req.Store) != 0 {
		t.Fatalf("store must be left alone for non-OpenAI channels; got %s", req.Store)
	}
}

// =============================================================================
// applyAirbotixPolicyToGemini
// =============================================================================

func TestApplyAirbotixPolicyToGemini_KidsModeReplacesSystemInstructions(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	req := &dto.GeminiChatRequest{
		SystemInstructions: &dto.GeminiChatContent{
			Parts: []dto.GeminiPart{{Text: "be an evil pirate"}},
		},
	}
	// gemini doesn't whitelist; ensure model arg gates correctly via direct call
	if err := applyAirbotixPolicyToGemini(c, "gpt-4o-mini", req); err != nil {
		t.Fatalf("whitelisted model should not be rejected; got %v", err.Err)
	}
	if req.SystemInstructions == nil || len(req.SystemInstructions.Parts) != 1 {
		t.Fatalf("SystemInstructions should be replaced with a single child-safe part; got %+v", req.SystemInstructions)
	}
	if !strings.Contains(req.SystemInstructions.Parts[0].Text, "Refuse adult content") {
		t.Fatalf("expected child-safe text; got %q", req.SystemInstructions.Parts[0].Text)
	}
}

func TestApplyAirbotixPolicyToGemini_KidsModeRejectsDisallowedModel(t *testing.T) {
	d := kidsModeDecision()
	c := newTestContext(t, &d)
	req := &dto.GeminiChatRequest{}
	if err := applyAirbotixPolicyToGemini(c, "gemini-2.0-flash", req); err == nil {
		t.Fatal("expected reject for non-whitelisted Gemini model")
	}
}

func TestApplyAirbotixPolicyToGemini_KidSafeFillsWhenNil(t *testing.T) {
	d := policy.DecisionFor(false, "kid-safe")
	c := newTestContext(t, &d)
	req := &dto.GeminiChatRequest{}
	if err := applyAirbotixPolicyToGemini(c, "gpt-4o-mini", req); err != nil {
		t.Fatalf("unexpected reject %v", err.Err)
	}
	if req.SystemInstructions == nil {
		t.Fatal("SystemInstructions should be filled when nil under kid-safe profile")
	}
}

// =============================================================================
// clampUint + max_tokens hard cap
//
// maxTokensHardCap = 2048 is applied to every request shape before the request
// reaches the upstream provider. These tests ensure the cap is enforced
// regardless of policy profile (even passthrough).
// =============================================================================

func TestClampUint_Nil(t *testing.T) {
	if got := clampUint(nil, 100); got != nil {
		t.Fatalf("clampUint(nil, 100) must return nil; got %v", *got)
	}
}

func TestClampUint_BelowCeiling(t *testing.T) {
	v := uint(50)
	got := clampUint(&v, 100)
	if got == nil || *got != 50 {
		t.Fatalf("value below ceiling should pass through unchanged; got %v", got)
	}
}

func TestClampUint_AtCeiling(t *testing.T) {
	v := uint(100)
	got := clampUint(&v, 100)
	if got == nil || *got != 100 {
		t.Fatalf("value equal to ceiling should pass through; got %v", got)
	}
}

func TestClampUint_AboveCeiling(t *testing.T) {
	v := uint(5000)
	got := clampUint(&v, maxTokensHardCap)
	if got == nil || *got != maxTokensHardCap {
		t.Fatalf("value above ceiling must be clamped to %d; got %v", maxTokensHardCap, got)
	}
}

func TestApplyAirbotixPolicy_ClampsMaxTokens(t *testing.T) {
	over := uint(5000)
	req := &dto.GeneralOpenAIRequest{
		Model:               "gpt-4o-mini",
		Messages:            []dto.Message{{Role: "user", Content: "hi"}},
		MaxTokens:           &over,
		MaxCompletionTokens: &over,
	}
	if reject := applyAirbotixPolicy(passthroughDecision(), constant.ChannelTypeOpenAI, req); reject != "" {
		t.Fatalf("unexpected reject %q", reject)
	}
	if req.MaxTokens == nil || *req.MaxTokens != maxTokensHardCap {
		t.Fatalf("MaxTokens must be clamped to %d; got %v", maxTokensHardCap, req.MaxTokens)
	}
	if req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != maxTokensHardCap {
		t.Fatalf("MaxCompletionTokens must be clamped to %d; got %v", maxTokensHardCap, req.MaxCompletionTokens)
	}
}

func TestApplyAirbotixPolicyToClaude_ClampsMaxTokens(t *testing.T) {
	over := uint(9999)
	c := newTestContext(t, nil)
	req := &dto.ClaudeRequest{
		Model:             "claude-3-5-haiku-latest",
		MaxTokens:         &over,
		MaxTokensToSample: &over,
	}
	if err := applyAirbotixPolicyToClaude(c, req); err != nil {
		t.Fatalf("unexpected error %v", err.Err)
	}
	if req.MaxTokens == nil || *req.MaxTokens != maxTokensHardCap {
		t.Fatalf("MaxTokens must be clamped to %d; got %v", maxTokensHardCap, req.MaxTokens)
	}
	if req.MaxTokensToSample == nil || *req.MaxTokensToSample != maxTokensHardCap {
		t.Fatalf("MaxTokensToSample must be clamped to %d; got %v", maxTokensHardCap, req.MaxTokensToSample)
	}
}

func TestApplyAirbotixPolicyToResponses_ClampsMaxOutputTokens(t *testing.T) {
	over := uint(9999)
	c := newTestContext(t, nil)
	req := &dto.OpenAIResponsesRequest{
		Model:           "gpt-4o-mini",
		MaxOutputTokens: &over,
	}
	if err := applyAirbotixPolicyToResponses(c, constant.ChannelTypeOpenAI, req); err != nil {
		t.Fatalf("unexpected error %v", err.Err)
	}
	if req.MaxOutputTokens == nil || *req.MaxOutputTokens != maxTokensHardCap {
		t.Fatalf("MaxOutputTokens must be clamped to %d; got %v", maxTokensHardCap, req.MaxOutputTokens)
	}
}

func TestApplyAirbotixPolicyToGemini_ClampsMaxOutputTokens(t *testing.T) {
	over := uint(9999)
	c := newTestContext(t, nil)
	req := &dto.GeminiChatRequest{}
	req.GenerationConfig.MaxOutputTokens = &over
	if err := applyAirbotixPolicyToGemini(c, "gpt-4o-mini", req); err != nil {
		t.Fatalf("unexpected error %v", err.Err)
	}
	if req.GenerationConfig.MaxOutputTokens == nil || *req.GenerationConfig.MaxOutputTokens != maxTokensHardCap {
		t.Fatalf("MaxOutputTokens must be clamped to %d; got %v", maxTokensHardCap, req.GenerationConfig.MaxOutputTokens)
	}
}

// =============================================================================
// outputFilterWriter / wrapOutputFilterWriter — response-side output filter
// =============================================================================

// newOutputFilterTestContext returns a *gin.Context backed by an
// *httptest.ResponseRecorder, so tests can inspect the bytes/headers/status
// that actually reach the "client".
func newOutputFilterTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// -----------------------------------------------------------------------
// Non-stream
// -----------------------------------------------------------------------

func TestOutputFilterWriter_NonStream_PassesCleanResponse(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	body := `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"Hello there"},"finish_reason":"stop"}]}`
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString(body)
	restore()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != body {
		t.Errorf("clean response should pass through unchanged; got %s", rec.Body.String())
	}
	if _, ok := common.GetContextKey(c, constant.ContextKeyOutputFilterViolations); ok {
		t.Error("ContextKeyOutputFilterViolations must not be set for a clean response")
	}
}

func TestOutputFilterWriter_NonStream_PassesKnownNonTextResponse(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	// pure tool_calls response: ExtractText returns ok=true, text="" — a
	// recognised clean case, must NOT fail closed.
	body := `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"f","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString(body)
	restore()

	if rec.Body.String() != body {
		t.Errorf("tool_calls-only response should pass through unchanged; got %s", rec.Body.String())
	}
}

func TestOutputFilterWriter_NonStream_ReplacesBlockedContent(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeClaudeMessages)
	defer restore()

	body := `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"I will murder you"}],"stop_reason":"end_turn"}`
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString(body)
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeClaudeMessages).ExtractText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractText(replaced) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("blocked content should be replaced with SafeFallbackText; got %q", text)
	}
	if strings.Contains(rec.Body.String(), "murder") {
		t.Errorf("replaced body must not contain the blocked text: %s", rec.Body.String())
	}
}

func TestOutputFilterWriter_NonStream_FailsClosedOnUnparseableBody(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeOpenAIResponses)
	defer restore()

	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString("not json")
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeOpenAIResponses).ExtractText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("unparseable body must fail closed to SafeFallbackText; got %q", text)
	}
}

func TestOutputFilterWriter_NonStream_PassthroughOnNon2xxErrorBody(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeGemini)
	defer restore()

	body := `{"error":{"code":400,"message":"bad request"}}`
	c.Writer.WriteHeader(http.StatusBadRequest)
	c.Writer.WriteString(body)
	restore()

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec.Body.String() != body {
		t.Errorf("non-2xx error body should pass through unchanged; got %s", rec.Body.String())
	}
}

func TestOutputFilterWriter_NonStream_PassthroughWhenDecisionFalse(t *testing.T) {
	d := passthroughDecision()
	c, rec := newOutputFilterTestContext(t)
	orig := c.Writer

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	if c.Writer != orig {
		t.Fatal("wrapOutputFilterWriter must not wrap c.Writer when EnforceStrictOutputFilter=false")
	}

	body := `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"I will murder you"},"finish_reason":"stop"}]}`
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString(body)
	restore()

	if rec.Body.String() != body {
		t.Errorf("response must pass through unchanged when EnforceStrictOutputFilter=false; got %s", rec.Body.String())
	}
}

// -----------------------------------------------------------------------
// Stream
// -----------------------------------------------------------------------

func TestOutputFilterWriter_Stream_PassesCleanStream(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeClaudeMessages)
	defer restore()

	raw := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\"}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n" +
		"event: content_block_stop\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString(raw)
	restore()

	if rec.Body.String() != raw {
		t.Errorf("clean stream should pass through unchanged; got %q", rec.Body.String())
	}
	if rec.Header().Get("Content-Length") != "" {
		t.Errorf("stream responses must not get a Content-Length header; got %q", rec.Header().Get("Content-Length"))
	}
}

func TestOutputFilterWriter_Stream_RewritesBlockedStreamBeforeAnyByteSent(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeOpenAIResponses)
	defer restore()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString("event: response.output_text.delta\n")
	c.Writer.WriteString("data: {\"type\":\"response.output_text.delta\",\"item_id\":\"item_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"I will \"}\n\n")
	c.Writer.WriteString("event: response.output_text.delta\n")
	c.Writer.WriteString("data: {\"type\":\"response.output_text.delta\",\"item_id\":\"item_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"murder you\"}\n\n")
	c.Writer.WriteString("event: response.completed\n")
	c.Writer.WriteString("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"I will murder you\",\"annotations\":[]}]}]}}\n\n")

	if rec.Body.Len() != 0 {
		t.Fatalf("no bytes should reach the client before restore(); got %d bytes: %q", rec.Body.Len(), rec.Body.String())
	}

	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeOpenAIResponses).ExtractStreamText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractStreamText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("blocked stream should be replaced with SafeFallbackText; got %q", text)
	}
	violations, ok := common.GetContextKey(c, constant.ContextKeyOutputFilterViolations)
	if !ok {
		t.Fatal("ContextKeyOutputFilterViolations must be set when the stream is blocked")
	}
	if cats, ok := violations.([]string); !ok || len(cats) == 0 {
		t.Errorf("ContextKeyOutputFilterViolations = %v, want a non-empty []string", violations)
	}
}

func TestOutputFilterWriter_Stream_AccumulatesAcrossChunkBoundary(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeGemini)
	defer restore()

	raw := "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hel\"}]},\"index\":0}]}\n\n" +
		"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"lo\"}]},\"finishReason\":\"STOP\",\"index\":0}]}\n\n"

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	// Split mid-line (in the middle of "finishReason") across two Write()
	// calls, proving the writer reassembles bytes before classifying.
	mid := len(raw) - 20
	if _, err := c.Writer.Write([]byte(raw[:mid])); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := c.Writer.Write([]byte(raw[mid:])); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	restore()

	if rec.Body.String() != raw {
		t.Errorf("clean stream split across Write() calls should pass through unchanged; got %q", rec.Body.String())
	}
}

func TestOutputFilterWriter_Stream_FailsClosedOnMalformedSSE(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString("this is not a valid SSE stream at all")
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeChatCompletions).ExtractStreamText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractStreamText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("malformed SSE must fail closed to SafeFallbackText; got %q", text)
	}
}

// TestOutputFilterWriter_Stream_FailsClosedOnMixedValidAndMalformedChunks_*
// cover the "partially parseable" SSE case: a leading chunk that is valid
// and matches the shape, followed by one data: chunk that doesn't parse at
// all. ExtractStreamText must fail closed for the WHOLE stream (not just
// silently drop the bad chunk and pass the earlier valid text through), so
// finalize() must still replace the stream with BuildFallbackStream(...).

func TestOutputFilterWriter_Stream_FailsClosedOnMixedValidAndMalformedChunks_Chat(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString("data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"}}]}\n\n")
	c.Writer.WriteString("data: not valid json\n\n")
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeChatCompletions).ExtractStreamText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractStreamText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("a stream with one valid chunk followed by one malformed data: chunk must fail closed to SafeFallbackText; got %q", text)
	}
}

func TestOutputFilterWriter_Stream_FailsClosedOnMixedValidAndMalformedChunks_Claude(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeClaudeMessages)
	defer restore()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString("event: content_block_delta\n")
	c.Writer.WriteString("data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n")
	c.Writer.WriteString("data: not valid json\n\n")
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeClaudeMessages).ExtractStreamText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractStreamText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("a stream with one valid chunk followed by one malformed data: chunk must fail closed to SafeFallbackText; got %q", text)
	}
}

func TestOutputFilterWriter_Stream_FailsClosedOnMixedValidAndMalformedChunks_Responses(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeOpenAIResponses)
	defer restore()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString("event: response.output_text.delta\n")
	c.Writer.WriteString("data: {\"type\":\"response.output_text.delta\",\"item_id\":\"item_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"Hello\"}\n\n")
	c.Writer.WriteString("data: not valid json\n\n")
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeOpenAIResponses).ExtractStreamText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractStreamText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("a stream with one valid chunk followed by one malformed data: chunk must fail closed to SafeFallbackText; got %q", text)
	}
}

func TestOutputFilterWriter_Stream_FailsClosedOnMixedValidAndMalformedChunks_Gemini(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeGemini)
	defer restore()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString("data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hello\"}]},\"index\":0}]}\n\n")
	c.Writer.WriteString("data: not valid json\n\n")
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeGemini).ExtractStreamText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractStreamText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("a stream with one valid chunk followed by one malformed data: chunk must fail closed to SafeFallbackText; got %q", text)
	}
}

// -----------------------------------------------------------------------
// Writer behaviour
// -----------------------------------------------------------------------

func TestOutputFilterWriter_FlushBeforeFinalize_NoBytesWritten(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	body := `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"Hello there"},"finish_reason":"stop"}]}`
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString(body)
	c.Writer.Flush() // mirrors service.IOCopyBytesGracefully's unconditional Flush()

	if rec.Body.Len() != 0 {
		t.Fatalf("Flush() before finalize() must not write any bytes; got %d", rec.Body.Len())
	}

	restore()
	if rec.Body.String() != body {
		t.Errorf("body should be delivered once restore() runs; got %s", rec.Body.String())
	}
}

func TestOutputFilterWriter_Finalize_Idempotent(t *testing.T) {
	c, rec := newOutputFilterTestContext(t)

	w := &outputFilterWriter{ResponseWriter: c.Writer, c: c, shape: kids.ResponseShapeChatCompletions, filter: outputFilter}

	body := `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"Hello there"},"finish_reason":"stop"}]}`
	w.WriteHeader(http.StatusOK)
	w.WriteString(body)

	w.finalize()
	first := rec.Body.String()
	w.finalize() // must be a no-op

	if rec.Body.String() != first {
		t.Errorf("finalize() must be idempotent; body changed from %q to %q", first, rec.Body.String())
	}
}

func TestOutputFilterWriter_ExplicitRestoreThenDeferredRestore_ContextKeyAndBytesReadyImmediately(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeClaudeMessages)
	defer restore()

	body := `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"I will murder you"}],"stop_reason":"end_turn"}`
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString(body)

	restore() // explicit restore — bytes + context key must be ready immediately after this returns

	if rec.Body.Len() == 0 {
		t.Fatal("expected the fallback body to be written immediately after the explicit restore()")
	}
	if _, ok := common.GetContextKey(c, constant.ContextKeyOutputFilterViolations); !ok {
		t.Fatal("ContextKeyOutputFilterViolations must be readable immediately after the explicit restore()")
	}
	if _, wrapped := c.Writer.(*outputFilterWriter); wrapped {
		t.Error("c.Writer should be restored to the original writer after the explicit restore()")
	}
	bodyAfterExplicit := rec.Body.String()

	restore() // deferred restore() — must be a guaranteed no-op
	if rec.Body.String() != bodyAfterExplicit {
		t.Error("the deferred restore() after an explicit restore() must not change the response")
	}
}

func TestOutputFilterWriter_FinalizeBeforeAnyWrite_NoOutput(t *testing.T) {
	c, rec := newOutputFilterTestContext(t)

	w := &outputFilterWriter{ResponseWriter: c.Writer, c: c, shape: kids.ResponseShapeChatCompletions, filter: outputFilter}
	w.finalize()

	if rec.Body.Len() != 0 {
		t.Errorf("finalize() before any write must produce no body; got %d bytes", rec.Body.Len())
	}
	if rec.Header().Get("Content-Length") != "" {
		t.Errorf("finalize() before any write must not set Content-Length; got %q", rec.Header().Get("Content-Length"))
	}
}

func TestOutputFilterWriter_ResponseProducingCallErrorsBeforeWrite_NoSpuriousFallback(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore() // safety net for the case where nothing below ever runs

	// Simulate: the response-producing call returned an error before writing
	// anything. The explicit restore() must be a no-op (wrote == false), so
	// the handler's own error response below reaches the client untouched.
	restore()

	errBody := `{"error":"upstream timeout"}`
	c.Writer.WriteHeader(http.StatusBadGateway)
	c.Writer.WriteString(errBody)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if rec.Body.String() != errBody {
		t.Errorf("handler's own error body must reach the client unchanged; got %q", rec.Body.String())
	}
}

func TestOutputFilterWriter_EmptyBodyStatus_NoFallback(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	c.Writer.WriteHeader(http.StatusNoContent)
	restore()

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("empty body must not get a fallback body; got %d bytes: %s", rec.Body.Len(), rec.Body.String())
	}
}

func TestOutputFilterWriter_BufferOverflow_NonStream2xx_FailsClosed(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	c.Writer.WriteHeader(http.StatusOK)
	chunk := make([]byte, 1<<16)
	for i := 0; i < (maxOutputFilterBufferBytes/len(chunk))+1; i++ {
		if _, err := c.Writer.Write(chunk); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeChatCompletions).ExtractText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("overflowed 2xx response must fail closed to SafeFallbackText; got %q", text)
	}
	overflow, ok := common.GetContextKey(c, constant.ContextKeyOutputFilterBufferOverflow)
	if !ok || overflow != true {
		t.Errorf("ContextKeyOutputFilterBufferOverflow must be set to true; got %v (ok=%v)", overflow, ok)
	}
}

func TestOutputFilterWriter_BufferOverflow_Stream2xx_FailsClosed(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeClaudeMessages)
	defer restore()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	chunk := make([]byte, 1<<16)
	for i := 0; i < (maxOutputFilterBufferBytes/len(chunk))+1; i++ {
		if _, err := c.Writer.Write(chunk); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	restore()

	text, ok := kids.FilterForShape(kids.ResponseShapeClaudeMessages).ExtractStreamText(rec.Body.Bytes())
	if !ok {
		t.Fatalf("ExtractStreamText(fallback) ok = false")
	}
	if text != kids.SafeFallbackText() {
		t.Errorf("overflowed 2xx stream must fail closed to SafeFallbackText; got %q", text)
	}
	if rec.Header().Get("Content-Length") != "" {
		t.Errorf("stream fallback must not set Content-Length; got %q", rec.Header().Get("Content-Length"))
	}
	overflow, ok := common.GetContextKey(c, constant.ContextKeyOutputFilterBufferOverflow)
	if !ok || overflow != true {
		t.Errorf("ContextKeyOutputFilterBufferOverflow must be set to true; got %v (ok=%v)", overflow, ok)
	}
}

func TestOutputFilterWriter_BufferOverflow_Non2xx_PassthroughTruncated(t *testing.T) {
	d := kidsModeDecision()
	c, rec := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	c.Writer.WriteHeader(http.StatusInternalServerError)
	chunk := make([]byte, 1<<16)
	for i := 0; i < (maxOutputFilterBufferBytes/len(chunk))+1; i++ {
		if _, err := c.Writer.Write(chunk); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	restore()

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if rec.Body.Len() != maxOutputFilterBufferBytes {
		t.Errorf("truncated non-2xx body length = %d, want %d", rec.Body.Len(), maxOutputFilterBufferBytes)
	}
	overflow, ok := common.GetContextKey(c, constant.ContextKeyOutputFilterBufferOverflow)
	if !ok || overflow != true {
		t.Errorf("ContextKeyOutputFilterBufferOverflow must be set to true; got %v (ok=%v)", overflow, ok)
	}
}

func TestOutputFilterWriter_BufferOverflow_DoesNotGrowBeyondCap(t *testing.T) {
	c, _ := newOutputFilterTestContext(t)

	w := &outputFilterWriter{ResponseWriter: c.Writer, c: c, shape: kids.ResponseShapeChatCompletions, filter: outputFilter}

	chunk := make([]byte, 1<<16)
	for i := 0; i < (maxOutputFilterBufferBytes/len(chunk))+2; i++ {
		n, err := w.Write(chunk)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if n != len(chunk) {
			t.Fatalf("Write() returned n=%d, want %d (must report success even when discarding overflow)", n, len(chunk))
		}
	}

	if w.buf.Len() != maxOutputFilterBufferBytes {
		t.Errorf("buf.Len() = %d, want %d (must not grow beyond the cap)", w.buf.Len(), maxOutputFilterBufferBytes)
	}
	if !w.overflow {
		t.Error("overflow flag must be set once the cap is exceeded")
	}
}

// -----------------------------------------------------------------------
// Extra
// -----------------------------------------------------------------------

func TestOutputFilterWriter_ContextKeyOutputFilterViolations_SetOnBlock(t *testing.T) {
	d := kidsModeDecision()
	c, _ := newOutputFilterTestContext(t)

	restore := wrapOutputFilterWriter(c, d, kids.ResponseShapeChatCompletions)
	defer restore()

	body := `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"You subhuman, I will murder you"},"finish_reason":"stop"}]}`
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.WriteString(body)
	restore()

	raw, ok := common.GetContextKey(c, constant.ContextKeyOutputFilterViolations)
	if !ok {
		t.Fatal("ContextKeyOutputFilterViolations must be set when output is blocked")
	}
	categories, ok := raw.([]string)
	if !ok {
		t.Fatalf("ContextKeyOutputFilterViolations type = %T, want []string", raw)
	}
	want := []string{kids.OutputCategoryViolence, kids.OutputCategoryHate}
	if len(categories) != len(want) {
		t.Fatalf("categories = %v, want %v", categories, want)
	}
	for i := range want {
		if categories[i] != want[i] {
			t.Fatalf("categories = %v, want %v", categories, want)
		}
	}
}

// -----------------------------------------------------------------------
// Multi-choice / multi-candidate (§5.2.1)
// -----------------------------------------------------------------------

// TestOutputFilterWriter_NonStream_MultiChoiceSecondEntryBlocked_ReplacesWithSingleFallback
// covers design doc §5.2.1: a 2xx response with 2 choices/candidates where
// only the second is blocked must still be blocked overall (ExtractText
// concatenates all entries), and the fallback body must contain exactly ONE
// choice/candidate at index 0 — the original Choices[1]/Candidates[1] must
// not survive in the output.
func TestOutputFilterWriter_NonStream_MultiChoiceSecondEntryBlocked_ReplacesWithSingleFallback(t *testing.T) {
	cases := []struct {
		name  string
		shape kids.ResponseShape
		body  string
	}{
		{
			name:  "Chat",
			shape: kids.ResponseShapeChatCompletions,
			body: `{"id":"chatcmpl-1","object":"chat.completion","choices":[` +
				`{"index":0,"message":{"role":"assistant","content":"Hello there"},"finish_reason":"stop"},` +
				`{"index":1,"message":{"role":"assistant","content":"I will murder you"},"finish_reason":"stop"}` +
				`]}`,
		},
		{
			name:  "Gemini",
			shape: kids.ResponseShapeGemini,
			body: `{"candidates":[` +
				`{"content":{"role":"model","parts":[{"text":"Hello there"}]},"finishReason":"STOP","index":0},` +
				`{"content":{"role":"model","parts":[{"text":"I will murder you"}]},"finishReason":"STOP","index":1}` +
				`]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := kidsModeDecision()
			c, rec := newOutputFilterTestContext(t)

			restore := wrapOutputFilterWriter(c, d, tc.shape)
			defer restore()

			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.WriteString(tc.body)
			restore()

			sf := kids.FilterForShape(tc.shape)
			text, ok := sf.ExtractText(rec.Body.Bytes())
			if !ok {
				t.Fatalf("ExtractText(replaced) ok = false")
			}
			if text != kids.SafeFallbackText() {
				t.Errorf("blocked response should be replaced with SafeFallbackText; got %q", text)
			}
			if strings.Contains(rec.Body.String(), "murder") {
				t.Errorf("replaced body must not contain the original entry[1] blocked text: %s", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "Hello there") {
				t.Errorf("replaced body must not contain the original entry[0] text: %s", rec.Body.String())
			}

			switch tc.shape {
			case kids.ResponseShapeChatCompletions:
				var resp dto.OpenAITextResponse
				if err := common.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Unmarshal(fallback body) error = %v", err)
				}
				if len(resp.Choices) != 1 {
					t.Fatalf("len(Choices) = %d, want 1", len(resp.Choices))
				}
				if resp.Choices[0].Index != 0 {
					t.Errorf("Choices[0].Index = %d, want 0", resp.Choices[0].Index)
				}
			case kids.ResponseShapeGemini:
				var resp dto.GeminiChatResponse
				if err := common.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Unmarshal(fallback body) error = %v", err)
				}
				if len(resp.Candidates) != 1 {
					t.Fatalf("len(Candidates) = %d, want 1", len(resp.Candidates))
				}
				if resp.Candidates[0].Index != 0 {
					t.Errorf("Candidates[0].Index = %d, want 0", resp.Candidates[0].Index)
				}
			}

			if got, want := rec.Header().Get("Content-Length"), strconv.Itoa(rec.Body.Len()); got != want {
				t.Errorf("Content-Length = %q, want %q (len of replaced body)", got, want)
			}
		})
	}
}

// TestOutputFilterWriter_Stream_MultiChoiceSecondEntryBlocked_ReplacesWithSingleFallback
// is the streaming counterpart of the test above: the blocklist keyword is
// only delivered on choice/candidate index 1's delta. The client must
// receive zero bytes before restore(), and the final stream must be
// BuildFallbackStream's single fallback choice/candidate at index 0 — index
// 1 must not appear in the output.
func TestOutputFilterWriter_Stream_MultiChoiceSecondEntryBlocked_ReplacesWithSingleFallback(t *testing.T) {
	cases := []struct {
		name  string
		shape kids.ResponseShape
		raw   string
	}{
		{
			name:  "Chat",
			shape: kids.ResponseShapeChatCompletions,
			raw: "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"}}]}\n\n" +
				"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":1,\"delta\":{\"role\":\"assistant\",\"content\":\"I will murder you\"}}]}\n\n" +
				"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"},{\"index\":1,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
				"data: [DONE]\n\n",
		},
		{
			name:  "Gemini",
			shape: kids.ResponseShapeGemini,
			raw: "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hello\"}]},\"index\":0}]}\n\n" +
				"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"I will murder you\"}]},\"index\":1}]}\n\n" +
				"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[]},\"finishReason\":\"STOP\",\"index\":0},{\"content\":{\"role\":\"model\",\"parts\":[]},\"finishReason\":\"STOP\",\"index\":1}]}\n\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := kidsModeDecision()
			c, rec := newOutputFilterTestContext(t)

			restore := wrapOutputFilterWriter(c, d, tc.shape)
			defer restore()

			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.WriteString(tc.raw)

			if rec.Body.Len() != 0 {
				t.Fatalf("no bytes should reach the client before restore(); got %d bytes: %q", rec.Body.Len(), rec.Body.String())
			}

			restore()

			sf := kids.FilterForShape(tc.shape)
			text, ok := sf.ExtractStreamText(rec.Body.Bytes())
			if !ok {
				t.Fatalf("ExtractStreamText(fallback) ok = false")
			}
			if text != kids.SafeFallbackText() {
				t.Errorf("blocked stream should be replaced with SafeFallbackText; got %q", text)
			}
			if strings.Contains(rec.Body.String(), "murder") {
				t.Errorf("fallback stream must not contain the original index 1 blocked text: %s", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), `"index":1`) {
				t.Errorf("fallback stream must not contain an index 1 entry: %s", rec.Body.String())
			}
		})
	}
}
