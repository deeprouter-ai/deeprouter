package middleware

import (
	"context"
	"math"
	"strconv"
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

// chatRequestSnippet is a minimal subset of the OpenAI chat request used to
// extract messages for the smart-router call. We intentionally avoid coupling
// to dto.GeneralOpenAIRequest because:
//   - The dto type carries fields the smart-router doesn't need (functions,
//     tool definitions, response format) and parsing them adds latency.
//   - Smart-router's input contract is stable (PRD §6.1); the dto type evolves
//     with upstream features.
type chatRequestSnippet struct {
	Messages []smart_router_client.Message `json:"messages"`
	Stream   bool                          `json:"stream,omitempty"`
}

// ResolveAutoModel attempts to swap modelName == "deeprouter-auto" for a
// concrete model name via the smart-router sidecar. Returns the resolved
// name on success, an empty string on graceful failure. Context keys + the
// X-DeepRouter-Routed-Model response header are set on success.
//
// Failure modes (all return DefaultAutoFallbackModel + recorded reason):
//   - SMART_ROUTER_URL unset → "smart_router_disabled"
//   - empty messages parsed from body → "smart_router_no_messages"
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

	tenantID := strconv.Itoa(common.GetContextKeyInt(c, constant.ContextKeyUserId))

	resolved := DefaultAutoFallbackModel
	reason := "smart_router_disabled"
	strategy := ""
	var fallbackChain []string

	switch {
	case !client.Enabled():
		reason = "smart_router_disabled"
	case len(snippet.Messages) == 0:
		reason = "smart_router_no_messages"
	default:
		ctx, cancel := context.WithTimeout(c.Request.Context(), smartRouterCallTimeout)
		defer cancel()
		req := smart_router_client.RouteRequest{
			TenantID:  tenantID,
			Messages:  snippet.Messages,
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
