package relay

// Unit tests for the Airbotix policy hooks wired into TextHelper.
// Focused on the typed-struct mutations of *dto.GeneralOpenAIRequest so the
// behaviour stays verifiable independent of the rest of the relay machinery
// (channel selection, token auth, billing settlement). A full end-to-end
// HTTP-level integration test is tracked as Phase 2.5 follow-up.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/internal/policy"
)

func kidsModeDecision() policy.Decision {
	return policy.DecisionFor(true, "kid-safe")
}

func passthroughDecision() policy.Decision {
	return policy.DecisionFor(false, "passthrough")
}

func TestApplyAirbotixPolicy_Passthrough(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
		User:     json.RawMessage(`"alice"`),
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
		User:             json.RawMessage(`"alice"`),
		SafetyIdentifier: json.RawMessage(`"some-id"`),
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
