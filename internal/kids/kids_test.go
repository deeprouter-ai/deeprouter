package kids

import "testing"

func TestIsModelEligible(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"gpt-4o-mini", true},
		{"claude-3-5-sonnet-latest", true},
		{"claude-3-5-sonnet-20241022", true}, // versioned variant of whitelisted base
		{"gpt-3.5-turbo", false},             // not on whitelist
		{"some-uncensored-model-v2", false},
		{"", false},
		// Image models removed from the whitelist on 2026-06-15: kids_mode's
		// strict output filter only covers the 4 text response shapes, so no
		// image model is eligible until an image NSFW filter covers
		// /v1/images/generations and /v1/images/edits.
		{"gpt-image-2", false},
		{"gpt-image-2-2026-04-21", false}, // snapshot variant via HasPrefix
		{"gpt-image-1", false},
		{"flux-schnell", false},
		{"flux-1.1-pro", false},
		{"dall-e-3", false}, // retired by OpenAI on 2026-05-12
		{"dall-e-2", false}, // retired same date
	}
	for _, tc := range cases {
		if got := IsModelEligible(tc.model); got != tc.want {
			t.Errorf("IsModelEligible(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestStripIdentifyingMetadata(t *testing.T) {
	req := map[string]any{
		"model": "gpt-4o-mini",
		"user":  "kid-12345",
		"metadata": map[string]any{
			"user_id":        "u1",
			"kid_profile_id": "k1",
			"family_id":      "f1",
			"safe_field":     "keep-me",
		},
		"messages": []any{},
	}
	got := StripIdentifyingMetadata(req)
	if _, has := got["user"]; has {
		t.Errorf("expected 'user' removed, still present")
	}
	md, _ := got["metadata"].(map[string]any)
	if _, has := md["user_id"]; has {
		t.Errorf("expected metadata.user_id removed")
	}
	if _, has := md["kid_profile_id"]; has {
		t.Errorf("expected metadata.kid_profile_id removed")
	}
	if md["safe_field"] != "keep-me" {
		t.Errorf("expected safe_field preserved, got %v", md["safe_field"])
	}
}

func TestStripIdentifyingMetadata_DropsEmptyMetadata(t *testing.T) {
	req := map[string]any{
		"metadata": map[string]any{
			"user_id": "u1",
		},
	}
	got := StripIdentifyingMetadata(req)
	if _, has := got["metadata"]; has {
		t.Errorf("expected metadata removed when it becomes empty")
	}
}

func TestEnforceZeroDataRetention_OpenAI(t *testing.T) {
	req := map[string]any{"model": "gpt-4o-mini"}
	got := EnforceZeroDataRetention(req, "openai")
	if got["store"] != false {
		t.Errorf("expected store=false for openai, got %v", got["store"])
	}
}

func TestEnforceZeroDataRetention_NonOpenAI(t *testing.T) {
	req := map[string]any{"model": "claude-3-5-haiku-latest"}
	got := EnforceZeroDataRetention(req, "anthropic")
	if _, has := got["store"]; has {
		t.Errorf("expected store NOT set for non-openai, got %v", got["store"])
	}
}

func TestChildSafeSystemPrompt_Nonempty(t *testing.T) {
	if p := ChildSafeSystemPrompt(); len(p) < 50 {
		t.Errorf("system prompt too short, got %d chars", len(p))
	}
}
