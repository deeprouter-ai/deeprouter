// Package alias_setting maps DeepRouter Simple-mode (purpose, brand) bindings
// to concrete upstream models, and exposes the "purpose card" metadata used
// by the API Key create UI (PRD docs/tasks/api-key-simple-advanced-prd.md).
//
// Data is seeded from the embedded YAML at data/aliases.yaml. Phase 1 ships
// YAML-only (rebuild required to change); admin-UI overrides are deferred
// to Phase 3.
package alias_setting

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"gopkg.in/yaml.v3"
)

//go:embed seed/aliases.yaml
var seedYAML []byte

// PurposeInfo is the per-card payload returned to the frontend. The label /
// desc / human_estimate fields are stored per-language; the controller picks
// the right one based on the request's resolved language.
type PurposeInfo struct {
	ID               string   `yaml:"id"`
	LabelEn          string   `yaml:"label_en"`
	LabelZh          string   `yaml:"label_zh"`
	Icon             string   `yaml:"icon"`
	DescEn           string   `yaml:"desc_en"`
	DescZh           string   `yaml:"desc_zh"`
	HumanEstimateEn  string   `yaml:"human_estimate_en"`
	HumanEstimateZh  string   `yaml:"human_estimate_zh"`
	PriceRange       string   `yaml:"price_range"`
	RecommendedBrand string   `yaml:"recommended_brand"`
	AvailableBrands  []string `yaml:"available_brands"`
	ModelWhitelist   []string `yaml:"model_whitelist"`
}

// PriceTierInfo describes the 4 caps available when purpose = "all".
type PriceTierInfo struct {
	ID              string   `yaml:"id"`
	LabelEn         string   `yaml:"label_en"`
	LabelZh         string   `yaml:"label_zh"`
	DescEn          string   `yaml:"desc_en"`
	DescZh          string   `yaml:"desc_zh"`
	PriceRange      string   `yaml:"price_range"`
	IsDefault       bool     `yaml:"is_default"`
	RequiresConfirm bool     `yaml:"requires_confirm"`
	ModelWhitelist  []string `yaml:"model_whitelist"`
}

type aliasEntry struct {
	Purpose string `yaml:"purpose"`
	Brand   string `yaml:"brand"`
	Target  string `yaml:"target"`
}

type seedFile struct {
	Purposes      []PurposeInfo            `yaml:"purposes"`
	PriceTiers    map[string]PriceTierInfo `yaml:"price_tiers"`
	Aliases       []aliasEntry             `yaml:"aliases"`
	VirtualModels []string                 `yaml:"virtual_models"`
}

// PurposeSummary is the API response shape for GET /api/user/self/purposes.
// It collapses the per-language strings down to the caller's language.
type PurposeSummary struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Icon             string   `json:"icon"`
	Desc             string   `json:"desc"`
	HumanEstimate    string   `json:"human_estimate"`
	PriceRange       string   `json:"price_range"`
	RecommendedBrand string   `json:"recommended_brand"`
	AvailableBrands  []string `json:"available_brands"`
}

// PriceTierSummary is the API response shape for a single price tier.
type PriceTierSummary struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Desc            string `json:"desc"`
	PriceRange      string `json:"price_range"`
	IsDefault       bool   `json:"is_default"`
	RequiresConfirm bool   `json:"requires_confirm"`
}

var (
	mu                sync.RWMutex
	purposes          []PurposeInfo
	purposesByID      map[string]*PurposeInfo
	priceTiers        map[string]PriceTierInfo
	aliasMap          map[string]map[string]string // purpose → brand → target
	virtualModels     map[string]struct{}
	defaultTierID     = "standard"
	priceTierOrder    = []string{"economy", "standard", "premium", "ultra"}
)

// InitAliasSettings parses the embedded YAML into in-memory lookup tables.
// Idempotent; safe to call once at boot from main.go InitResources().
func InitAliasSettings() error {
	var seed seedFile
	if err := yaml.Unmarshal(seedYAML, &seed); err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	purposes = seed.Purposes
	purposesByID = make(map[string]*PurposeInfo, len(seed.Purposes))
	for i := range purposes {
		purposesByID[purposes[i].ID] = &purposes[i]
	}

	priceTiers = make(map[string]PriceTierInfo, len(seed.PriceTiers))
	for id, info := range seed.PriceTiers {
		info.ID = id
		priceTiers[id] = info
		if info.IsDefault {
			defaultTierID = id
		}
	}

	aliasMap = make(map[string]map[string]string)
	for _, a := range seed.Aliases {
		if _, ok := aliasMap[a.Purpose]; !ok {
			aliasMap[a.Purpose] = make(map[string]string)
		}
		aliasMap[a.Purpose][a.Brand] = a.Target
	}

	virtualModels = make(map[string]struct{}, len(seed.VirtualModels))
	for _, m := range seed.VirtualModels {
		virtualModels[m] = struct{}{}
	}

	common.SysLog("alias_setting initialized: " +
		itoa(len(purposes)) + " purposes, " +
		itoa(len(seed.Aliases)) + " aliases, " +
		itoa(len(virtualModels)) + " virtual models")
	return nil
}

// itoa is a tiny stringification helper used only for boot logging; avoids
// pulling strconv into the public surface.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// IsVirtualModel reports whether a model name should trigger Simple-mode
// alias resolution in the distribution middleware.
func IsVirtualModel(model string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := virtualModels[model]
	return ok
}

// ResolveAlias returns the concrete upstream model name for a (purpose, brand)
// pair. Falls back to (purpose, "auto") when the specific brand is missing.
// Returns empty string when no resolution exists (caller should treat as
// "no alias — use the model name as-is").
func ResolveAlias(purpose, brand string) string {
	mu.RLock()
	defer mu.RUnlock()

	byBrand, ok := aliasMap[purpose]
	if !ok {
		return ""
	}
	if brand != "" {
		if target, ok := byBrand[brand]; ok && target != "" {
			return target
		}
	}
	return byBrand["auto"]
}

// ResolveAliasForVirtualModel handles per-virtual-model overrides
// (e.g. "deeprouter-coding" forces purpose=coding regardless of token binding,
// so power users can mix tasks under one Auto key).
func ResolveAliasForVirtualModel(virtualModel, tokenPurpose, tokenBrand string) string {
	purpose := tokenPurpose
	switch virtualModel {
	case "deeprouter-chat":
		purpose = "chat"
	case "deeprouter-coding":
		purpose = "coding"
	case "deeprouter-image":
		purpose = "image"
	case "deeprouter-video":
		purpose = "video"
	case "deeprouter-voice":
		purpose = "voice"
	case "deeprouter-voice-tts":
		// Explicit TTS — resolved as voice but distributor may need to swap
		// whisper for tts-1-hd. For now, return whisper-1 fallback; channel
		// adapters handle the actual TTS endpoint routing.
		purpose = "voice"
	}
	if purpose == "" || purpose == "all" {
		// Auto purpose has no alias binding; caller should leave the model
		// name as-is so the user's explicit choice routes through normally.
		return ""
	}
	return ResolveAlias(purpose, tokenBrand)
}

// ModelWhitelistForToken returns the comma-joined model_limits string for
// a freshly-created Simple-mode token. controller.AddToken writes this into
// Token.ModelLimits + sets ModelLimitsEnabled=true.
//
//   - When purpose != "all": returns the purpose's model_whitelist.
//   - When purpose == "all" and priceTier is set: returns the tier whitelist.
//     Empty whitelist (ultra tier) means "no limit" — caller should leave
//     ModelLimitsEnabled=false.
func ModelWhitelistForToken(purpose, brand, priceTier string) ([]string, bool) {
	mu.RLock()
	defer mu.RUnlock()

	if purpose == "all" {
		tierID := priceTier
		if tierID == "" {
			tierID = defaultTierID
		}
		tier, ok := priceTiers[tierID]
		if !ok {
			return nil, false
		}
		if len(tier.ModelWhitelist) == 0 {
			return nil, false // unlimited
		}
		// Always include the virtual alias so the client can call "deeprouter"
		// in Auto mode and have it pass the model_limits gate; the distributor
		// will fall through (no alias binding for purpose=all) and require
		// the client to send a real model name. The virtual name itself just
		// needs to survive the gate when present.
		out := make([]string, 0, len(tier.ModelWhitelist)+len(virtualModels))
		out = append(out, tier.ModelWhitelist...)
		for v := range virtualModels {
			out = append(out, v)
		}
		return out, true
	}

	info, ok := purposesByID[purpose]
	if !ok {
		return nil, false
	}
	if len(info.ModelWhitelist) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(info.ModelWhitelist)+len(virtualModels))
	out = append(out, info.ModelWhitelist...)
	for v := range virtualModels {
		out = append(out, v)
	}
	return out, true
}

// ModelWhitelistString joins a whitelist into the CSV format that
// model.Token.ModelLimits uses on the wire.
func ModelWhitelistString(list []string) string {
	return strings.Join(list, ",")
}

// GetPurposeSummary returns the localized purpose cards in stable order
// (matches the YAML seed order, which drives the UI grid).
func GetPurposeSummary(lang string) []PurposeSummary {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]PurposeSummary, 0, len(purposes))
	zh := strings.HasPrefix(strings.ToLower(lang), "zh")
	for _, p := range purposes {
		label := p.LabelEn
		desc := p.DescEn
		humanEst := p.HumanEstimateEn
		if zh {
			if p.LabelZh != "" {
				label = p.LabelZh
			}
			if p.DescZh != "" {
				desc = p.DescZh
			}
			if p.HumanEstimateZh != "" {
				humanEst = p.HumanEstimateZh
			}
		}
		out = append(out, PurposeSummary{
			ID:               p.ID,
			Label:            label,
			Icon:             p.Icon,
			Desc:             desc,
			HumanEstimate:    humanEst,
			PriceRange:       p.PriceRange,
			RecommendedBrand: p.RecommendedBrand,
			AvailableBrands:  p.AvailableBrands,
		})
	}
	return out
}

// GetPriceTierSummary returns the localized price-tier cards in canonical
// order (economy → standard → premium → ultra).
func GetPriceTierSummary(lang string) []PriceTierSummary {
	mu.RLock()
	defer mu.RUnlock()

	zh := strings.HasPrefix(strings.ToLower(lang), "zh")
	out := make([]PriceTierSummary, 0, len(priceTiers))
	for _, id := range priceTierOrder {
		t, ok := priceTiers[id]
		if !ok {
			continue
		}
		label := t.LabelEn
		desc := t.DescEn
		if zh {
			if t.LabelZh != "" {
				label = t.LabelZh
			}
			if t.DescZh != "" {
				desc = t.DescZh
			}
		}
		out = append(out, PriceTierSummary{
			ID:              id,
			Label:           label,
			Desc:            desc,
			PriceRange:      t.PriceRange,
			IsDefault:       t.IsDefault,
			RequiresConfirm: t.RequiresConfirm,
		})
	}
	return out
}

// DefaultPriceTierID returns the tier id flagged is_default in YAML, used
// by the frontend's initial form state when purpose=all is selected.
func DefaultPriceTierID() string {
	mu.RLock()
	defer mu.RUnlock()
	return defaultTierID
}
