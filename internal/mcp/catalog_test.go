package mcp

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/internal/keypurpose"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"

	"github.com/stretchr/testify/require"
)

// TestPurposesOf checks that a model's purposes come from its endpoints, in
// listing order, and that an endpoint nobody described is "unknown".
func TestPurposesOf(t *testing.T) {
	require.Equal(t, []string{purposeChat}, purposesOf([]constant.EndpointType{constant.EndpointTypeAnthropic, constant.EndpointTypeOpenAI}))
	// The gateway's preferred endpoint comes first, and so does its purpose.
	require.Equal(t, []string{purposeImage, purposeChat}, purposesOf([]constant.EndpointType{constant.EndpointTypeImageGeneration, constant.EndpointTypeOpenAI}))
	require.Equal(t, []string{purposeVideo}, purposesOf([]constant.EndpointType{keypurpose.VideoGeneration}))
	require.Equal(t, []string{purposeSpeech}, purposesOf([]constant.EndpointType{constant.EndpointTypeAudioSpeech}))
	require.Equal(t, []string{purposeTranscription}, purposesOf([]constant.EndpointType{keypurpose.AudioTranscription}))
	require.Equal(t, []string{purposeUnknown}, purposesOf(nil))
	require.Equal(t, []string{purposeUnknown}, purposesOf([]constant.EndpointType{"made-up"}))
	require.True(t, isAsync([]constant.EndpointType{constant.EndpointTypeOpenAIVideo}))
	require.False(t, isAsync([]constant.EndpointType{constant.EndpointTypeOpenAI}))
}

// TestEndpointsOf checks the full URL an AI is told to call, with the model
// put into a path that carries it, and that an endpoint without a recipe says
// "unknown" rather than inventing parameters.
func TestEndpointsOf(t *testing.T) {
	gemini := endpointsOf("gemini-2.5-pro", []constant.EndpointType{constant.EndpointTypeGemini})
	require.Len(t, gemini, 1)
	require.Equal(t, "POST", gemini[0].Method)
	require.Equal(t, apiBase+"/v1beta/models/gemini-2.5-pro:generateContent", gemini[0].URL)

	video := endpointsOf("MiniMax-H3", []constant.EndpointType{keypurpose.VideoGeneration})
	require.Equal(t, apiBase+"/v1/video/generations", video[0].URL)
	require.Contains(t, video[0].Example, `"model":"MiniMax-H3"`)
	require.Len(t, video[0].Steps, 2)
	require.Contains(t, video[0].Steps[0], "/v1/videos/{task_id}")
	require.Contains(t, video[0].Steps[1], "/v1/videos/{task_id}/content")

	custom := endpointsOf("x", []constant.EndpointType{"made-up"})
	require.Equal(t, purposeUnknown, custom[0].URL)
	require.Equal(t, purposeUnknown, custom[0].Params)
	require.Equal(t, purposeUnknown, custom[0].Purpose)
}

// TestDescribePrice checks the arithmetic the pricing page uses (ratio x 2
// dollars per 1M tokens, times the group ratio), the base-price note on media,
// and that a model with no price of its own is "unknown", not the platform's
// fallback ratio passed off as a price.
func TestDescribePrice(t *testing.T) {
	chat := []string{purposeChat}
	tokens := &model.Pricing{ModelName: "m", QuotaType: 0, ModelRatio: 1.5, CompletionRatio: 4}
	d := describePrice(tokens, true, chat, 1.5, false)
	require.Equal(t, "per_1m_tokens", d.Unit)
	require.Equal(t, 4.5, *d.InputUSD)
	require.Equal(t, 18.0, *d.OutputUSD)
	require.Equal(t, 1.5, d.GroupRatio)
	require.Equal(t, "$4.5 per 1M input tokens, $18 per 1M output tokens", d.Line)
	require.Equal(t, "from $3 per 1M input tokens, $12 per 1M output tokens", describePrice(tokens, true, chat, 1, true).Line)

	video := describePrice(&model.Pricing{QuotaType: 1, ModelPrice: 0.13}, true, []string{purposeVideo}, 1, false)
	require.Equal(t, "per_request", video.Unit)
	require.Equal(t, 0.13, *video.RequestUSD)
	require.Equal(t, "$0.13 per request (base price, see price_note)", video.Line)
	require.Equal(t, scaledNote, video.Note)

	perCall := describePrice(&model.Pricing{QuotaType: 1, ModelPrice: 0.01}, true, chat, 2, false)
	require.Equal(t, "$0.02 per request", perCall.Line)
	require.Empty(t, perCall.Note)

	expr := describePrice(&model.Pricing{BillingMode: billing_setting.BillingModeTieredExpr, BillingExpr: "p * 2.5 + c * 10"}, true, chat, 1.5, false)
	require.Equal(t, "expression", expr.Unit)
	require.Equal(t, "p * 2.5 + c * 10", expr.Expression)
	require.Contains(t, expr.Line, "p * 2.5 + c * 10")
	require.Contains(t, expr.Line, "times the group ratio 1.5")

	require.Equal(t, "unknown", describePrice(tokens, false, chat, 1, false).Unit)
	require.Equal(t, "unknown", describePrice(nil, true, chat, 1, false).Unit)
}

// TestRatioFor checks which group ratio a price is quoted at: a visitor sees
// the cheapest group selling the model (marked "from" when groups differ), a
// key pays its own group's, and an "auto" key the cheapest group it may fall
// back to that sells the model.
func TestRatioFor(t *testing.T) {
	ratios := map[string]float64{"default": 1, "vip": 0.8, "enterprise": 1.2}
	both := &model.Pricing{EnableGroup: []string{"default", "vip"}}
	onlyDefault := &model.Pricing{EnableGroup: []string{"default"}}

	visitor := &pricedCatalog{groupRatio: ratios}
	r, from := visitor.ratioFor(both)
	require.Equal(t, 0.8, r)
	require.True(t, from)
	r, from = visitor.ratioFor(onlyDefault)
	require.Equal(t, 1.0, r)
	require.False(t, from)

	key := &pricedCatalog{groupRatio: ratios, using: "enterprise", userGroup: "default"}
	r, from = key.ratioFor(both)
	require.Equal(t, 1.2, r)
	require.False(t, from)

	auto := &pricedCatalog{groupRatio: ratios, autoGroups: []string{"default", "vip"}, using: "auto"}
	r, from = auto.ratioFor(both)
	require.Equal(t, 0.8, r)
	require.True(t, from)
	r, from = auto.ratioFor(onlyDefault)
	require.Equal(t, 1.0, r)
	require.False(t, from)
	// Priced from the settings, a model carries no groups: every group in
	// reach counts.
	r, from = auto.ratioFor(&model.Pricing{})
	require.Equal(t, 0.8, r)
	require.True(t, from)
}

// TestTokenStatus checks that each reason a key cannot be used gets its own
// name, and that being switched off wins over being out of date.
func TestTokenStatus(t *testing.T) {
	const now = 1_000_000
	ok := func() *model.Token {
		return &model.Token{Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	}
	require.Equal(t, statusOK, tokenStatus(ok(), now))

	tk := ok()
	tk.Status = common.TokenStatusDisabled
	tk.ExpiredTime = now - 1
	require.Equal(t, statusDisabled, tokenStatus(tk, now))

	tk = ok()
	tk.Status = common.TokenStatusExpired
	require.Equal(t, statusExpired, tokenStatus(tk, now))
	tk = ok()
	tk.ExpiredTime = now - 1
	require.Equal(t, statusExpired, tokenStatus(tk, now))
	tk.ExpiredTime = now + 1
	require.Equal(t, statusOK, tokenStatus(tk, now))

	tk = ok()
	tk.Status = common.TokenStatusExhausted
	require.Equal(t, statusExhausted, tokenStatus(tk, now))
	tk = ok()
	tk.UnlimitedQuota = false
	require.Equal(t, statusExhausted, tokenStatus(tk, now))
	tk.RemainQuota = 1
	require.Equal(t, statusOK, tokenStatus(tk, now))

	tk = ok()
	tk.Status = 99
	require.Equal(t, statusDisabled, tokenStatus(tk, now))
}

// TestBearerKey checks that the key is read out of the header the way
// middleware.TokenAuth reads it.
func TestBearerKey(t *testing.T) {
	require.Equal(t, "abc", bearerKey("Bearer sk-abc"))
	require.Equal(t, "abc", bearerKey("bearer abc"))
	require.Equal(t, "abc", bearerKey("sk-abc-12"))
	require.Equal(t, "", bearerKey(""))
	require.Equal(t, "", bearerKey("Bearer "))
}
