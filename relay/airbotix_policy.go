package relay

// Airbotix policy hooks for the V0 kids_mode hard constraints, covering both
// the request AND response lifecycle.
//
// Request side: applied to the typed request struct in each relay handler
// BEFORE the request is converted to a provider-specific payload, so that
// downstream adapters translate the kid-safe constraints automatically.
//
// Behaviours (all gated by policy.Decision from middleware/policy.go):
//   - EnforceModelWhitelist: reject early with HTTP 400 if request.Model is
//     not on kids.EligibleModels. Runs in every relay handler.
//   - InjectSystemPrompt: prepend (or replace, when KidsMode=true) the
//     per-profile system prompt. Chat-shaped endpoints only.
//   - RunInputFilter: reject entry input text that matches the profile denylist.
//   - StripIdentifying: clear User / SafetyIdentifier / Metadata.user_id.
//     Applied per-format where these fields exist.
//   - EnforceZDR + OpenAI-family channel: force Store=false. OpenAI shapes
//     only (chat/completions, responses).
//
// Response side: EnforceStrictOutputFilter installs outputFilterWriter (see
// wrapOutputFilterWriter below) around the relay handler's response-producing
// call. It buffers the ENTIRE response (non-stream body or full SSE stream),
// classifies the assistant-visible text via internal/kids' OutputFilter and
// ShapeFilter, and either passes the buffered bytes through unchanged or
// replaces them with a kids.SafeFallbackText()-based response in the same
// wire format — before any byte reaches the client.
//
// ADR-0006 controlled expansion (DR-30): wiring the response-side hook into
// relay/compatible_handler.go, relay/claude_handler.go,
// relay/responses_handler.go, and relay/gemini_handler.go is a deliberate,
// scoped expansion of this package's footprint into those 4 handler files.
// Each handler is limited to four thin touchpoints — obtaining the
// policy.Decision, calling wrapOutputFilterWriter, a deferred restore(), and
// an explicit restore() after the response-producing call returns — with
// ALL parsing/filter/fallback logic staying in this file and internal/kids.
//
// See internal/kids and internal/policy for the source semantics.

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/internal/kids"
	"github.com/QuantumNous/new-api/internal/policy"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// maxTokensHardCap is the global ceiling for max_tokens / max_completion_tokens
// across all request shapes and all tenants. Prevents a single request from
// consuming unbounded upstream tokens regardless of quota settings.
const maxTokensHardCap uint = 2048

// policyDecisionFromContext returns the policy.Decision stashed by
// middleware/policy.go, or zero (passthrough) + false when none is present.
func policyDecisionFromContext(c *gin.Context) (policy.Decision, bool) {
	raw, ok := common.GetContextKey(c, constant.ContextKeyPolicyDecision)
	if !ok || raw == nil {
		return policy.Decision{}, false
	}
	d, ok := raw.(policy.Decision)
	return d, ok
}

// rejectAirbotixModel builds the consistent 400 error returned by every relay
// handler when a kids_mode tenant requests a non-whitelisted model.
func rejectAirbotixModel(model string) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		fmt.Errorf("model_not_eligible_for_kids_mode: %s", model),
		types.ErrorCodeChannelModelMappedError,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

func rejectAirbotixInput(deny *policy.InputDeny) *types.NewAPIError {
	reason := "policy_input_blocked"
	if deny != nil {
		reason = deny.Reason()
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s", reason),
		types.ErrorCodeInvalidRequest,
		http.StatusUnprocessableEntity,
		types.ErrOptionWithSkipRetry(),
	)
}

// checkAirbotixModelWhitelist is the minimal universal guard: every relay
// handler calls this after request parsing to enforce kids.EligibleModels.
// Returns non-nil error to abort the request; nil means continue.
func checkAirbotixModelWhitelist(c *gin.Context, model string) *types.NewAPIError {
	d, ok := policyDecisionFromContext(c)
	if !ok || !d.EnforceModelWhitelist {
		return nil
	}
	if kids.IsModelEligible(model) {
		return nil
	}
	return rejectAirbotixModel(model)
}

// prependProfileSystemPrompt mutates request.Messages so that the profile
// system prompt is the effective system message seen by upstream.
//
// kidsMode=true is a hard constraint: any incoming system message is replaced.
// kidsMode=false (i.e. profile alone) is softer: prepend only if no
// system message exists.
func prependProfileSystemPrompt(request *dto.GeneralOpenAIRequest, decision policy.Decision) {
	if request == nil {
		return
	}
	prompt, ok := policy.SystemPromptFor(decision)
	if !ok {
		return
	}
	role := request.GetSystemRoleName()
	for i, m := range request.Messages {
		if m.Role == role || m.Role == "system" || m.Role == "developer" {
			if decision.KidsMode {
				request.Messages[i].Role = role
				request.Messages[i].SetStringContent(prompt)
			}
			return
		}
	}
	sysMsg := dto.Message{Role: role}
	sysMsg.SetStringContent(prompt)
	request.Messages = append([]dto.Message{sysMsg}, request.Messages...)
}

// isOpenAIFamilyChannel returns true for channels where OpenAI's `store: false`
// Zero-Data-Retention flag is honoured. Non-OpenAI providers ignore the field
// (or worse, reject it), so we limit ZDR injection to the OpenAI family.
func isOpenAIFamilyChannel(channelType int) bool {
	switch channelType {
	case constant.ChannelTypeOpenAI,
		constant.ChannelTypeAzure,
		constant.ChannelTypeOpenAIMax:
		return true
	}
	return false
}

func rawJSON(v any) []byte {
	data, err := common.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

// clampUint returns v clamped to ceiling. If v == nil, returns nil unchanged.
func clampUint(v *uint, ceiling uint) *uint {
	if v != nil && *v > ceiling {
		return &ceiling
	}
	return v
}

func collectGeneralOpenAIInputTexts(request *dto.GeneralOpenAIRequest) []string {
	if request == nil {
		return nil
	}
	texts := make([]string, 0, len(request.Messages)+3)
	for _, message := range request.Messages {
		if message.Role == "user" || message.Role == "" {
			texts = append(texts, message.StringContent())
		}
	}
	texts = append(texts, collectAnyText(request.Prompt)...)
	texts = append(texts, collectAnyText(request.Input)...)
	if request.Instruction != "" {
		texts = append(texts, request.Instruction)
	}
	return texts
}

func collectAnyText(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		texts := make([]string, 0, len(v))
		for _, item := range v {
			texts = append(texts, collectAnyText(item)...)
		}
		return texts
	case map[string]any:
		var texts []string
		if text, ok := v["text"].(string); ok {
			texts = append(texts, text)
		}
		if content, ok := v["content"]; ok {
			texts = append(texts, collectAnyText(content)...)
		}
		return texts
	default:
		return nil
	}
}

// applyAirbotixPolicy is the single mutation entry-point called from TextHelper.
// Returns a non-nil reject string when the model whitelist check denies the
// request; otherwise mutates request in place and returns "".
func applyAirbotixPolicy(decision policy.Decision, channelType int, request *dto.GeneralOpenAIRequest) (rejectReason string) {
	if request == nil {
		return ""
	}
	request.MaxTokens = clampUint(request.MaxTokens, maxTokensHardCap)
	request.MaxCompletionTokens = clampUint(request.MaxCompletionTokens, maxTokensHardCap)
	if decision.EnforceModelWhitelist && !kids.IsModelEligible(request.Model) {
		return "model_not_eligible_for_kids_mode: " + request.Model
	}
	if deny := policy.CheckInput(decision, collectGeneralOpenAIInputTexts(request)...); deny != nil {
		return deny.Reason()
	}
	if decision.InjectSystemPrompt {
		prependProfileSystemPrompt(request, decision)
	}
	if decision.StripIdentifying {
		request.User = nil
		request.SafetyIdentifier = nil
	}
	if decision.EnforceZDR && isOpenAIFamilyChannel(channelType) {
		request.Store = rawJSON(false)
	}
	return ""
}

// applyAirbotixPolicyToClaude mutates a *dto.ClaudeRequest (Anthropic native
// /v1/messages shape) in place. Returns non-nil error if the model is blocked.
//
// Anthropic-specific semantics:
//   - System: replace under kids_mode (hard); prepend-if-empty under kid-safe
//     profile only. The field is `any` (string OR array of content blocks);
//     we always normalise to a string for simplicity.
//   - Metadata: cleared entirely under StripIdentifying. The kids package's
//     StripIdentifyingMetadata operates on map[string]any; the field here is
//     json.RawMessage, and Anthropic accepts a missing metadata field, so we
//     just drop it rather than parse + filter + re-marshal.
//   - Store: N/A for Anthropic.
func applyAirbotixPolicyToClaude(c *gin.Context, request *dto.ClaudeRequest) *types.NewAPIError {
	if request == nil {
		return nil
	}
	request.MaxTokens = clampUint(request.MaxTokens, maxTokensHardCap)
	request.MaxTokensToSample = clampUint(request.MaxTokensToSample, maxTokensHardCap)
	d, ok := policyDecisionFromContext(c)
	if !ok {
		return nil
	}
	if d.EnforceModelWhitelist && !kids.IsModelEligible(request.Model) {
		return rejectAirbotixModel(request.Model)
	}
	if deny := policy.CheckInput(d, collectClaudeInputTexts(request)...); deny != nil {
		return rejectAirbotixInput(deny)
	}
	if d.InjectSystemPrompt {
		prompt, _ := policy.SystemPromptFor(d)
		// kids_mode is hard: replace whatever the client sent.
		// profile alone is soft: only fill if empty.
		if d.KidsMode || request.System == nil {
			request.System = prompt
		} else if s, isStr := request.System.(string); isStr && s == "" {
			request.System = prompt
		}
	}
	if d.StripIdentifying {
		request.Metadata = nil
	}
	return nil
}

// applyAirbotixPolicyToResponses mutates a *dto.OpenAIResponsesRequest in
// place (OpenAI /v1/responses shape). Returns non-nil error if the model is
// blocked.
//
// Notes vs the chat/completions shape:
//   - Instructions is json.RawMessage (string-or-array); we only replace it
//     under hard kids_mode and only with a JSON-encoded string. For kid-safe
//     profile we only fill when empty, to avoid clobbering caller intent.
//   - Store + User + SafetyIdentifier match the chat shape and are cleared
//     /forced the same way.
func applyAirbotixPolicyToResponses(c *gin.Context, channelType int, request *dto.OpenAIResponsesRequest) *types.NewAPIError {
	if request == nil {
		return nil
	}
	request.MaxOutputTokens = clampUint(request.MaxOutputTokens, maxTokensHardCap)
	d, ok := policyDecisionFromContext(c)
	if !ok {
		return nil
	}
	if d.EnforceModelWhitelist && !kids.IsModelEligible(request.Model) {
		return rejectAirbotixModel(request.Model)
	}
	if deny := policy.CheckInput(d, collectResponsesInputTexts(request)...); deny != nil {
		return rejectAirbotixInput(deny)
	}
	if d.InjectSystemPrompt {
		prompt, _ := policy.SystemPromptFor(d)
		promptJSON, mErr := common.Marshal(prompt)
		if mErr == nil {
			if d.KidsMode || len(request.Instructions) == 0 || string(request.Instructions) == "null" {
				request.Instructions = promptJSON
			}
		}
	}
	if d.StripIdentifying {
		request.User = nil
		request.SafetyIdentifier = nil
	}
	if d.EnforceZDR && isOpenAIFamilyChannel(channelType) {
		request.Store = rawJSON(false)
	}
	return nil
}

// applyAirbotixPolicyToGemini mutates a *dto.GeminiChatRequest in place.
// Returns non-nil error if the model is blocked.
//
// Notes:
//   - GeminiChatRequest does NOT carry the model name (the model is on
//     RelayInfo, taken from the URL path) — the caller passes it explicitly.
//   - SystemInstructions: replace under kids_mode with a single-text content
//     block; under kid-safe profile, only fill when nil.
//   - Gemini has no User/Store equivalents to strip.
func applyAirbotixPolicyToGemini(c *gin.Context, model string, request *dto.GeminiChatRequest) *types.NewAPIError {
	// Clamp before the policy check so the hard cap applies even when
	// policyDecisionFromContext returns !ok (transient DB error → passthrough).
	if request != nil {
		request.GenerationConfig.MaxOutputTokens = clampUint(request.GenerationConfig.MaxOutputTokens, maxTokensHardCap)
	}
	d, ok := policyDecisionFromContext(c)
	if !ok {
		return nil
	}
	if d.EnforceModelWhitelist && !kids.IsModelEligible(model) {
		return rejectAirbotixModel(model)
	}
	if request == nil {
		return nil
	}
	if deny := policy.CheckInput(d, collectGeminiInputTexts(request)...); deny != nil {
		return rejectAirbotixInput(deny)
	}
	if d.InjectSystemPrompt {
		prompt, _ := policy.SystemPromptFor(d)
		if d.KidsMode || request.SystemInstructions == nil {
			request.SystemInstructions = &dto.GeminiChatContent{
				Role: "system",
				Parts: []dto.GeminiPart{
					{Text: prompt},
				},
			}
		}
	}
	return nil
}

func collectClaudeInputTexts(request *dto.ClaudeRequest) []string {
	if request == nil {
		return nil
	}
	texts := make([]string, 0, len(request.Messages)+1)
	if request.Prompt != "" {
		texts = append(texts, request.Prompt)
	}
	for _, message := range request.Messages {
		if message.Role == "user" || message.Role == "" {
			texts = append(texts, message.GetStringContent())
		}
	}
	return texts
}

func collectResponsesInputTexts(request *dto.OpenAIResponsesRequest) []string {
	if request == nil {
		return nil
	}
	inputs := request.ParseInput()
	texts := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if input.Text != "" {
			texts = append(texts, input.Text)
		}
	}
	return texts
}

func collectGeminiInputTexts(request *dto.GeminiChatRequest) []string {
	if request == nil {
		return nil
	}
	var texts []string
	for _, content := range request.Contents {
		if content.Role != "" && content.Role != "user" {
			continue
		}
		for _, part := range content.Parts {
			if part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
	}
	return texts
}

// =============================================================================
// Response side: outputFilterWriter (PRD §6.4-pre, design doc §5)
// =============================================================================

// outputFilter is the OutputFilter consulted by outputFilterWriter.
// kids.StrictKeywordFilter{} is the CONFIRMED V0 default (design doc §3.1 /
// §3.3): a deny-list keyword scan, NOT the final Airbotix moderation
// classifier. It is wired in now so EnforceStrictOutputFilter has a real
// (if blunt) effect from day one; a future DR swaps this value for a richer
// kids.OutputFilter implementation behind the same interface — no other code
// in this file needs to change.
var outputFilter kids.OutputFilter = kids.StrictKeywordFilter{}

// maxOutputFilterBufferBytes caps how much of a single response
// outputFilterWriter buffers in memory before treating it as unverifiable.
// Confirmed 1 MiB (DR-30 阶段0 核对3): maxTokensHardCap=2048 bounds normal
// assistant output to well under this limit with a comfortable margin,
// kids_mode traffic uses small models with short responses, and the overflow
// path itself fails closed (2xx) or passes through truncated (non-2xx)
// rather than letting the buffer grow unbounded.
const maxOutputFilterBufferBytes = 1 << 20

// wrapOutputFilterWriter installs outputFilterWriter on c.Writer when
// decision.EnforceStrictOutputFilter is set, so that the ENTIRE response
// (non-stream body or full SSE stream) is buffered, classified against
// outputFilter via kids.FilterForShape(shape), and either passed through
// unchanged or replaced with a kids.SafeFallbackText()-based response in the
// same wire format — before any byte reaches the client.
//
// When EnforceStrictOutputFilter is false, restore is a no-op func — callers
// can unconditionally `defer restore()` without branching, so the buffering
// cost is paid only by kids_mode / kid-safe tenants.
//
// Calling convention for the 4 handler files (阶段8, ADR-0006 controlled
// expansion):
//
//	restore := wrapOutputFilterWriter(c, decision, kids.ResponseShapeXxx)
//	defer restore()
//	... existing response-producing call (adaptor.DoResponse / chatCompletionsViaResponses) ...
//	restore()
//
// The `defer restore()` is a safety net for error returns BEFORE the
// response-producing call: outputFilterWriter.finalize() is a no-op when
// nothing was ever written through it (see the `wrote` field below), so a
// deferred restore on a pre-response error path can never synthesize a
// spurious fallback body or an implicit 200.
//
// The explicit `restore()` AFTER the response-producing call is what
// actually runs finalize() and flushes the (possibly replaced) bytes to the
// client — any code that runs after it (quota settlement, billing dispatch,
// a future reader of ContextKeyOutputFilterViolations) observes the
// post-finalize state. finalize() is idempotent, so the deferred call that
// follows an explicit restore() is always a guaranteed no-op.
func wrapOutputFilterWriter(c *gin.Context, decision policy.Decision, shape kids.ResponseShape) (restore func()) {
	if !decision.EnforceStrictOutputFilter {
		return func() {}
	}
	orig := c.Writer
	w := &outputFilterWriter{ResponseWriter: orig, c: c, shape: shape, filter: outputFilter}
	c.Writer = w
	return func() {
		w.finalize()
		c.Writer = orig
	}
}

// outputFilterWriter wraps a gin.ResponseWriter and buffers every byte of the
// response (up to maxOutputFilterBufferBytes) instead of writing it through,
// so finalize() can classify the complete response before anything reaches
// the client.
//
// All response-writing / header-flushing members of gin.ResponseWriter are
// overridden below (WriteHeader, WriteHeaderNow, Write, WriteString, Flush)
// along with the state-inspection members some adapters rely on (Written,
// Size, Status) — any of these left to promote from the embedded
// gin.ResponseWriter would bypass buffering and write straight to the real
// connection. Flush in particular is reachable from a real code path:
// service.IOCopyBytesGracefully (used by chatCompletionsViaResponses) calls
// c.Writer.Flush() unconditionally after writing the body.
//
// Hijack, CloseNotify, and Pusher (also part of gin.ResponseWriter) are
// intentionally left promoted unchanged: no relay DoResponse /
// IOCopyBytesGracefully path for the 4 in-scope response shapes calls them
// (they exist for websocket upgrade, HTTP/2 push, and client-disconnect
// detection).
type outputFilterWriter struct {
	gin.ResponseWriter
	c      *gin.Context // for ContextKeyOutputFilterViolations / ContextKeyOutputFilterBufferOverflow
	shape  kids.ResponseShape
	filter kids.OutputFilter
	buf    bytes.Buffer

	statusCode int
	finalized  bool // guards finalize() against double invocation (defer + explicit restore())

	// wrote becomes true on the first Write / WriteString / WriteHeader call.
	// It distinguishes a pre-response error return (no response was ever
	// produced) from a real response that must be filtered. finalize() is a
	// no-op when wrote is false — see step 0 there.
	wrote bool

	// overflow becomes true once a Write/WriteString call would push buf past
	// maxOutputFilterBufferBytes. buf is filled up to the cap and the excess
	// is silently discarded; Write/WriteString still report success so the
	// caller completes normally. finalize() treats an overflowed 2xx response
	// as unverifiable (fails closed, like ExtractText/ExtractStreamText
	// returning ok=false) and an overflowed non-2xx response as
	// pass-through-truncated.
	overflow bool
}

func (w *outputFilterWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wrote = true
}

// WriteHeaderNow is a no-op: headers are only sent from finalize(), once the
// final (possibly replaced) body and Content-Length are known.
func (w *outputFilterWriter) WriteHeaderNow() {}

func (w *outputFilterWriter) Write(p []byte) (int, error) {
	w.wrote = true
	if w.overflow {
		return len(p), nil
	}
	if remaining := maxOutputFilterBufferBytes - w.buf.Len(); len(p) > remaining {
		w.buf.Write(p[:remaining])
		w.overflow = true
		return len(p), nil
	}
	return w.buf.Write(p)
}

func (w *outputFilterWriter) WriteString(s string) (int, error) {
	w.wrote = true
	if w.overflow {
		return len(s), nil
	}
	if remaining := maxOutputFilterBufferBytes - w.buf.Len(); len(s) > remaining {
		w.buf.WriteString(s[:remaining])
		w.overflow = true
		return len(s), nil
	}
	return w.buf.WriteString(s)
}

// Flush is a no-op: flushing here would send headers and/or a partial body
// before finalize() has classified the response.
func (w *outputFilterWriter) Flush() {}

func (w *outputFilterWriter) Written() bool { return w.wrote }

func (w *outputFilterWriter) Size() int { return w.buf.Len() }

func (w *outputFilterWriter) Status() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

// finalize classifies the buffered response (if any) and writes the
// (possibly replaced) status/headers/body to the underlying
// gin.ResponseWriter. See design doc §5.3/§5.4 for the full rationale.
func (w *outputFilterWriter) finalize() {
	if w.finalized {
		return
	}
	w.finalized = true

	// Step 0: nothing was ever written through w — a pre-response error
	// return, not a response to filter. Write nothing; the handler's own
	// error-response path (writing to orig, not w) is unaffected.
	if !w.wrote {
		return
	}

	raw := w.buf.Bytes()
	status := w.statusCode
	if status == 0 {
		status = http.StatusOK
	}

	// Step 1: an empty body (e.g. 204 No Content, or a 2xx response with no
	// body) cannot contain blocked content — pass the status through without
	// triggering fail-closed fallback. WriteHeaderNow is required here
	// because gin's WriteHeader only records the status; it is normally
	// flushed by the first Write, which an empty body never makes.
	if len(raw) == 0 {
		if w.statusCode != 0 {
			w.ResponseWriter.WriteHeader(w.statusCode)
			w.ResponseWriter.WriteHeaderNow()
		}
		return
	}

	sf := kids.FilterForShape(w.shape)
	isStream := strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream")

	out := raw
	switch {
	case status >= 200 && status < 300:
		var text string
		var ok bool
		switch {
		case w.overflow:
			// Truncated body: cannot reliably extract/replace text. Treat
			// like a parse failure and fail closed.
			ok = false
			common.SetContextKey(w.c, constant.ContextKeyOutputFilterBufferOverflow, true)
		case isStream:
			text, ok = sf.ExtractStreamText(raw)
		default:
			text, ok = sf.ExtractText(raw)
		}

		// !ok (parse failure or buffer overflow) fails closed, same as a
		// Blocked verdict. ok=true + text=="" (e.g. a tool_calls-only
		// response) is a recognised clean case and does NOT fail closed.
		blocked := !ok
		if ok {
			if verdict := w.filter.Check(text); verdict.Blocked {
				blocked = true
				common.SetContextKey(w.c, constant.ContextKeyOutputFilterViolations, verdict.Categories)
			}
		}

		if blocked {
			switch {
			case isStream:
				out = sf.BuildFallbackStream(kids.SafeFallbackText())
			case ok:
				if replaced, err := sf.ReplaceText(raw, kids.SafeFallbackText()); err == nil {
					out = replaced
				} else {
					out = sf.BuildFallbackBody(kids.SafeFallbackText())
				}
			default:
				out = sf.BuildFallbackBody(kids.SafeFallbackText())
			}
		}
	case w.overflow:
		// Non-2xx error body, truncated by the buffer cap: pass the
		// truncated bytes through (still more useful for ops debugging than
		// no body), but record that it was truncated.
		common.SetContextKey(w.c, constant.ContextKeyOutputFilterBufferOverflow, true)
	}
	// Other non-2xx responses pass through unchanged — not subject to the
	// output filter, so upstream error messages remain legible for ops.

	if !isStream {
		w.Header().Set("Content-Length", strconv.Itoa(len(out)))
	}
	if w.statusCode != 0 {
		w.ResponseWriter.WriteHeader(w.statusCode)
	}
	w.ResponseWriter.Write(out)
}
