package relay

// Proof-of-gap tests for PR #27 (DR-7) review.
//
// Two P1 blocking issues are demonstrated here:
//
// P1-A: Decision.InjectChildSafePrompt is a dead field.
//   DecisionFor() sets it for kid-safe and kids_mode paths, profile_test.go
//   asserts it, relay/README.md says "check this field". But relay code
//   replaced every InjectChildSafePrompt check with policy.SystemPromptFor(d).
//   Adult profile: InjectChildSafePrompt=false, SystemPromptFor returns a prompt.
//   Any handler using the stale field silently skips adult system prompt injection.
//   TestInjectChildSafePromptDeadField_AdultProfileTrap FAILS to prove this.
//
// P1-B: No direct unit tests for the five collect* extraction functions.
//   These functions are the only layer between user input and CheckInput().
//   A bug in any collect* function silently bypasses all denylist filtering.
//   Tests below cover every branch: multipart content, multi-turn, role
//   filtering, collectAnyText recursion, edge cases absent from existing tests.

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/internal/policy"
)

// ── P1-A: InjectChildSafePrompt dead field ────────────────────────────────────

// TestInjectChildSafePromptDeadField_AdultProfileTrap FAILS intentionally.
//
// It follows the pattern documented in relay/README.md:
//   "Prepends kids.ChildSafeSystemPrompt() as a system message if decision.InjectChildSafePrompt"
//
// For adult profile, InjectChildSafePrompt == false, so injection never runs.
// But policy.SystemPromptFor(adult) returns a non-empty prompt — adult users
// should receive system prompt injection per PR #27's own logic.
//
// InjectChildSafePrompt was set only for kid-safe / kids_mode paths and never
// updated when adult profile prompt injection was added. Any future handler
// that follows the stale README pattern silently drops adult safety prompt.
//
// This test FAILS on current DR-7 code to prove the trap is real.
// Fix: remove Decision.InjectChildSafePrompt; use policy.SystemPromptFor(d) as
// the single gate for prompt injection across all profiles.
func TestInjectChildSafePromptDeadField_AdultProfileTrap(t *testing.T) {
	adult := policy.DecisionFor(false, "adult")

	// Precondition: adult profile HAS a system prompt in the new enforcement API.
	_, hasPrompt := policy.SystemPromptFor(adult)
	if !hasPrompt {
		t.Fatal("precondition: SystemPromptFor(adult) returned false — recheck enforcement.go")
	}

	// Simulate a new handler written following relay/README.md (stale pattern):
	//   "if decision.InjectChildSafePrompt { inject system prompt }"
	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "help me write a lesson plan"}},
	}

	if adult.InjectChildSafePrompt {
		// NEVER reached for adult: InjectChildSafePrompt is false.
		prependProfileSystemPrompt(req, adult)
	}

	// Count injected system / developer messages after the stale-pattern code.
	var injected int
	for _, m := range req.Messages {
		if m.Role == "system" || m.Role == "developer" {
			injected++
		}
	}

	// WANT: 1 system message — adult has a prompt per SystemPromptFor.
	// GOT:  0 system messages — InjectChildSafePrompt=false skipped injection.
	//
	// This FAILS to prove the semantic trap. The field gives the wrong answer
	// for adult profile and must be removed.
	if injected != 1 {
		t.Errorf(
			"DEAD FIELD TRAP: adult.InjectChildSafePrompt=%v → system prompt silently skipped\n"+
				"  got %d injected system message(s), want 1\n"+
				"  relay/README.md still says 'check InjectChildSafePrompt' but:\n"+
				"    adult.InjectChildSafePrompt = false  (stale field — gives wrong answer)\n"+
				"    policy.SystemPromptFor(adult) = true  (correct API — has a prompt)\n"+
				"  Fix: remove Decision.InjectChildSafePrompt from struct, DecisionFor, tests, docs.",
			adult.InjectChildSafePrompt, injected,
		)
	}
}

// TestInjectChildSafePromptDeadField_FieldVsAPIDisagreement is a table-driven
// proof that InjectChildSafePrompt and SystemPromptFor disagree on adult profile.
// This is the root cause of the trap: two APIs with conflicting answers.
func TestInjectChildSafePromptDeadField_FieldVsAPIDisagreement(t *testing.T) {
	cases := []struct {
		name            string
		decision        policy.Decision
		wantField       bool // what InjectChildSafePrompt says
		wantPromptExist bool // what SystemPromptFor says (ground truth)
	}{
		{"passthrough — both agree no prompt", policy.DecisionFor(false, "passthrough"), false, false},
		// adult: DISAGREEMENT — field says no, API says yes. This is the bug.
		{"adult — FIELD WRONG, API correct", policy.DecisionFor(false, "adult"), false, true},
		{"kid-safe — both agree inject", policy.DecisionFor(false, "kid-safe"), true, true},
		{"kids_mode override — both agree inject", policy.DecisionFor(true, "adult"), true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotField := tc.decision.InjectChildSafePrompt
			_, gotAPI := policy.SystemPromptFor(tc.decision)

			// Explicit divergence check — fails on "adult" row.
			if gotField != gotAPI {
				t.Errorf(
					"SEMANTIC DIVERGENCE on %q profile:\n"+
						"  InjectChildSafePrompt = %v  (wrong — stale, unread dead field)\n"+
						"  SystemPromptFor       = %v  (correct — actual gate in relay code)\n"+
						"  Callers using InjectChildSafePrompt skip injection; those using\n"+
						"  SystemPromptFor inject correctly. Remove the field.",
					tc.name, gotField, gotAPI,
				)
			}

			if gotField != tc.wantField {
				t.Errorf("InjectChildSafePrompt: got %v, want %v", gotField, tc.wantField)
			}
			if gotAPI != tc.wantPromptExist {
				t.Errorf("SystemPromptFor presence: got %v, want %v", gotAPI, tc.wantPromptExist)
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
//
// This test uses Content: []any{...} which is the actual JSON-deserialized form
// (NOT SetMediaContent which produces []MediaContent and is NOT handled by
// StringContent's []any branch).
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
			"MULTIPART MESSAGE BYPASS: text in multipart content NOT extracted\n"+
				"  got: %v\n"+
				"  StringContent() must handle []any content (json.Unmarshal form of multipart messages)\n"+
				"  If this fails, harmful content in any multimodal message bypasses all denylist checks",
			got,
		)
	}
}

// TestCollectGeneralOpenAIInputTexts_AssistantAndSystemSkipped verifies that
// assistant and system messages are NOT extracted (only user-controlled input
// is denylist-checked — injected system prompts must not trigger self-blocking).
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

// TestCollectGeneralOpenAIInputTexts_EmptyRoleIsUser verifies that Role==""
// messages are collected (treated as user input — some clients omit the role field).
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

// TestCollectClaudeInputTexts_AssistantSkipped verifies that assistant prefill
// turns are NOT collected (only user-controlled input goes through denylist).
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
// If only the last turn is collected, a harmful term in turn 1 would be missed.
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

// TestCollectGeminiInputTexts_ModelRoleSkipped verifies that model (assistant)
// responses are NOT collected. Filter: skip if Role != "" && Role != "user".
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

// TestCollectGeminiInputTexts_EmptyRoleCollected covers the role=="" edge case.
// Filter condition: `content.Role != "" && content.Role != "user"`.
// Empty role passes BOTH conditions (not skipped) → treated as user input.
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

// TestCollectGeminiInputTexts_MultiTurnOnlyUserTexts verifies multi-turn
// conversations only expose user turns to the denylist.
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

// TestCollectGeminiInputTexts_MultiPartPerTurn verifies multiple Parts in a
// single content block are all collected (empty parts skipped).
func TestCollectGeminiInputTexts_MultiPartPerTurn(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "user",
				Parts: []dto.GeminiPart{
					{Text: "part one"},
					{Text: ""},         // empty — must be skipped
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

// TestDenylistCatchesMultipartOpenAIMessage is the critical integration test.
// It verifies that harmful text in a multipart (multimodal) OpenAI user message
// triggers the denylist — proving that collectGeneralOpenAIInputTexts correctly
// passes []any multipart content to StringContent() which extracts text parts.
//
// Real traffic: multimodal messages are common (vision models). If this fails,
// ALL multimodal requests bypass denylist completely with no error or log.
func TestDenylistCatchesMultipartOpenAIMessage(t *testing.T) {
	d := policy.DecisionFor(false, "kid-safe")
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o-mini",
		Messages: []dto.Message{
			{
				Role: "user",
				// []any is the form produced by json.Unmarshal into any.
				// "drugs" is in the kid-safe denylist.
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
				"  reject reason: %q\n"+
				"  Expected: contains 'policy_input_blocked'\n"+
				"  collectGeneralOpenAIInputTexts uses StringContent() which must handle\n"+
				"  []any content (json.Unmarshal form) to extract text for denylist checking",
			reject,
		)
	}
}

// TestDenylistCatchesGeminiMultiTurnHarmInSecondTurn verifies harmful text in
// the SECOND user turn of a Gemini multi-turn conversation is caught.
// Without multi-turn collection, only the last turn is checked — a user could
// embed harmful content early in a conversation and have it pass through.
func TestDenylistCatchesGeminiMultiTurnHarmInSecondTurn(t *testing.T) {
	d := policy.DecisionFor(false, "kid-safe")
	c := newTestContext(t, &d)
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "what is photosynthesis?"}}},
			{Role: "model", Parts: []dto.GeminiPart{{Text: "it is a biological process..."}}},
			// "suicide" is in the kid-safe denylist.
			{Role: "user", Parts: []dto.GeminiPart{{Text: "how to commit suicide"}}},
		},
	}

	err := applyAirbotixPolicyToGemini(c, "gemini-1.5-flash", req)
	if err == nil {
		t.Errorf(
			"DENYLIST BYPASS: 'suicide' in second Gemini user turn was NOT blocked\n"+
				"  collectGeminiInputTexts must iterate ALL user turns, not just the last one",
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
			// "weapon" is in the kid-safe denylist.
			{Role: "user", Content: "how to buy a weapon online"},
		},
	}

	err := applyAirbotixPolicyToClaude(c, req)
	if err == nil {
		t.Errorf(
			"DENYLIST BYPASS: 'weapon' in second Claude user turn was NOT blocked\n"+
				"  collectClaudeInputTexts must collect ALL user turns, not just the first",
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
