package alias_setting

import (
	"strings"
	"testing"
)

func mustInit(t *testing.T) {
	t.Helper()
	if err := InitAliasSettings(); err != nil {
		t.Fatalf("InitAliasSettings failed: %v", err)
	}
}

func TestInitParsesSeedYAML(t *testing.T) {
	mustInit(t)

	if len(GetPurposeSummary("en")) != 6 {
		t.Fatalf("expected 6 purpose cards, got %d", len(GetPurposeSummary("en")))
	}
	if len(GetPriceTierSummary("en")) != 4 {
		t.Fatalf("expected 4 price tiers, got %d", len(GetPriceTierSummary("en")))
	}
	if DefaultPriceTierID() != "standard" {
		t.Fatalf("expected default tier 'standard', got %q", DefaultPriceTierID())
	}
}

func TestIsVirtualModel(t *testing.T) {
	mustInit(t)

	cases := map[string]bool{
		"deeprouter":           true,
		"deeprouter-coding":    true,
		"deeprouter-voice-tts": true,
		"gpt-4o":               false,
		"claude-sonnet-4-7":    false,
		"":                     false,
	}
	for model, want := range cases {
		if got := IsVirtualModel(model); got != want {
			t.Errorf("IsVirtualModel(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestResolveAliasFallback(t *testing.T) {
	mustInit(t)

	// Direct (purpose, brand) hit.
	if got := ResolveAlias("coding", "openai"); got != "gpt-4o" {
		t.Errorf("coding+openai → %q, want gpt-4o", got)
	}
	// Brand missing → fall back to auto.
	if got := ResolveAlias("coding", "gemini"); got != "claude-sonnet-4-7" {
		t.Errorf("coding+gemini → %q, want auto fallback claude-sonnet-4-7", got)
	}
	// Purpose entirely missing → empty.
	if got := ResolveAlias("nonsense", "claude"); got != "" {
		t.Errorf("nonsense+claude → %q, want empty", got)
	}
	// Empty brand → auto.
	if got := ResolveAlias("chat", ""); got != "claude-sonnet-4-7" {
		t.Errorf("chat+empty → %q, want claude-sonnet-4-7", got)
	}
}

func TestResolveAliasForVirtualModelOverridesPurpose(t *testing.T) {
	mustInit(t)

	// Token bound to chat, but client asks for coding via virtual model name.
	if got := ResolveAliasForVirtualModel("deeprouter-coding", "chat", "openai"); got != "gpt-4o" {
		t.Errorf("deeprouter-coding under chat token → %q, want gpt-4o", got)
	}
	// Plain "deeprouter" honours the token's bound purpose.
	if got := ResolveAliasForVirtualModel("deeprouter", "chat", "claude"); got != "claude-sonnet-4-7" {
		t.Errorf("deeprouter under chat+claude → %q, want claude-sonnet-4-7", got)
	}
	// purpose=all has no alias binding — must return empty so distributor
	// leaves the client-supplied model name alone.
	if got := ResolveAliasForVirtualModel("deeprouter", "all", ""); got != "" {
		t.Errorf("deeprouter under all → %q, want empty (no alias)", got)
	}
}

func TestModelWhitelistForToken(t *testing.T) {
	mustInit(t)

	// Coding purpose → coding whitelist + virtual models tacked on.
	list, ok := ModelWhitelistForToken("coding", "", "")
	if !ok {
		t.Fatal("expected coding whitelist to be non-empty")
	}
	if !containsPattern(list, "claude-sonnet-*") {
		t.Errorf("coding whitelist missing claude-sonnet-*: %v", list)
	}
	if !containsPattern(list, "deeprouter") {
		t.Errorf("coding whitelist must include 'deeprouter' virtual alias so clients can call it")
	}

	// Auto + standard tier → tier whitelist.
	list, ok = ModelWhitelistForToken("all", "", "standard")
	if !ok {
		t.Fatal("expected standard-tier whitelist to be non-empty")
	}
	if containsPattern(list, "claude-opus-*") {
		t.Errorf("standard tier must NOT include Opus models, got: %v", list)
	}
	if !containsPattern(list, "gpt-4o*") {
		t.Errorf("standard tier should include gpt-4o*, got: %v", list)
	}

	// Auto + ultra tier → unlimited (empty whitelist).
	if _, ok := ModelWhitelistForToken("all", "", "ultra"); ok {
		t.Errorf("ultra tier should return ok=false (no model_limits restriction)")
	}

	// Auto + missing tier → falls through to default (standard).
	list, ok = ModelWhitelistForToken("all", "", "")
	if !ok {
		t.Fatal("expected default-tier whitelist to be non-empty")
	}
	if containsPattern(list, "claude-opus-*") {
		t.Errorf("default (standard) tier must NOT include Opus, got: %v", list)
	}
}

func TestGetPurposeSummaryLocalizes(t *testing.T) {
	mustInit(t)

	en := GetPurposeSummary("en")
	zh := GetPurposeSummary("zh-CN")
	if len(en) != len(zh) {
		t.Fatalf("language switch changed card count: en=%d zh=%d", len(en), len(zh))
	}
	for i, card := range en {
		if card.ID == "" {
			t.Errorf("card %d missing id", i)
		}
		if !strings.Contains(zh[i].Label, "聊") &&
			!strings.Contains(zh[i].Label, "编") &&
			!strings.Contains(zh[i].Label, "图") &&
			!strings.Contains(zh[i].Label, "视") &&
			!strings.Contains(zh[i].Label, "语") &&
			!strings.Contains(zh[i].Label, "全部") {
			// At least one Chinese character should appear in every zh label.
			t.Errorf("zh card %d label %q looks untranslated", i, zh[i].Label)
		}
	}
}

// TestVideoPurposeMatchesLiveModels pins the video purpose to the models the
// video page actually sells (Video First Wave PRD v0.9.3, AC-E). The bug it
// guards against: the whitelist pointing at models with no channel (the
// veo/sora/runway pre-launch placeholders) made every Simple-mode video key
// unable to call any live video model.
func TestVideoPurposeMatchesLiveModels(t *testing.T) {
	mustInit(t)

	list, ok := ModelWhitelistForToken("video", "", "")
	if !ok {
		t.Fatal("expected video whitelist to be non-empty")
	}
	for _, dead := range []string{"veo-*", "sora*", "runway*"} {
		if containsPattern(list, dead) {
			t.Errorf("video whitelist still carries placeholder %q: %v", dead, list)
		}
	}
	if !containsPattern(list, "MiniMax-H3") {
		t.Errorf("video whitelist missing MiniMax-H3: %v", list)
	}
	if !containsPattern(list, "doubao-seedance-*") {
		t.Errorf("video whitelist missing doubao-seedance-*: %v", list)
	}
	if !containsPattern(list, "deeprouter-video") {
		t.Errorf("video whitelist must keep the deeprouter-video virtual alias: %v", list)
	}

	// Every model the video page offers must pass the whitelist under
	// model.MatchModelLimit's exact-then-trailing-* semantics (mirrored
	// locally — importing model/ from setting/ would be a cycle).
	pageModels := []string{
		"MiniMax-H3",
		"doubao-seedance-2-5-260628",
		"doubao-seedance-2-0-260128",
	}
	for _, m := range pageModels {
		if !matchByLimitSemantics(list, m) {
			t.Errorf("video-page model %q not permitted by video whitelist %v", m, list)
		}
	}

	// deeprouter-video (and plain purpose=video auto) must resolve to the
	// video page's default model, not a placeholder.
	if got := ResolveAlias("video", "auto"); got != "MiniMax-H3" {
		t.Errorf("video+auto → %q, want MiniMax-H3", got)
	}
}

// TestVideoPurposeCardIsUSDPriced guards the card copy: DeepRouter pricing is
// USD-denominated, and the video card must not regress to the pre-launch RMB
// placeholder figures.
func TestVideoPurposeCardIsUSDPriced(t *testing.T) {
	mustInit(t)

	for _, lang := range []string{"en", "zh-CN"} {
		for _, card := range GetPurposeSummary(lang) {
			if card.ID != "video" {
				continue
			}
			if strings.Contains(card.HumanEstimate, "¥") || strings.Contains(card.PriceRange, "¥") {
				t.Errorf("[%s] video card still RMB-priced: %q / %q", lang, card.HumanEstimate, card.PriceRange)
			}
			if !strings.Contains(card.HumanEstimate, "$") || !strings.Contains(card.PriceRange, "$") {
				t.Errorf("[%s] video card not USD-priced: %q / %q", lang, card.HumanEstimate, card.PriceRange)
			}
		}
	}
}

// matchByLimitSemantics mirrors model.MatchModelLimit: exact first, then any
// entry ending in "*" as a prefix match.
func matchByLimitSemantics(list []string, modelName string) bool {
	for _, entry := range list {
		if entry == modelName {
			return true
		}
		if strings.HasSuffix(entry, "*") && strings.HasPrefix(modelName, strings.TrimSuffix(entry, "*")) {
			return true
		}
	}
	return false
}

func containsPattern(list []string, pattern string) bool {
	for _, p := range list {
		if p == pattern {
			return true
		}
	}
	return false
}
