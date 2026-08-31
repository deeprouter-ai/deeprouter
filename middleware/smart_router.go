package middleware

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/internal/smart_router_client"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

const (
	// VirtualModelAuto is the model name that triggers content-aware routing
	// via the smart-router sidecar.
	VirtualModelAuto = "deeprouter-auto"

	// DefaultAutoFallbackModel is used when smart-router is unreachable or
	// disabled. Chosen for the cheapest-reasonable-quality balance — admins
	// can override via SMART_ROUTER_DEFAULT_FALLBACK env (read at call time).
	DefaultAutoFallbackModel = "gpt-4o-mini"

	smartRouterCallTimeout = 150 * time.Millisecond
)

// chatRequestSnippet is a minimal subset of an incoming request used to
// extract the conversation for the smart-router call. We intentionally avoid
// coupling to dto.GeneralOpenAIRequest / dto.GeminiChatRequest because:
//   - The dto types carry fields the smart-router doesn't need (functions,
//     tool definitions, response format) and parsing them adds latency.
//   - Smart-router's input contract is stable (PRD §6.1); the dto types evolve
//     with upstream features.
//
// It covers all four protocols the gateway relays. Only OpenAI and Anthropic
// name the conversation `messages`; Gemini uses `contents` and the OpenAI
// Responses API (what Codex CLI speaks) uses `input`, each keeping its system
// prompt in a separate top-level field. A body only ever populates one shape.
type chatRequestSnippet struct {
	// OpenAI /v1/chat/completions and Anthropic /v1/messages.
	Messages []smart_router_client.Message `json:"messages"`

	// Google Gemini /v1beta/models/*. Google accepts systemInstruction in
	// either case convention, so both spellings are read (same as dto).
	Contents               []geminiContentSnippet `json:"contents"`
	SystemInstruction      *geminiContentSnippet  `json:"systemInstruction"`
	SystemInstructionSnake *geminiContentSnippet  `json:"system_instruction"`

	// OpenAI Responses /v1/responses.
	Input        json.RawMessage `json:"input"`
	Instructions string          `json:"instructions"`

	Stream bool `json:"stream,omitempty"`
}

type geminiContentSnippet struct {
	Role  string              `json:"role"`
	Parts []geminiPartSnippet `json:"parts"`
}

type geminiPartSnippet struct {
	Text            string          `json:"text"`
	InlineData      json.RawMessage `json:"inlineData"`
	InlineDataSnake json.RawMessage `json:"inline_data"`
	FileData        json.RawMessage `json:"fileData"`
}

// responsesInputItem is one turn of a Responses `input` array. Its content is
// itself either a bare string or an array of typed parts.
type responsesInputItem struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// conversation returns the request's messages in smart-router's neutral
// format, whichever protocol they arrived in. Returning nothing here is what
// makes resolveAutoModel fall back to the default model — which is correct for
// a genuinely empty request, and was the bug for Gemini and Responses.
func (s *chatRequestSnippet) conversation() []smart_router_client.Message {
	if len(s.Messages) > 0 {
		return s.Messages
	}
	if msgs := s.geminiConversation(); len(msgs) > 0 {
		return msgs
	}
	return s.responsesConversation()
}

func (s *chatRequestSnippet) geminiConversation() []smart_router_client.Message {
	system := s.SystemInstruction
	if system == nil {
		system = s.SystemInstructionSnake
	}
	out := make([]smart_router_client.Message, 0, len(s.Contents)+1)
	if system != nil {
		if content := geminiPartsContent(system.Parts); content != nil {
			out = append(out, smart_router_client.Message{Role: "system", Content: content})
		}
	}
	for _, c := range s.Contents {
		content := geminiPartsContent(c.Parts)
		if content == nil {
			continue
		}
		out = append(out, smart_router_client.Message{Role: geminiRole(c.Role), Content: content})
	}
	return out
}

// geminiRole maps Gemini's speaker names onto the OpenAI ones the contract
// declares. Gemini says "model" where OpenAI says "assistant".
func geminiRole(role string) string {
	switch role {
	case "model":
		return "assistant"
	case "":
		return "user"
	default:
		return role
	}
}

// geminiPartsContent flattens one Gemini message's parts. A message may carry
// several and keeping only the first drops most of the prompt.
func geminiPartsContent(parts []geminiPartSnippet) any {
	var texts []string
	media := 0
	for _, p := range parts {
		if p.Text != "" {
			texts = append(texts, p.Text)
		}
		if len(p.InlineData) > 0 || len(p.InlineDataSnake) > 0 || len(p.FileData) > 0 {
			media++
		}
	}
	return neutralContent(texts, media)
}

func (s *chatRequestSnippet) responsesConversation() []smart_router_client.Message {
	var out []smart_router_client.Message
	if s.Instructions != "" {
		out = append(out, smart_router_client.Message{Role: "system", Content: s.Instructions})
	}
	switch common.GetJsonType(s.Input) {
	case "string":
		var text string
		if err := json.Unmarshal(s.Input, &text); err == nil && text != "" {
			out = append(out, smart_router_client.Message{Role: "user", Content: text})
		}
	case "array":
		var items []responsesInputItem
		if err := json.Unmarshal(s.Input, &items); err == nil {
			for _, item := range items {
				content := responsesItemContent(item.Content)
				if content == nil {
					continue
				}
				role := item.Role
				if role == "" {
					role = "user"
				}
				out = append(out, smart_router_client.Message{Role: role, Content: content})
			}
		}
	}
	return out
}

// responsesItemContent flattens one Responses turn. Codex CLI sends the array
// form, but the single-turn string form is equally valid and skipping it would
// leave the most common shape untranslated.
func responsesItemContent(raw json.RawMessage) any {
	switch common.GetJsonType(raw) {
	case "string":
		var text string
		if err := json.Unmarshal(raw, &text); err == nil && text != "" {
			return text
		}
	case "array":
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &parts); err != nil {
			return nil
		}
		var texts []string
		media := 0
		for _, p := range parts {
			if p.Text != "" {
				texts = append(texts, p.Text)
			}
			if p.Type == "input_image" {
				media++
			}
		}
		return neutralContent(texts, media)
	}
	return nil
}

// neutralContent picks the content shape smart-router can actually read: a
// plain string for text (what every other protocol sends), or the parts array
// when the turn has media — that array is the only form its vision detection
// recognises, so text-only flattening would silently route an image request to
// a model that cannot see.
func neutralContent(texts []string, media int) any {
	if media == 0 {
		if len(texts) == 0 {
			return nil
		}
		return strings.Join(texts, "\n")
	}
	out := make([]any, 0, len(texts)+media)
	for _, t := range texts {
		out = append(out, map[string]any{"type": "text", "text": t})
	}
	for i := 0; i < media; i++ {
		out = append(out, map[string]any{"type": "image_url"})
	}
	return out
}

// ResolveAutoModel attempts to swap modelName == "deeprouter-auto" for a
// concrete model name via the smart-router sidecar. Returns the resolved
// name on success, an empty string on graceful failure. Context keys + the
// X-DeepRouter-Routed-Model response header are set on success.
//
// Failure modes (all return DefaultAutoFallbackModel + recorded reason):
//   - SMART_ROUTER_URL unset → "smart_router_disabled"
//   - no conversation found in the body, in ANY of the four protocol shapes →
//     "smart_router_no_messages" (see chatRequestSnippet.conversation)
//   - smart-router HTTP call errored → "smart_router_error"
//   - smart-router returned a sentinel no-decision response → "smart_router_no_decision"
//
// The caller (Distribute) treats a non-empty return as "use this model and
// continue"; an empty return is treated as "leave the model name alone".
//
// Wraps resolveAutoModel with the process-wide Default() client; tests use
// the unexported variant with their own httptest-backed client.
func ResolveAutoModel(c *gin.Context, modelName string) string {
	return resolveAutoModel(c, modelName, smart_router_client.Default())
}

func resolveAutoModel(c *gin.Context, modelName string, client *smart_router_client.Client) string {
	if modelName != VirtualModelAuto {
		return ""
	}

	originalModel := modelName

	// Parse only the snippet we need from the request body. Failure here
	// is non-fatal — we fall back to the default model.
	var snippet chatRequestSnippet
	_ = common.UnmarshalBodyReusable(c, &snippet)
	messages := snippet.conversation()

	tenantID := strconv.Itoa(common.GetContextKeyInt(c, constant.ContextKeyUserId))

	resolved := DefaultAutoFallbackModel
	reason := "smart_router_disabled"
	strategy := ""
	var fallbackChain []string

	switch {
	case !client.Enabled():
		reason = "smart_router_disabled"
	case len(messages) == 0:
		reason = "smart_router_no_messages"
	default:
		ctx, cancel := context.WithTimeout(c.Request.Context(), smartRouterCallTimeout)
		defer cancel()
		req := smart_router_client.RouteRequest{
			TenantID:  tenantID,
			Messages:  messages,
			RequestID: c.GetString("request_id"),
			Stream:    snippet.Stream,
		}
		decision, err := client.Route(ctx, req)
		switch {
		case err != nil:
			common.SysError("smart-router call failed: " + err.Error())
			reason = "smart_router_error"
		case decision == nil:
			reason = "smart_router_no_decision"
		default:
			resolved = decision.Primary
			reason = decision.Reason
			strategy = decision.StrategyVersion
			fallbackChain = decision.FallbackChain
		}
	}

	// The token's model whitelist is enforced later in Distribute on whatever
	// name we return here (MatchModelLimit on the RESOLVED name) — but
	// smart-router picks from the TENANT's catalog, which is wider than any
	// one token's whitelist. Without this re-filter, a limited token gets a
	// hard 403 on a model the user never typed. Re-pick inside the whitelist:
	// primary → fallback chain → cheapest whitelisted model the group can
	// serve. See docs/adr/0007-auto-model-token-whitelist.md.
	resolved, fallbackChain, reason = filterByTokenWhitelist(c, resolved, fallbackChain, reason)
	if len(fallbackChain) > 0 {
		common.SetContextKey(c, constant.ContextKeySmartRouterFallback, fallbackChain)
	}

	common.SetContextKey(c, constant.ContextKeyAliasResolvedFrom, originalModel)
	common.SetContextKey(c, constant.ContextKeySmartRouterReason, reason)
	if strategy != "" {
		common.SetContextKey(c, constant.ContextKeySmartRouterStrategy, strategy)
	}

	c.Header("X-DeepRouter-Routed-Model", resolved)
	c.Header("X-DeepRouter-Routed-Reason", reason)
	if strategy != "" {
		c.Header("X-DeepRouter-Routed-Strategy", strategy)
	}

	return resolved
}

// filterByTokenWhitelist re-checks a smart-router pick against the token's
// model whitelist — the exact check Distribute runs later (FormatMatchingModelName
// + MatchModelLimit), so whatever this returns is guaranteed to pass it.
//
// Selection order: primary if allowed → first allowed fallback-chain entry →
// cheapest whitelisted model the request's group can actually serve. The chain
// is also filtered, because controller/relay_cross_model.go retries through it
// and a non-whitelisted entry there would hit the same 403 on retry.
//
// Inputs come back unchanged when the token has no whitelist, or when nothing
// at all survives the filter — in that last case the later 403 is honest: the
// token cannot use anything, and hiding that would only move the error.
func filterByTokenWhitelist(c *gin.Context, primary string, chain []string, reason string) (string, []string, string) {
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		return primary, chain, reason
	}
	raw, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
	if !ok {
		return primary, chain, reason
	}
	limits, ok := raw.(map[string]bool)
	if !ok || len(limits) == 0 {
		return primary, chain, reason
	}
	allowed := func(m string) bool {
		return model.MatchModelLimit(limits, ratio_setting.FormatMatchingModelName(m))
	}

	kept := make([]string, 0, len(chain))
	for _, m := range chain {
		if allowed(m) {
			kept = append(kept, m)
		}
	}
	if allowed(primary) {
		return primary, kept, reason
	}
	if len(kept) > 0 {
		return kept[0], kept[1:], reason + "+token_whitelist"
	}
	if m := cheapestAllowedGroupModel(c, allowed); m != "" {
		return m, nil, reason + "+token_whitelist_degraded"
	}
	return primary, chain, reason
}

// groupModelsForWhitelistFallback lists the models the request's group can
// serve, expanding the "auto" pseudo-group the same way channel selection
// does. Package-level var so unit tests can stub the DB-backed lookup.
var groupModelsForWhitelistFallback = func(c *gin.Context) []string {
	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if usingGroup != "auto" {
		return model.GetGroupEnabledModels(usingGroup)
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	seen := make(map[string]bool)
	var out []string
	for _, g := range service.GetUserAutoGroup(userGroup) {
		for _, m := range model.GetGroupEnabledModels(g) {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	return out
}

// cheapestAllowedGroupModel picks the cheapest chat-usable whitelisted model
// the group can serve; empty string when none qualifies. Per-call-priced
// models (image / video / audio — the same set the router catalog skips) are
// excluded: routing a chat request to them fails or pre-charges nonsense.
func cheapestAllowedGroupModel(c *gin.Context, allowed func(string) bool) string {
	best, bestRatio := "", math.MaxFloat64
	for _, m := range groupModelsForWhitelistFallback(c) {
		if m == "" || !allowed(m) {
			continue
		}
		if _, perCall := ratio_setting.GetModelPrice(m, false); perCall {
			continue
		}
		ratio, _, _ := ratio_setting.GetModelRatio(m)
		// Tie-break by name so the pick is deterministic across restarts.
		if ratio < bestRatio || (ratio == bestRatio && m < best) {
			best, bestRatio = m, ratio
		}
	}
	return best
}
