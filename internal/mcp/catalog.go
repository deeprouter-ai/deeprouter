package mcp

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/internal/keypurpose"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// apiBase is the host every API path below is called on.
const apiBase = "https://api.deeprouter.co"

// What a model is for, the way list_models groups and filters models.
const (
	purposeChat          = "chat"
	purposeImage         = "image"
	purposeVideo         = "video"
	purposeSpeech        = "speech"
	purposeTranscription = "transcription"
	purposeEmbedding     = "embedding"
	purposeRerank        = "rerank"
	purposeUnknown       = "unknown"
)

// purposeOrder is the order models are listed in.
var purposeOrder = []string{purposeChat, purposeImage, purposeVideo, purposeSpeech, purposeTranscription, purposeEmbedding, purposeRerank, purposeUnknown}

const bearerAuth = "Authorization: Bearer <your DeepRouter key>"

// recipe is how to call one kind of endpoint, written for the AI making the
// call. The video steps and the speech voice rule are the ones the
// paste-to-AI prompts were verified with (web/default/src/features/video and
// features/simple). "unknown" means nobody has written it down - never a guess.
type recipe struct {
	purpose  string
	auth     string
	params   string
	example  string // a minimal body; {model} stands for the model id
	response string
	steps    []string // the asynchronous flow, for video
}

// videoSteps is the flow after submitting a video task, whichever endpoint
// took it.
var videoSteps = []string{
	"Poll: GET " + apiBase + "/v1/videos/{task_id} every 10 seconds until status is \"completed\"; on \"failed\", tell the user what error.message says and stop.",
	"Download: GET " + apiBase + "/v1/videos/{task_id}/content and save it as an .mp4 file.",
}

var recipes = map[constant.EndpointType]recipe{
	constant.EndpointTypeOpenAI: {
		purpose:  purposeChat,
		auth:     bearerAuth,
		params:   "model, messages",
		example:  `{"model":"{model}","messages":[{"role":"user","content":"Hello"}]}`,
		response: `choices[0].message.content; send "stream": true for server-sent events`,
	},
	constant.EndpointTypeOpenAIResponse: {
		purpose:  purposeChat,
		auth:     bearerAuth,
		params:   "model, input",
		example:  `{"model":"{model}","input":"Hello"}`,
		response: "OpenAI Responses API: the text is in output[].content[] entries of type output_text",
	},
	constant.EndpointTypeOpenAIResponseCompact: {
		purpose:  purposeChat,
		auth:     bearerAuth,
		params:   purposeUnknown,
		response: purposeUnknown,
	},
	constant.EndpointTypeAnthropic: {
		purpose:  purposeChat,
		auth:     "x-api-key: <your DeepRouter key> (Authorization: Bearer is accepted too)",
		params:   "model, max_tokens, messages",
		example:  `{"model":"{model}","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}`,
		response: "Anthropic Messages API: content[0].text",
	},
	constant.EndpointTypeGemini: {
		purpose:  purposeChat,
		auth:     "x-goog-api-key: <your DeepRouter key>",
		params:   "contents (the model is part of the URL)",
		example:  `{"contents":[{"parts":[{"text":"Hello"}]}]}`,
		response: "Gemini API: candidates[0].content.parts[0].text",
	},
	constant.EndpointTypeImageGeneration: {
		purpose:  purposeImage,
		auth:     bearerAuth,
		params:   "model, prompt",
		example:  `{"model":"{model}","prompt":"a lighthouse at sunrise, watercolour"}`,
		response: "data[0].url (download it) or data[0].b64_json (decode it)",
	},
	keypurpose.VideoGeneration: {
		purpose:  purposeVideo,
		auth:     bearerAuth,
		params:   `model, prompt; optional "duration" in seconds and model-specific fields such as "size"`,
		example:  `{"model":"{model}","prompt":"waves breaking on rocks at sunrise"}`,
		response: "id is the task id; the video itself comes from the steps",
		steps:    videoSteps,
	},
	constant.EndpointTypeOpenAIVideo: {
		purpose:  purposeVideo,
		auth:     bearerAuth,
		params:   `model, prompt; optional "seconds" and "size"`,
		example:  `{"model":"{model}","prompt":"waves breaking on rocks at sunrise"}`,
		response: "id is the task id; the video itself comes from the steps",
		steps:    videoSteps,
	},
	constant.EndpointTypeAudioSpeech: {
		purpose:  purposeSpeech,
		auth:     bearerAuth,
		params:   `model, input; models whose id starts with gpt- or tts- also need "voice" (for example "alloy"), other providers use their own voice ids`,
		example:  `{"model":"{model}","input":"Hello, and welcome.","response_format":"mp3"}`,
		response: "the response body is the audio file",
	},
	keypurpose.AudioTranscription: {
		purpose:  purposeTranscription,
		auth:     bearerAuth,
		params:   "multipart/form-data with file (the audio) and model",
		response: "text",
	},
	constant.EndpointTypeEmbeddings: {
		purpose:  purposeEmbedding,
		auth:     bearerAuth,
		params:   "model, input",
		example:  `{"model":"{model}","input":"The quick brown fox"}`,
		response: "data[0].embedding",
	},
	constant.EndpointTypeJinaRerank: {
		purpose:  purposeRerank,
		auth:     bearerAuth,
		params:   "model, query, documents",
		example:  `{"model":"{model}","query":"Which one is about cats?","documents":["A cat sleeps.","A car drives."]}`,
		response: "results[].index and results[].relevance_score",
	},
}

// autoModelNote explains the smart-routing alias wherever it appears.
const autoModelNote = "Smart routing for chat: DeepRouter picks a suitable model for each request. Chat only - never use it for images, video or audio."

// endpointDetail is how to call one model on one of its endpoints.
type endpointDetail struct {
	Type     string   `json:"type" jsonschema:"the supported_endpoint_types value this describes"`
	Purpose  string   `json:"purpose"`
	Method   string   `json:"method"`
	URL      string   `json:"url"`
	Auth     string   `json:"auth"`
	Params   string   `json:"required_params"`
	Example  string   `json:"body_example,omitempty"`
	Response string   `json:"response"`
	Steps    []string `json:"async_steps,omitempty" jsonschema:"for video: what to do after submitting"`
}

// endpointsOf describes every endpoint a model supports, the gateway's
// preferred one first.
func endpointsOf(modelID string, types []constant.EndpointType) []endpointDetail {
	paths := model.GetSupportedEndpointMap()
	details := make([]endpointDetail, 0, len(types))
	for _, t := range types {
		info, ok := paths[string(t)]
		if !ok {
			info, ok = common.GetDefaultEndpointInfo(t)
		}
		r, known := recipes[t]
		if !known {
			r = recipe{purpose: purposeUnknown, auth: bearerAuth, params: purposeUnknown, response: purposeUnknown}
		}
		d := endpointDetail{
			Type:     string(t),
			Purpose:  r.purpose,
			Method:   info.Method,
			URL:      apiBase + strings.ReplaceAll(info.Path, "{model}", modelID),
			Auth:     r.auth,
			Params:   r.params,
			Example:  strings.ReplaceAll(r.example, "{model}", modelID),
			Response: r.response,
			Steps:    r.steps,
		}
		if !ok {
			d.Method, d.URL = purposeUnknown, purposeUnknown
		}
		details = append(details, d)
	}
	return details
}

// purposesOf is what a model is for, from its endpoints, in the gateway's
// order: the first endpoint is the one it prefers, so an image model served
// by an OpenAI channel is "image" first and "chat" second.
func purposesOf(types []constant.EndpointType) []string {
	purposes := make([]string, 0, len(types))
	for _, t := range types {
		if r, ok := recipes[t]; ok && !common.StringsContains(purposes, r.purpose) {
			purposes = append(purposes, r.purpose)
		}
	}
	if len(purposes) == 0 {
		purposes = append(purposes, purposeUnknown)
	}
	return purposes
}

// isAsync reports whether calling the model means submitting a task and
// coming back for the result.
func isAsync(types []constant.EndpointType) bool {
	for _, t := range types {
		if len(recipes[t].steps) > 0 {
			return true
		}
	}
	return false
}

// priceDetail is what a model costs, in words and in numbers. Prices are in
// US dollars of credit, after the group ratio.
type priceDetail struct {
	Unit       string   `json:"billing_unit" jsonschema:"per_1m_tokens, per_request, expression or unknown"`
	Line       string   `json:"price"`
	InputUSD   *float64 `json:"input_usd_per_1m_tokens,omitempty"`
	OutputUSD  *float64 `json:"output_usd_per_1m_tokens,omitempty"`
	RequestUSD *float64 `json:"usd_per_request,omitempty"`
	Expression string   `json:"billing_expression,omitempty"`
	GroupRatio float64  `json:"group_ratio"`
	Note       string   `json:"price_note,omitempty"`
}

// scaledNote is said about every per-request price of a media model: the
// gateway multiplies it by what the request asks for (relay/channel/task/*
// EstimateBilling), and which factors apply is the provider adapter's
// business, not something the price table records.
const scaledNote = "This is the base price per request. For some providers the gateway multiplies it by what the request asks for - clip length in seconds, resolution, number of images - so tell the user the expected cost before generating, and treat the bill as final."

// describePrice turns a pricing entry into what an AI can tell a user.
// configured says whether the model has a price of its own; without one the
// catalog carries the platform's fallback ratio, which is not a price anyone
// set. from marks a price that differs by group and shows the cheapest.
func describePrice(p *model.Pricing, configured bool, purposes []string, ratio float64, from bool) priceDetail {
	d := priceDetail{GroupRatio: ratio}
	if p == nil || !configured {
		d.Unit = "unknown"
		d.Line = "unknown: no price is configured for this model"
		return d
	}
	prefix := ""
	if from {
		prefix = "from "
	}
	switch {
	case p.BillingMode == billing_setting.BillingModeTieredExpr && strings.TrimSpace(p.BillingExpr) != "":
		d.Unit = "expression"
		d.Expression = p.BillingExpr
		d.Line = "billed by a pricing expression whose coefficients are USD per 1M tokens: " + p.BillingExpr
		if ratio != 1 {
			d.Line += ", times the group ratio " + number(ratio)
		}
	case p.QuotaType == 1:
		perRequest := round(p.ModelPrice * ratio)
		d.Unit = "per_request"
		d.RequestUSD = &perRequest
		d.Line = prefix + usd(perRequest) + " per request"
		if isMedia(purposes) {
			d.Line += " (base price, see price_note)"
			d.Note = scaledNote
		}
	default:
		input := round(p.ModelRatio * 2 * ratio)
		output := round(p.ModelRatio * 2 * ratio * p.CompletionRatio)
		d.Unit = "per_1m_tokens"
		d.InputUSD, d.OutputUSD = &input, &output
		d.Line = prefix + usd(input) + " per 1M input tokens, " + usd(output) + " per 1M output tokens"
	}
	return d
}

// isMedia reports whether a model makes images, video or audio.
func isMedia(purposes []string) bool {
	for _, p := range purposes {
		switch p {
		case purposeImage, purposeVideo, purposeSpeech, purposeTranscription:
			return true
		}
	}
	return false
}

// round keeps a dollar amount to the gateway's precision (one quota unit is $0.000002).
func round(v float64) float64 { return math.Round(v*1e6) / 1e6 }

// number writes a rounded value without trailing zeros.
func number(v float64) string { return strconv.FormatFloat(round(v), 'f', -1, 64) }

// usd writes a dollar amount the way the answers quote prices.
func usd(v float64) string { return "$" + number(v) }

// pricedCatalog is the pricing catalog exactly as the website shows it to one
// viewer - a visitor, or the account behind a key - and the group the prices
// are for.
type pricedCatalog struct {
	byName     map[string]*model.Pricing
	groupRatio map[string]float64
	autoGroups []string
	using      string // the key's group; "" for a visitor
	userGroup  string
}

// pricing fetches the catalog for keys - nil for a visitor, or the key's
// authenticated context - by running the /api/pricing handler itself.
func (c *caller) pricing(keys map[string]any) (*pricedCatalog, error) {
	raw, err := c.catalog(keys)
	if err != nil {
		return nil, err
	}
	pc := &pricedCatalog{byName: map[string]*model.Pricing{}, groupRatio: raw.GroupRatio, autoGroups: raw.AutoGroups}
	for i := range raw.Models {
		pc.byName[raw.Models[i].ModelName] = &raw.Models[i]
	}
	pc.using, _ = keys[string(constant.ContextKeyUsingGroup)].(string)
	pc.userGroup, _ = keys[string(constant.ContextKeyUserGroup)].(string)
	return pc, nil
}

// ratioFor is the group ratio a model is priced at for this viewer. A visitor
// sees the cheapest group the model is sold in, as the pricing page does; a
// key pays its own group's ratio, or for an "auto" key the cheapest of the
// groups it may fall back to.
func (pc *pricedCatalog) ratioFor(p *model.Pricing) (ratio float64, from bool) {
	if pc.using != "" && pc.using != "auto" {
		if r, ok := pc.groupRatio[pc.using]; ok {
			return r, false
		}
		return service.GetUserGroupRatio(pc.userGroup, pc.using), false
	}
	groups := pc.autoGroups
	if pc.using == "" {
		groups = nil
		for g := range pc.groupRatio {
			groups = append(groups, g)
		}
	}
	// A model priced from the settings (see priceOf) carries no groups; then
	// every group in reach counts.
	var enabled []string
	if p != nil {
		enabled = p.EnableGroup
	}
	ratios := map[float64]bool{}
	for _, g := range groups {
		r, ok := pc.groupRatio[g]
		if ok && (len(enabled) == 0 || common.StringsContains(enabled, g) || common.StringsContains(enabled, "all")) {
			ratios[r] = true
		}
	}
	if len(ratios) == 0 {
		return 1, false
	}
	keys := make([]float64, 0, len(ratios))
	for r := range ratios {
		keys = append(keys, r)
	}
	sort.Float64s(keys)
	return keys[0], len(keys) > 1
}

// priceOf describes what a model costs this viewer. A model a key can call
// can still be missing from the catalog - the catalog leaves out a model whose
// entry on the Models page is switched off, the relay does not - so it is then
// priced from the settings the relay bills by.
func (pc *pricedCatalog) priceOf(id string, purposes []string) priceDetail {
	p := pc.byName[id]
	configured := helper.HasModelBillingConfig(id)
	if p == nil && configured {
		p = pricingFromSettings(id)
	}
	ratio, from := pc.ratioFor(p)
	return describePrice(p, configured, purposes, ratio, from)
}

// pricingFromSettings reads a model's price the way model.updatePricing fills
// a catalog entry: a fixed price per request if one is set, otherwise the
// token ratios, and a pricing expression when the model is billed by one.
func pricingFromSettings(id string) *model.Pricing {
	p := &model.Pricing{ModelName: id}
	if price, ok := ratio_setting.GetModelPrice(id, false); ok {
		p.ModelPrice, p.QuotaType = price, 1
	} else {
		p.ModelRatio, _, _ = ratio_setting.GetModelRatio(id)
		p.CompletionRatio = ratio_setting.GetCompletionRatio(id)
	}
	if billing_setting.GetBillingMode(id) == billing_setting.BillingModeTieredExpr {
		if expr, ok := billing_setting.GetBillingExpr(id); ok {
			p.BillingMode, p.BillingExpr = billing_setting.BillingModeTieredExpr, expr
		}
	}
	return p
}

// rawCatalog is the part of the /api/pricing answer the tools read.
type rawCatalog struct {
	Models     []model.Pricing    `json:"data"`
	GroupRatio map[string]float64 `json:"group_ratio"`
	AutoGroups []string           `json:"auto_groups"`
}

// catalog runs the /api/pricing handler for keys (nil for a visitor).
func (c *caller) catalog(keys map[string]any) (*rawCatalog, error) {
	ctx, out := c.shadow(keys)
	controller.GetPricing(ctx)
	var answer struct {
		Success    bool               `json:"success"`
		Models     []model.Pricing    `json:"data"`
		GroupRatio map[string]float64 `json:"group_ratio"`
		AutoGroups []string           `json:"auto_groups"`
	}
	if err := common.Unmarshal(out.Bytes(), &answer); err != nil {
		return nil, err
	}
	if !answer.Success {
		return nil, fmt.Errorf("the pricing catalog is unavailable")
	}
	return &rawCatalog{Models: answer.Models, GroupRatio: answer.GroupRatio, AutoGroups: answer.AutoGroups}, nil
}
