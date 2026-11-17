package smart_router_client

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/config"
)

// setJevOptions saves the toggle and key through the same option path the settings page uses.
func setJevOptions(t *testing.T, enabled, key string) {
	t.Helper()
	err := config.GlobalConfig.LoadFromDB(map[string]string{
		"smart_router_setting.jev_enabled": enabled,
		"smart_router_setting.jev_api_key": key,
	})
	if err != nil {
		t.Fatalf("LoadFromDB: %v", err)
	}
}

func TestJevCatalogFields_OffByDefault(t *testing.T) {
	for _, kids := range []bool{false, true} {
		if enabled, key := JevCatalogFields(kids); enabled || key != "" {
			t.Errorf("kids=%v: got (%v, %q); the toggle must start off and no key is saved", kids, enabled, key)
		}
	}
	if JevAPIKeyConfigured() {
		t.Error("no key is configured until an admin saves one")
	}
}

func TestJevCatalogFields_ToggleAndKeyButNeverForKids(t *testing.T) {
	t.Cleanup(func() { setJevOptions(t, "false", "") })

	setJevOptions(t, "true", "k-admin")
	if enabled, key := JevCatalogFields(false); !enabled || key != "k-admin" {
		t.Errorf("ordinary tenant: got (%v, %q), want (true, k-admin)", enabled, key)
	}
	if enabled, key := JevCatalogFields(true); enabled || key != "" {
		t.Errorf("kids tenant: got (%v, %q); it must get neither the toggle nor the key", enabled, key)
	}
	if !JevAPIKeyConfigured() {
		t.Error("a saved key must report as configured")
	}

	setJevOptions(t, "true", "")
	if enabled, key := JevCatalogFields(false); !enabled || key != "" {
		t.Errorf("toggle on without a key: got (%v, %q), want (true, empty) so smart-router logs no_key", enabled, key)
	}

	setJevOptions(t, "false", "k-admin")
	if enabled, key := JevCatalogFields(false); enabled || key != "" {
		t.Errorf("toggle off: got (%v, %q); the key must not be sent while Jev is off", enabled, key)
	}
}

func TestJevSetting_IsRegisteredUnderItsOptionNames(t *testing.T) {
	if config.GlobalConfig.Get("smart_router_setting") == nil {
		t.Fatal(`"smart_router_setting" is not registered; the settings page could not save the toggle`)
	}
	exported := config.GlobalConfig.ExportAllConfigs()
	for _, k := range []string{"smart_router_setting.jev_enabled", "smart_router_setting.jev_api_key"} {
		if _, ok := exported[k]; !ok {
			t.Errorf("option key %s missing from %v", k, exported)
		}
	}
}

func TestClient_TimeoutIsTheConfiguredOne(t *testing.T) {
	if got := NewClient("http://x", 800*time.Millisecond).Timeout(); got != 800*time.Millisecond {
		t.Errorf("Timeout() = %v, want 800ms", got)
	}
	if got := NewClient("http://x", 0).Timeout(); got != defaultTimeoutMs*time.Millisecond {
		t.Errorf("Timeout() with no value = %v, want the %dms default", got, defaultTimeoutMs)
	}
	var nilClient *Client
	if got := nilClient.Timeout(); got != defaultTimeoutMs*time.Millisecond {
		t.Errorf("nil client Timeout() = %v, want the default", got)
	}
}
