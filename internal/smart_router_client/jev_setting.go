package smart_router_client

import "github.com/QuantumNous/new-api/setting/config"

// JevSetting is what an admin sets under System Settings → Models & Routing →
// Smart Routing: whether smart-router may ask Jev what kind of task each
// deeprouter-auto request is, and the TypeSafe key to do it with. Both are
// stored as options ("smart_router_setting.jev_enabled" / ".jev_api_key") and
// reach smart-router through the router catalog, so a change applies within
// one catalog poll (30s) without a restart.
//
// Off by default: turning it on sends a summary of user prompts to TypeSafe
// (the Jev provider), which is an admin decision, not a deploy side effect.
// The key is never returned by the options listing (GetOptions withholds
// every *api_key option) and never logged.
type JevSetting struct {
	JevEnabled bool   `json:"jev_enabled"`
	JevAPIKey  string `json:"jev_api_key"`
}

var jevSetting = JevSetting{}

// init registers the settings with the option system so the settings page can read and save them.
func init() {
	config.GlobalConfig.Register("smart_router_setting", &jevSetting)
}

// JevCatalogFields is what the router catalog tells smart-router for one
// tenant: whether it may ask Jev, and the key to use. Kids tenants get
// neither: their prompts do not leave the box for classification (Smart
// Routing V2 PRD §4), so the key is not sent to them either.
func JevCatalogFields(kidsMode bool) (enabled bool, apiKey string) {
	if kidsMode || !jevSetting.JevEnabled {
		return false, ""
	}
	return true, jevSetting.JevAPIKey
}

// JevAPIKeyConfigured reports whether an admin has saved a TypeSafe key, without revealing it.
func JevAPIKeyConfigured() bool {
	return jevSetting.JevAPIKey != ""
}
