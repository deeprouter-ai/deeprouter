package hailuo

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// Every model advertised in ModelList is one an admin can enable on a MiniMax
// (type 35) channel. Video task submission prices via ModelPriceHelperPerCall →
// per-call price, then falls back to the model ratio; if neither is configured,
// the request is rejected with "价格未配置 / price not configured" whenever
// self-use mode is off. Same guard as doubao's TestDoubaoVideoModelListHasPricing.
func TestHailuoModelListHasPricing(t *testing.T) {
	ratio_setting.InitRatioSettings()

	var missing []string
	for _, name := range ModelList {
		if _, _, exist := ratio_setting.GetModelRatioOrPrice(name); !exist {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("%d Hailuo model(s) in ModelList have neither a model ratio nor "+
			"a per-call price (would fail with 价格未配置). Add them to "+
			"defaultModelPrice: %v", len(missing), missing)
	}
}

// The MiniMax-H3 row in defaultModelPrice is the 2K per-second rate that
// EstimateBilling's resolution ratio derives from; if the two drift apart,
// every H3 clip is billed at the wrong rate.
func TestH3PriceMatchesPerSecondRate(t *testing.T) {
	ratio_setting.InitRatioSettings()

	price, usePrice := ratio_setting.GetModelPrice("MiniMax-H3", false)
	if !usePrice {
		t.Fatal("MiniMax-H3 has no per-call price entry — v2 per-second billing bases on it")
	}
	if price != H3RatePerSecond2K {
		t.Errorf("defaultModelPrice[MiniMax-H3] = %v, want the 2K per-second rate %v "+
			"(hailuo/constants.go H3RatePerSecond2K)", price, H3RatePerSecond2K)
	}
}

// MiniMax-H3 capability parameters must match the intl API docs:
// 4–15s integer durations, 768P/2K resolutions, 2K default.
func TestH3ModelConfig(t *testing.T) {
	cfg := GetModelConfig("MiniMax-H3")

	if cfg.DefaultResolution != Resolution2K {
		t.Errorf("default resolution = %q, want %q", cfg.DefaultResolution, Resolution2K)
	}
	for d := H3MinDuration; d <= H3MaxDuration; d++ {
		if !containsInt(cfg.SupportedDurations, d) {
			t.Errorf("duration %d missing from SupportedDurations", d)
		}
	}
	if len(cfg.SupportedDurations) != H3MaxDuration-H3MinDuration+1 {
		t.Errorf("SupportedDurations has %d entries, want %d (4..15)",
			len(cfg.SupportedDurations), H3MaxDuration-H3MinDuration+1)
	}
	if !contains(cfg.SupportedResolutions, Resolution768P) || !contains(cfg.SupportedResolutions, Resolution2K) {
		t.Errorf("SupportedResolutions = %v, want 768P and 2K", cfg.SupportedResolutions)
	}
	if !IsV2Model("MiniMax-H3") {
		t.Error("MiniMax-H3 must be recognised as a v2 model")
	}
	if IsV2Model("MiniMax-Hailuo-2.3") {
		t.Error("MiniMax-Hailuo-2.3 must stay on the v1 API")
	}
}

// resolveV2DurationResolution clamps duration into the H3 range and maps size
// strings onto the two supported resolutions — these are the values billing uses.
func TestResolveV2DurationResolution(t *testing.T) {
	cfg := GetModelConfig("MiniMax-H3")
	cases := []struct {
		name           string
		duration       int
		size           string
		wantDuration   int
		wantResolution string
	}{
		{"defaults", 0, "", DefaultDuration, Resolution2K},
		{"clamp high", 30, "", H3MaxDuration, Resolution2K},
		{"clamp low", 1, "", H3MinDuration, Resolution2K},
		{"768 size", 6, "768", 6, Resolution768P},
		{"lowercase 2k", 10, "2k", 10, Resolution2K},
		{"unsupported size falls back", 6, "1080", 6, Resolution2K},
	}
	for _, tc := range cases {
		req := &relaycommon.TaskSubmitReq{Duration: tc.duration, Size: tc.size}
		gotDuration, gotResolution := resolveV2DurationResolution(req, cfg)
		if gotDuration != tc.wantDuration || gotResolution != tc.wantResolution {
			t.Errorf("%s: got (%d, %s), want (%d, %s)",
				tc.name, gotDuration, gotResolution, tc.wantDuration, tc.wantResolution)
		}
	}
}

// convertToV2RequestPayload: the prompt becomes a text content item, metadata
// extras pass through, but metadata must NOT change the billed duration or
// resolution (billing is estimated from the top-level fields beforehand).
func TestConvertToV2RequestPayload(t *testing.T) {
	a := &TaskAdaptor{}
	req := &relaycommon.TaskSubmitReq{
		Prompt:   "a cat surfing",
		Duration: 30, // clamps to 15
		Size:     "768",
		Metadata: map[string]any{"duration": 4, "resolution": "2K", "ratio": "9:16"},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "MiniMax-H3"}}

	got, err := a.convertToV2RequestPayload(req, info)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if got.Model != "MiniMax-H3" {
		t.Errorf("model = %q", got.Model)
	}
	if len(got.Content) != 1 || got.Content[0].Type != "text" || got.Content[0].Text != "a cat surfing" {
		t.Errorf("content = %+v, want a single text item with the prompt", got.Content)
	}
	if got.Duration == nil || *got.Duration != H3MaxDuration {
		t.Errorf("duration = %v, want clamped %d (metadata override must not win)", got.Duration, H3MaxDuration)
	}
	if got.Resolution != Resolution768P {
		t.Errorf("resolution = %q, want %q (metadata override must not win)", got.Resolution, Resolution768P)
	}
	if got.Ratio != "9:16" {
		t.Errorf("ratio = %q, want metadata passthrough 9:16", got.Ratio)
	}
}

// A first-frame image from the top-level fields becomes an image_url content item.
func TestConvertToV2RequestPayloadFirstFrame(t *testing.T) {
	a := &TaskAdaptor{}
	req := &relaycommon.TaskSubmitReq{
		Prompt: "zoom out",
		Images: []string{"https://example.com/frame.png"},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "MiniMax-H3"}}

	got, err := a.convertToV2RequestPayload(req, info)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if len(got.Content) != 2 {
		t.Fatalf("content has %d items, want text + image_url", len(got.Content))
	}
	img := got.Content[1]
	if img.Type != "image_url" || img.Role != "first_frame" || img.ImageURL == nil || img.ImageURL.URL != "https://example.com/frame.png" {
		t.Errorf("image item = %+v", img)
	}
}

// ParseTaskResult must understand both response generations: the v2
// {"task":{...}} wrapper (URL delivered inline) and the v1 base_resp shape.
func TestParseTaskResultV2(t *testing.T) {
	a := &TaskAdaptor{}

	succeeded := []byte(`{"task":{"id":"424","status":"succeeded","content":{"url":"https://cdn.example.com/out.mp4"}}}`)
	ti, err := a.ParseTaskResult(succeeded)
	if err != nil {
		t.Fatalf("parse succeeded: %v", err)
	}
	if ti.Status != model.TaskStatusSuccess {
		t.Errorf("status = %v, want %v", ti.Status, model.TaskStatusSuccess)
	}
	if ti.Url != "https://cdn.example.com/out.mp4" {
		t.Errorf("url = %q, want the inline content.url", ti.Url)
	}
	if ti.Progress != "100%" {
		t.Errorf("progress = %q, want 100%%", ti.Progress)
	}

	failed := []byte(`{"task":{"id":"424","status":"failed"}}`)
	ti, err = a.ParseTaskResult(failed)
	if err != nil {
		t.Fatalf("parse failed-status: %v", err)
	}
	if ti.Status != model.TaskStatusFailure || ti.Progress != "100%" || ti.Reason == "" {
		t.Errorf("failed task: status=%v progress=%q reason=%q, want terminal failure with a reason",
			ti.Status, ti.Progress, ti.Reason)
	}

	// v1 regression: the old shape must keep parsing (in-progress avoids the
	// file-retrieve HTTP call the v1 success path makes).
	v1Body := []byte(`{"task_id":"abc","status":"Processing","base_resp":{"status_code":0}}`)
	ti, err = a.ParseTaskResult(v1Body)
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	if ti.Progress != "50%" {
		t.Errorf("v1 Processing progress = %q, want 50%%", ti.Progress)
	}
}
