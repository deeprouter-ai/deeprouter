package alias_setting

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
		"claude-sonnet-5":      false,
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
	if got := ResolveAlias("coding", "openai"); got != "gpt-5.4" {
		t.Errorf("coding+openai → %q, want gpt-5.4", got)
	}
	// Brand missing → fall back to auto.
	if got := ResolveAlias("coding", "gemini"); got != "claude-sonnet-5" {
		t.Errorf("coding+gemini → %q, want auto fallback claude-sonnet-5", got)
	}
	// Purpose entirely missing → empty.
	if got := ResolveAlias("nonsense", "claude"); got != "" {
		t.Errorf("nonsense+claude → %q, want empty", got)
	}
	// Empty brand → auto.
	if got := ResolveAlias("chat", ""); got != "claude-sonnet-5" {
		t.Errorf("chat+empty → %q, want claude-sonnet-5", got)
	}
}

func TestResolveAliasForVirtualModelOverridesPurpose(t *testing.T) {
	mustInit(t)

	// Token bound to chat, but client asks for coding via virtual model name.
	if got := ResolveAliasForVirtualModel("deeprouter-coding", "chat", "openai"); got != "gpt-5.4" {
		t.Errorf("deeprouter-coding under chat token → %q, want gpt-5.4", got)
	}
	// Plain "deeprouter" honours the token's bound purpose.
	if got := ResolveAliasForVirtualModel("deeprouter", "chat", "claude"); got != "claude-sonnet-5" {
		t.Errorf("deeprouter under chat+claude → %q, want claude-sonnet-5", got)
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

// The three checks a key's binding goes through before anything reads it
// (meta-repo docs/adlc/tasks/fix-key-purpose-validation-and-whitelists-task.md):
// a value is known when the seed defines it, and nothing else is.
func TestKnownPurposeBrandAndPriceTier(t *testing.T) {
	mustInit(t)

	for _, id := range []string{"chat", "coding", "image", "video", "voice", "all"} {
		if !KnownPurpose(id) {
			t.Errorf("KnownPurpose(%q) = false, want true", id)
		}
	}
	for _, name := range []string{"claude", "openai", "gemini", "deepseek"} {
		if !KnownBrand(name) {
			t.Errorf("KnownBrand(%q) = false, want true", name)
		}
	}
	for _, id := range []string{"economy", "standard", "premium", "ultra"} {
		if !KnownPriceTier(id) {
			t.Errorf("KnownPriceTier(%q) = false, want true", id)
		}
	}
	// A typo, another field's value, a persona, the wrong case, nothing at all.
	for _, unknown := range []string{"", "codng", "Chat", "CHAT", "chat ", "dev", "casual", "team", "auto", "standard", "claude"} {
		if KnownPurpose(unknown) {
			t.Errorf("KnownPurpose(%q) = true, want false", unknown)
		}
	}
	for _, unknown := range []string{"", "claud", "Claude", "anthropic", "auto", "chat", "ultra"} {
		if KnownBrand(unknown) {
			t.Errorf("KnownBrand(%q) = true, want false", unknown)
		}
	}
	for _, unknown := range []string{"", "standrad", "Standard", "free", "all", "claude"} {
		if KnownPriceTier(unknown) {
			t.Errorf("KnownPriceTier(%q) = true, want false", unknown)
		}
	}
}

// ruleLists returns every rule list of the seed that ends up in a key's
// model_limits, by what it belongs to.
func ruleLists(t *testing.T) map[string][]string {
	t.Helper()
	mustInit(t)
	lists := map[string][]string{}
	for _, purpose := range purposes {
		lists["purpose "+purpose.ID] = purpose.ModelWhitelist
	}
	for id, tier := range priceTiers {
		lists["tier "+id] = tier.ModelWhitelist
	}
	return lists
}

// A rule matches exactly, or — ending in "*" — by prefix (model.MatchModelLimit).
// A "*" anywhere else is read as a character of the name, so such a rule
// matches no model at all: "gemini-2*-flash*" sat in the economy tier without
// ever letting a Gemini model through.
func TestRules_WildcardOnlyAtTheEnd(t *testing.T) {
	for owner, rules := range ruleLists(t) {
		for _, rule := range rules {
			if rule == "" {
				t.Errorf("%s has an empty rule", owner)
			}
			if at := strings.Index(rule, "*"); at >= 0 && at != len(rule)-1 {
				t.Errorf("%s: rule %q has a wildcard that is not its last character", owner, rule)
			}
		}
	}
}

// An empty rule list is how "no model limit" is written, and exactly two
// things mean that: the purpose "all", which takes its rules from a price
// tier, and the top tier, which has none and asks for a confirmation. Any
// other list that came out empty would turn the keys bound to it loose.
func TestRules_OnlyTheTopTierHasNone(t *testing.T) {
	for owner, rules := range ruleLists(t) {
		unlimited := owner == "purpose all" || owner == "tier ultra"
		if unlimited != (len(rules) == 0) {
			t.Errorf("%s has %d rules; only \"purpose all\" and \"tier ultra\" have none", owner, len(rules))
		}
	}
	for _, id := range []string{"economy", "standard", "premium"} {
		if _, limited := ModelWhitelistForToken("all", "", id); !limited {
			t.Errorf("a key for everything at the %s tier must be limited", id)
		}
	}
	for _, id := range []string{"chat", "coding"} {
		if _, limited := ModelWhitelistForToken(id, "", ""); !limited {
			t.Errorf("a %s key must be limited", id)
		}
	}
}

// modelsOnSale reads the models of every enabled channel of the seed file the
// operators provision a deployment from (scripts/seed-models/channels.yaml):
// the closest thing the repository has to "what is on sale".
func modelsOnSale(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../scripts/seed-models/channels.yaml")
	if err != nil {
		t.Fatalf("the seed file of channels: %v", err)
	}
	var seed struct {
		Channels []struct {
			Enabled *bool    `yaml:"enabled"`
			Models  []string `yaml:"models"`
		} `yaml:"channels"`
	}
	if err := yaml.Unmarshal(raw, &seed); err != nil {
		t.Fatalf("the seed file of channels: %v", err)
	}
	var models []string
	for _, channel := range seed.Channels {
		if channel.Enabled == nil || *channel.Enabled {
			models = append(models, channel.Models...)
		}
	}
	if len(models) < 50 {
		t.Fatalf("the seed file lists only %d models; has its shape changed?", len(models))
	}
	return models
}

// The chat and coding lists were audited against what is on sale on
// 2026-10-08 (they had fallen a generation behind: a chat key could call
// Claude and nothing else that was sold). This keeps them from going stale
// unnoticed: every rule lets at least one model on sale through, so a family
// that is retired takes its rule with it.
func TestChatAndCodingRulesMatchWhatIsOnSale(t *testing.T) {
	onSale := modelsOnSale(t)
	lists := ruleLists(t)
	for _, purpose := range []string{"purpose chat", "purpose coding"} {
		for _, rule := range lists[purpose] {
			matched := false
			for _, name := range onSale {
				if matchByLimitSemantics([]string{rule}, name) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("%s: rule %q matches no model on sale", purpose, rule)
			}
		}
	}
}

// A model made for writing code says so in its name. Whichever of them is on
// sale, a coding key lets it through — "codestral-latest" was sold for months
// while no coding key could call it.
func TestCodingRulesCoverTheModelsMadeForCode(t *testing.T) {
	coding := ruleLists(t)["purpose coding"]
	for _, name := range modelsOnSale(t) {
		lower := strings.ToLower(name)
		madeForCode := false
		for _, word := range []string{"codex", "codestral", "coder", "-code"} {
			madeForCode = madeForCode || strings.Contains(lower, word)
		}
		// A provider-prefixed name ("Qwen/…") is another catalogue's spelling.
		if madeForCode && !strings.Contains(name, "/") && !matchByLimitSemantics(coding, name) {
			t.Errorf("%q is made for code and on sale, and no coding rule lets it through", name)
		}
	}
}

// A default model is what the virtual names — "deeprouter", "deeprouter-coding"
// … — resolve to. One that is no longer sold turns every such request into
// "no channel for this model": on 2026-10-08 nine of the thirteen defaults
// were on sale nowhere.
func TestEveryDefaultModelIsOnSale(t *testing.T) {
	mustInit(t)
	onSale := map[string]bool{}
	for _, name := range modelsOnSale(t) {
		onSale[name] = true
	}
	for purpose, byBrand := range aliasMap {
		for brand, target := range byBrand {
			if !onSale[target] {
				t.Errorf("%s + %s resolves to %q, which no enabled channel of the seed file sells", purpose, brand, target)
			}
		}
	}
}
