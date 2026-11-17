package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/internal/smart_router_client"
	"github.com/QuantumNous/new-api/setting/config"

	"github.com/gin-gonic/gin"
)

func TestGetSmartRouterStatus_ReportsWhetherAKeyIsSavedWithoutRevealingIt(t *testing.T) {
	set := func(key string) {
		t.Helper()
		if err := config.GlobalConfig.LoadFromDB(map[string]string{"smart_router_setting.jev_api_key": key}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { set("") })

	for _, tc := range []struct {
		key  string
		want string
	}{
		{"", `"jev_api_key_configured":false`},
		{"k-secret", `"jev_api_key_configured":true`},
	} {
		set(tc.key)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/option/smart_router", nil)
		GetSmartRouterStatus(c)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("key %q: got %d %s, want %s", tc.key, w.Code, w.Body.String(), tc.want)
		}
		if strings.Contains(w.Body.String(), "k-secret") {
			t.Errorf("the status must never carry the key: %s", w.Body.String())
		}
	}
}

// runTest posts prompt to the test endpoint as admin user 42, with smart-router reached through client.
func runTest(t *testing.T, client *smart_router_client.Client, prompt string) string {
	t.Helper()
	smartRouterForTest = func() *smart_router_client.Client { return client }
	t.Cleanup(func() { smartRouterForTest = smart_router_client.Default })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/option/smart_router/test", strings.NewReader(`{"prompt":`+prompt+`}`))
	c.Set("id", 42)
	RunSmartRouterTest(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	return w.Body.String()
}

func TestRunSmartRouterTest_ShowsTheDecisionAndJevVerdict(t *testing.T) {
	var sent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		sent = string(b)
		_, _ = io.WriteString(w, `{"primary":"claude-sonnet-5","reason":"jev:code_hard","explain":{"jev_status":"ok","jev_ms":287,"task":"code","difficulty":4,"rule":"jev_code_hard","proposed_primary":"claude-sonnet-5"}}`)
	}))
	defer srv.Close()

	body := runTest(t, smart_router_client.NewClient(srv.URL, 800*time.Millisecond), `"  design a lock-free map  "`)
	for _, want := range []string{`"success":true`, `"outcome":"ok"`, `"primary":"claude-sonnet-5"`, `"task":"code"`, `"difficulty":4`, `"rule":"jev_code_hard"`, `"gateway_wait_ms":800`, `"elapsed_ms":`} {
		if !strings.Contains(body, want) {
			t.Errorf("reply lacks %s: %s", want, body)
		}
	}
	for _, want := range []string{`"tenant_id":"42"`, `"explain":true`, `"content":"design a lock-free map"`} {
		if !strings.Contains(sent, want) {
			t.Errorf("smart-router was sent %s, missing %s", sent, want)
		}
	}
}

func TestRunSmartRouterTest_SaysWhenSmartRouterIsMissingOrDown(t *testing.T) {
	if body := runTest(t, smart_router_client.NewClient("", 0), `"hi"`); !strings.Contains(body, `"outcome":"not_configured"`) {
		t.Errorf("no SMART_ROUTER_URL: %s", body)
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // nothing listens there any more
	if body := runTest(t, smart_router_client.NewClient(srv.URL, 0), `"hi"`); !strings.Contains(body, `"outcome":"unreachable"`) || !strings.Contains(body, `"success":true`) {
		t.Errorf("smart-router down: %s", body)
	}
}

func TestRunSmartRouterTest_RejectsAnEmptyPrompt(t *testing.T) {
	for _, prompt := range []string{`""`, `"   "`} {
		if body := runTest(t, smart_router_client.NewClient("http://127.0.0.1:1", 0), prompt); !strings.Contains(body, `"success":false`) {
			t.Errorf("prompt %s: %s", prompt, body)
		}
	}
}
