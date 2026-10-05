package ratio_setting

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The channel Quick Import dialog (web/default/.../provider-presets.ts) creates
// channels straight from these lists. A preset model with no price imports
// fine and then fails every request with 价格未配置 — so every model, and every
// testModel, must be priced here.
var (
	presetModelsRe    = regexp.MustCompile(`models:\s*'([^']+)'`)
	presetTestModelRe = regexp.MustCompile(`testModel:\s*'([^']+)'`)
)

func TestQuickImportPresetModelsArePriced(t *testing.T) {
	InitRatioSettings()

	raw, err := os.ReadFile("../../web/default/src/features/channels/lib/provider-presets.ts")
	if err != nil {
		t.Fatalf("reading provider presets: %v", err)
	}
	src := string(raw)

	names := map[string]bool{}
	for _, m := range presetModelsRe.FindAllStringSubmatch(src, -1) {
		for _, name := range strings.Split(m[1], ",") {
			names[strings.TrimSpace(name)] = true
		}
	}
	for _, m := range presetTestModelRe.FindAllStringSubmatch(src, -1) {
		names[m[1]] = true
	}
	if len(names) < 10 {
		t.Fatalf("parsed only %d preset models — the parser or the file layout changed", len(names))
	}

	var unpriced []string
	for name := range names {
		if _, found, _ := GetModelRatio(name); found {
			continue
		}
		if _, found := GetModelPrice(name, false); found {
			continue
		}
		unpriced = append(unpriced, name)
	}
	sort.Strings(unpriced)
	if len(unpriced) > 0 {
		t.Errorf("Quick Import presets list unpriced models (import works, every request then fails with 价格未配置): %v", unpriced)
	}
}
