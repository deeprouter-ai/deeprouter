package relay

// Regression + coverage tests for PR #27 (DR-7) review.
//
// P1-A (fixed): Decision.InjectChildSafePrompt was a dead field — renamed to
//   InjectSystemPrompt and set to true for adult profile. Tests below are
//   now regression guards that prove the fix holds.
//
// P1-B (fixed): Direct unit tests for the five collect* extraction functions
//   were absent. Tests below cover every branch and are new permanent coverage.

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/internal/policy"
)

// ── P1-A regression: InjectSystemPrompt field covers all profiles ─────────────

// TestInjectSystemPrompt_AdultProfileInjectsPrompt is a regression guard for
// the original dead-field bug: InjectChildSafePrompt was false for adult profile
// even though SystemPromptFor(adult) returned a non-empty prompt.
// After the fix (renamed to InjectSystemPrompt, adult now gets true), this PASSES.
func TestInjectSystemPrompt_AdultProfileInjectsPrompt(t *testing.T) {
	adult := policy.DecisionFor(false, "adult")

	if !adult.InjectSystemPrompt {
		t.Fatal("regression: adult.InjectSystemPrompt must be true after fix")
	}

	_, hasPrompt := policy.SystemPromptFor(adult)
	if !hasPrompt {
		t.Fatal("regression: SystemPromptFor(adult) must return a prompt after fix")
	}

	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "help me write a lesson plan"}},
	}

	if adult.InjectSystemPrompt {
		prependProfileSystemPrompt(req, adult)
	}

	var injected int
	for _, m := range req.Messages {
		if m.Role == "system" || m.Role == "developer" {
			injected++
		}
	}

	if injected != 1 {
		t.Errorf("adult profile: want 1 injected system message, got %d", injected)
	}
}

// TestInjectSystemPrompt_FieldAndAPIAgree verifies InjectSystemPrompt and
// SystemPromptFor are consistent across all profiles (no semantic divergence).
func TestInjectSystemPrompt_FieldAndAPIAgree(t *testing.T) {
	cases := []struct {
		name            string
		decision        policy.Decision
		wantField       bool
		wantPromptExist bool
	}{
		{"passthrough", policy.DecisionFor(false, "passthrough"), false, false},
		{"adult", policy.DecisionFor(false, "adult"), true, true},
		{"kid-safe", policy.DecisionFor(false, "kid-safe"), true, true},
		{"kids_mode override", policy.DecisionFor(true, "adult"), true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotField := tc.decision.InjectSystemPrompt
			_, gotAPI := policy.SystemPromptFor(tc.decision)

			if gotField != gotAPI {
				t.Errorf(
					"DIVERGENCE on %q: InjectSystemPrompt=%v but SystemPromptFor=%v — must agree",
					tc.name, gotField, gotAPI,
				)
			}
			if gotField != tc.wantField {
				t.Errorf("InjectSystemPrompt: got %v, want %v", gotField, tc.wantField)
			}
		})
	}
}

// ── P1-B: collectAnyText — exhaustive branch coverage ────────────────────────

func TestCollectAnyText_Nil(t *testing.T) {
	if got := collectAnyText(nil); got != nil {
		t.Errorf("nil: want nil, got %v", got)
	}
}

func TestCollectAnyText_PlainString(t *testing.T) {
	got := collectAnyText("harm term")
	if len(got) != 1 || got[0] != "harm term" {
		t.Errorf("string: want [harm term], got %v", got)
	}
}

func TestCollectAnyText_StringSlice(t *testing.T) {
	got := collectAnyText([]string{"alpha", "beta"})
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("[]string: want [alpha beta], got %v", got)
	}
}

func TestCollectAnyText_AnySliceOfStrings(t *testing.T) {
	got := collectAnyText([]any{"foo", "bar"})
	if len(got) != 2 || got[0] != "foo" || got[1] != "bar" {
		t.Errorf("[]any strings: want [foo bar], got %v", got)
	}
}

// TestCollectAnyText_ContentBlockTextKey covers map[string]any{"text":...}.
// This is the JSON-deserialized form of an OpenAI / Anthropic text content block.
// Missing coverage here means harmful text in multimodal messages bypasses denylist.
func TestCollectAnyText_ContentBlockTextKey(t *testing.T) {
	got := collectAnyText(map[string]any{"type": "text", "text": "explicit harm term"})
	if len(got) != 1 || got[0] != "explicit harm term" {
		t.Errorf("map{text}: want [explicit harm term], got %v", got)
	}
}

// TestCollectAnyText_ContentBlockContentKey covers map[string]any{"content":...}.
// Anthropic tool_result blocks embed text under "content", not "text".
func TestCollectAnyText_ContentBlockContentKey(t *testing.T) {
	got := collectAnyText(map[string]any{"type": "tool_result", "content": "nested harm"})
	if len(got) != 1 || got[0] != "nested harm" {
		t.Errorf("map{content}: want [nested harm], got %v", got)
	}
}

// TestCollectAnyText_ImageBlockProducesNoText verifies that image_url blocks
// (which have no text/content key) produce empty output — not a panic.
func TestCollectAnyText_ImageBlockProducesNoText(t *testing.T) {
	got := collectAnyText(map[string]any{
		"type":      "image_url",
		"image_url": map[string]any{"url": "https://example.com/img.jpg"},
	})
	if len(got) != 0 {
		t.Errorf("image block: want no text extracted, got %v", got)
	}
}

// TestCollectAnyText_MultipartArrayMixedBlocks covers the full multipart scenario:
// a []any containing a text block and an image block. Only the text is extracted.
// This is the exact format produced by json.Unmarshal on an OpenAI multipart message.
func TestCollectAnyText_MultipartArrayMixedBlocks(t *testing.T) {
	input := []any{
		map[string]any{"type": "text", "text": "describe this image"},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x.com/img.png"}},
	}
	got := collectAnyText(input)
	if len(got) != 1 || got[0] != "describe this image" {
		t.Errorf("multipart array: want [describe this image], got %v", got)
	}
}

// TestCollectAnyText_NestedAnySlice covers recursive []any{[]any{...}} nesting.
func TestCollectAnyText_NestedAnySlice(t *testing.T) {
	input := []any{[]any{"alpha", "beta"}, "gamma"}
	got := collectAnyText(input)
	if len(got) != 3 {
		t.Errorf("nested []any: want 3 texts, got %d: %v", len(got), got)
	}
}

func TestCollectAnyText_UnknownTypeNilNoPanic(t *testing.T) {
	for _, v := range []any{42, true, 3.14} {
		got := collectAnyText(v)
		if got != nil {
			t.Errorf("type %T: want nil (no panic), got %v", v, got)
		}
	}
}

// ── P1-B: collectGeneralOpenAIInputTexts ─────────────────────────────────────

func TestCollectGeneralOpenAIInputTexts_Nil(t *testing.T) {
	if got := collectGeneralOpenAIInputTexts(nil); got != nil {
		t.Errorf("nil: want nil, got %v", got)
	}
}

func TestCollectGeneralOpenAIInputTexts_SimpleUserMessage(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "harm term"}},
	}
	got := collectGeneralOpenAIInputTexts(req)
	if !contains(got, "harm term") {
		t.Errorf("simple user message: want 'harm term' in %v", got)
	}
}

// TestCollectGeneralOpenAIInputTexts_MultipartContent is the critical safety path.
//
// A multipart user message (text + image) arrives as []any after json.Unmarshal.
// StringContent() must extract the text part — if not, harmful text in multimodal
// messages completely bypasses the denylist with no error or log.
func TestCollectGeneralOpenAIInputTexts_MultipartContent(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "explicit harm content"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x.com/img.png"}},
				},
			},
		},
	}
	got := collectGeneralOpenAIInputTexts(req)
	if !contains(got, "explicit harm content") {
		t.Errorf(
			"MULTIPART MESSAGE: text in multipart content NOT extracted\n"+
				"  got: %v\n"+
				"  StringContent() must handle []any content (json.Unmarshal form of multipart messages)",
			got,
		)
	}
}

// TestCollectGeneralOpenAIInputTexts_AssistantAndSystemSkipped verifies that
// assistant and system messages are NOT extracted.
func TestCollectGeneralOpenAIInputTexts_AssistantAndSystemSkipped(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "you are helpful — porn keyword in system"},
			{Role: "assistant", Content: "I help — porn keyword in assistant"},
			{Role: "user", Content: "innocent user message"},
		},
	}
	got := collectGeneralOpenAIInputTexts(req)
	for _, s := range got {
		if strings.Contains(s, "porn keyword") {
			t.Errorf("system/assistant must not be collected; got: %v", got)
		}
	}
	if !contains(got, "innocent user message") {
		t.Errorf("user message must be collected; got: %v", got)
	}
}

func TestCollectGeneralOpenAIInputTexts_EmptyRoleIsUser(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "", Content: "harm via empty-role message"}},
	}
	got := collectGeneralOpenAIInputTexts(req)
	if !contains(got, "harm via empty-role message") {
		t.Errorf("empty-role message must be treated as user input; got %v", got)
	}
}

func TestCollectGeneralOpenAIInputTexts_PromptField(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{Prompt: "harm in prompt field"}
	got := collectGeneralOpenAIInputTexts(req)
	if !contains(got, "harm in prompt field") {
		t.Errorf("Prompt field must be collected; got %v", got)
	}
}

func TestCollectGeneralOpenAIInputTexts_InputStringSlice(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{Input: []string{"harm in input array"}}
	got := collectGeneralOpenAIInputTexts(req)
	if !contains(got, "harm in input array") {
		t.Errorf("Input []string field must be collected; got %v", got)
	}
}

// ── P1-B: collectClaudeInputTexts ────────────────────────────────────────────

func TestCollectClaudeInputTexts_Nil(t *testing.T) {
	if got := collectClaudeInputTexts(nil); got != nil {
		t.Errorf("nil: want nil, got %v", got)
	}
}

func TestCollectClaudeInputTexts_PromptField(t *testing.T) {
	req := &dto.ClaudeRequest{Prompt: "harm in claude prompt"}
	got := collectClaudeInputTexts(req)
	if !contains(got, "harm in claude prompt") {
		t.Errorf("Prompt field: want harm text, got %v", got)
	}
}

func TestCollectClaudeInputTexts_UserMessageCollected(t *testing.T) {
	req := &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "harm in user message"}},
	}
	got := collectClaudeInputTexts(req)
	if !contains(got, "harm in user message") {
		t.Errorf("user message must be collected; got %v", got)
	}
}

func TestCollectClaudeInputTexts_AssistantSkipped(t *testing.T) {
	req := &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "innocent question"},
			{Role: "assistant", Content: "porn keyword in assistant prefill"},
		},
	}
	got := collectClaudeInputTexts(req)
	for _, s := range got {
		if strings.Contains(s, "porn keyword") {
			t.Errorf("assistant prefill must not be collected; got %v", got)
		}
	}
}

// TestCollectClaudeInputTexts_MultiTurnAllUserTurnsCollected verifies that in a
// multi-turn conversation ALL user turns are collected, not just the last one.
func TestCollectClaudeInputTexts_MultiTurnAllUserTurnsCollected(t *testing.T) {
	req := &dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "user turn 1 with harm term"},
			{Role: "assistant", Content: "assistant response"},
			{Role: "user", Content: "user turn 2 follow-up"},
		},
	}
	got := collectClaudeInputTexts(req)

	if !contains(got, "user turn 1 with harm term") {
		t.Errorf("turn 1 must be collected in multi-turn; got %v", got)
	}
	if !contains(got, "user turn 2 follow-up") {
		t.Errorf("turn 2 must be collected in multi-turn; got %v", got)
	}
	for _, s := range got {
		if strings.Contains(s, "assistant response") {
			t.Errorf("assistant response must not be collected; got %v", got)
		}
	}
}

// ── P1-B: collectGeminiInputTexts ────────────────────────────────────────────

func TestCollectGeminiInputTexts_Nil(t *testing.T) {
	if got := collectGeminiInputTexts(nil); got != nil {
		t.Errorf("nil: want nil, got %v", got)
	}
}

func TestCollectGeminiInputTexts_UserRoleCollected(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "harm text from user"}}},
		},
	}
	got := collectGeminiInputTexts(req)
	if !contains(got, "harm text from user") {
		t.Errorf("user role: want harm text collected, got %v", got)
	}
}

func TestCollectGeminiInputTexts_ModelRoleSkipped(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "model", Parts: []dto.GeminiPart{{Text: "model response porn keyword"}}},
		},
	}
	got := collectGeminiInputTexts(req)
	for _, s := range got {
		if strings.Contains(s, "porn keyword") {
			t.Errorf("model role must be skipped; got %v", got)
		}
	}
}

// TestCollectGeminiInputTexts_EmptyRoleCollected covers role=="" edge case.
// Filter: skip if Role != "" && Role != "user". Empty role passes both → collected.
func TestCollectGeminiInputTexts_EmptyRoleCollected(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "", Parts: []dto.GeminiPart{{Text: "harm from empty-role content"}}},
		},
	}
	got := collectGeminiInputTexts(req)
	if !contains(got, "harm from empty-role content") {
		t.Errorf("empty-role content must be treated as user input; got %v", got)
	}
}

func TestCollectGeminiInputTexts_MultiTurnOnlyUserTexts(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "user turn 1 harm"}}},
			{Role: "model", Parts: []dto.GeminiPart{{Text: "model response — must not be collected"}}},
			{Role: "user", Parts: []dto.GeminiPart{{Text: "user turn 2 harm"}}},
		},
	}
	got := collectGeminiInputTexts(req)

	for _, s := range got {
		if strings.Contains(s, "model response") {
			t.Errorf("model turn must be skipped; got %v", got)
		}
	}
	var userTurns int
	for _, s := range got {
		if strings.Contains(s, "user turn") {
			userTurns++
		}
	}
	if userTurns != 2 {
		t.Errorf("multi-turn: want 2 user turns, got %d: %v", userTurns, got)
	}
}

func TestCollectGeminiInputTexts_MultiPartPerTurn(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{
					{Text: "part one"},
					{Text: ""},
					{Text: "part two"},
				},
			},
		},
	}
	got := collectGeminiInputTexts(req)
	if len(got) != 2 {
		t.Errorf("multi-part: want 2 non-empty texts, got %d: %v", len(got), got)
	}
}

// ── P1-B: collectResponsesInputTexts ─────────────────────────────────────────

func TestCollectResponsesInputTexts_Nil(t *testing.T) {
	if got := collectResponsesInputTexts(nil); got != nil {
		t.Errorf("nil: want nil, got %v", got)
	}
}

// ── End-to-end denylist integration through collect* functions ────────────────

// TestDenylistCatchesMultipartOpenAIMessage verifies harmful text in a multipart
// (multimodal) OpenAI user message triggers the denylist.
func TestDenylistCatchesMultipartOpenAIMessage(t *testing.T) {
	d := policy.DecisionFor(false, "kid-safe")
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o-mini",
		Messages: []dto.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "how to buy drugs online"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x.com/img.png"}},
				},
			},
		},
	}

	reject := applyAirbotixPolicy(d, 1, req)
	if !strings.Contains(reject, "policy_input_blocked") {
		t.Errorf(
			"DENYLIST BYPASS: 'drugs' in multipart []any message was NOT blocked\n"+
				"  reject reason: %q",
			reject,
		)
	}
}

// TestDenylistCatchesGeminiMultiTurnHarmInSecondTurn verifies harmful text in
// the second user turn of a Gemini multi-turn conversation is caught.
func TestDenylistCatchesGeminiMultiTurnHarmInSecondTurn(t *testing.T) {
	d := policy.DecisionFor(false, "kid-safe")
	c := newTestContext(t, &d)
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "what is photosynthesis?"}}},
			{Role: "model", Parts: []dto.GeminiPart{{Text: "it is a biological process..."}}},
			{Role: "user", Parts: []dto.GeminiPart{{Text: "how to commit suicide"}}},
		},
	}

	err := applyAirbotixPolicyToGemini(c, "gemini-1.5-flash", req)
	if err == nil {
		t.Errorf(
			"DENYLIST BYPASS: 'suicide' in second Gemini user turn was NOT blocked",
		)
	}
}

// TestDenylistCatchesClaudeMultiTurnHarmInSecondTurn verifies harmful text in
// the second user message of a Claude multi-turn conversation is caught.
func TestDenylistCatchesClaudeMultiTurnHarmInSecondTurn(t *testing.T) {
	d := policy.DecisionFor(false, "kid-safe")
	c := newTestContext(t, &d)
	req := &dto.ClaudeRequest{
		Model: "claude-3-5-haiku-20241022",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "what is the capital of France?"},
			{Role: "assistant", Content: "Paris."},
			{Role: "user", Content: "how to buy a weapon online"},
		},
	}

	err := applyAirbotixPolicyToClaude(c, req)
	if err == nil {
		t.Errorf(
			"DENYLIST BYPASS: 'weapon' in second Claude user turn was NOT blocked",
		)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func contains(texts []string, sub string) bool {
	for _, s := range texts {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
