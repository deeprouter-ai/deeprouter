package connect

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() { gin.SetMode(gin.TestMode) }

// withKeysDB points model.DB at a throwaway SQLite file holding two users' keys,
// so the ownership check runs against a real query rather than a stub.
func withKeysDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "connect.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Token{}))

	require.NoError(t, db.Create(&model.Token{
		Id: 1, UserId: 100, Name: "alice key", Key: "alice-secret-key", Status: 1,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: 2, UserId: 200, Name: "bob key", Key: "bob-secret-key", Status: 1,
	}).Error)

	prev := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = prev
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

// issueAs posts an issue request as the given user id; userID 0 means "no session".
func issueAs(t *testing.T, userID, keyID int, tools []string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(issueRequest{KeyID: keyID, Tools: tools})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/connect/token", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID != 0 {
		c.Set("id", userID)
	}
	IssueToken(c)
	return w
}

// redeem calls the script endpoint with the token as a path parameter.
func redeem(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/i/"+token, nil)
	c.Params = gin.Params{{Key: "token", Value: token}}
	RedeemScript(c)
	return w
}

// tokenFrom pulls the issued token out of a successful issue response.
func tokenFrom(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Token      string   `json:"token"`
			BaseURL    string   `json:"base_url"`
			Tools      []string `json:"tools"`
			ExpiresIn  int      `json:"expires_in"`
			ScriptPath string   `json:"script_path"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.NotEmpty(t, body.Data.Token)
	return body.Data.Token
}

func TestConnectHandler_IssueAndRedeemDeliversOwnKey(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)
	system_setting.ServerAddress = "https://example.test"

	w := issueAs(t, 100, 1, []string{ToolCodex, ToolClaudeCode})
	require.Equal(t, http.StatusOK, w.Code)
	token := tokenFrom(t, w)

	got := redeem(t, token)
	require.Equal(t, http.StatusOK, got.Code)
	script := got.Body.String()

	require.Contains(t, script, "alice-secret-key", "the key must be injected server-side")
	require.Contains(t, script, "https://example.test")
	require.Contains(t, script, "claude-code")
	require.Contains(t, script, "codex")
	require.Equal(t, "no-store", got.Header().Get("Cache-Control"))
}

// 🔴 The single privilege-escalation path this feature has. A session may only
// ever turn into that session's own key.
func TestConnectHandler_CannotIssueForAnotherUsersKey(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)

	// Alice (100) asks for Bob's key (id 2).
	w := issueAs(t, 100, 2, []string{ToolCodex})

	require.Equal(t, http.StatusNotFound, w.Code)
	require.NotContains(t, w.Body.String(), "bob-secret-key")
}

// The grant carries the key's row id, so even a tampered store entry can only
// yield a key belonging to the user the grant names.
func TestConnectHandler_RedeemIsScopedToTheGrantsOwner(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)

	// A grant naming Alice as owner but Bob's key id resolves to nothing.
	token, err := Issue(Grant{UserID: 100, TokenID: 2, Tools: []string{ToolCodex}})
	require.NoError(t, err)

	got := redeem(t, token)
	require.Equal(t, http.StatusNotFound, got.Code)
	require.NotContains(t, got.Body.String(), "bob-secret-key")
}

func TestConnectHandler_IssueRequiresSession(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)

	w := issueAs(t, 0, 1, []string{ToolCodex})

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.NotContains(t, w.Body.String(), "alice-secret-key")
}

func TestConnectHandler_IssueRejectsEmptyToolSelection(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)

	t.Run("nothing selected", func(t *testing.T) {
		require.Equal(t, http.StatusBadRequest, issueAs(t, 100, 1, nil).Code)
	})
	t.Run("only unsupported tools selected", func(t *testing.T) {
		require.Equal(t, http.StatusBadRequest, issueAs(t, 100, 1, []string{"cursor"}).Code)
	})
}

// A spent or unknown token must answer in a way a person reading their terminal
// can act on — and must not be a stack trace or a JSON blob piped into sh.
func TestConnectHandler_RedeemFailureIsReadableShell(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)

	token := tokenFrom(t, issueAs(t, 100, 1, []string{ToolCodex}))
	require.Equal(t, http.StatusOK, redeem(t, token).Code)

	second := redeem(t, token)
	require.Equal(t, http.StatusNotFound, second.Code)
	body := second.Body.String()
	require.NotContains(t, body, "alice-secret-key")
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		require.True(t, strings.HasPrefix(line, "#"),
			"every line must be a shell comment so `curl | sh` cannot execute it, got %q", line)
	}
}

func TestConnectHandler_ListToolsMatchesCatalogue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/connect/tools", nil)
	ListTools(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data []Tool `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, SupportedTools, body.Data)
}

// The base URL comes from this instance's own setting and nothing else: two
// independent deployments exist with non-interchangeable keys, and a hardcoded
// default fails as a run of 401s that never mentions the address (PRD §0.1 F2).
func TestConnectScript_HasNoHardcodedBaseURL(t *testing.T) {
	script := RenderScript(PlatformPOSIX, "https://tenant-a.test", "k", []string{ToolCodex})

	require.Contains(t, script, "https://tenant-a.test")
	for _, host := range []string{"deeprouter.co", "deep-router.com", "api.deeprouter"} {
		require.NotContains(t, script, host, "no deployment may be baked into the template")
	}
}

// The key reaches the script through single quotes; a value containing one
// would otherwise end the quoting and turn setup into command execution.
func TestConnectScript_QuotesInjectedValues(t *testing.T) {
	script := RenderScript(PlatformPOSIX, "https://x.test", `k'; rm -rf /; echo '`, []string{ToolCodex})

	require.NotContains(t, script, "rm -rf /;\n")
	require.Contains(t, script, `'\''`)
}

// 🔴 A POSIX script piped into `iex` is not a harmless no-op: PowerShell reports
// every line it cannot parse back to the terminal, and one of those lines holds
// the key — so the key ends up on screen in red. Measured on 2026-08-27 by
// running the page's own Windows command before this split existed.
func TestConnectScript_PowerShellClientGetsPowerShell(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)
	system_setting.ServerAddress = "https://example.test"

	token := tokenFrom(t, issueAs(t, 100, 1, []string{ToolCodex}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/i/"+token, nil)
	c.Request.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT; Windows NT 10.0; zh-CN) WindowsPowerShell/5.1.26100.9168")
	c.Params = gin.Params{{Key: "token", Value: token}}
	RedeemScript(c)

	script := w.Body.String()
	require.Contains(t, script, "$DeepRouterApiKey")
	require.Contains(t, script, "alice-secret-key")
	// The POSIX forms are exactly what PowerShell chokes on.
	require.NotContains(t, script, "#!/bin/sh")
	require.NotContains(t, script, "set -eu")
	require.NotContains(t, script, "DEEPROUTER_API_KEY=")
}

func TestConnectScript_CurlClientGetsPosix(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)
	system_setting.ServerAddress = "https://example.test"

	token := tokenFrom(t, issueAs(t, 100, 1, []string{ToolCodex}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/i/"+token, nil)
	c.Request.Header.Set("User-Agent", "curl/8.21.0")
	c.Params = gin.Params{{Key: "token", Value: token}}
	RedeemScript(c)

	script := w.Body.String()
	require.Contains(t, script, "#!/bin/sh")
	require.Contains(t, script, "DEEPROUTER_API_KEY=")
	require.NotContains(t, script, "$DeepRouterApiKey")
}

func TestConnectScript_PlatformFromUserAgent(t *testing.T) {
	for ua, want := range map[string]Platform{
		"Mozilla/5.0 (Windows NT; Windows NT 10.0; zh-CN) WindowsPowerShell/5.1.26100.9168": PlatformPowerShell,
		"Mozilla/5.0 (Windows NT 10.0) PowerShell/7.4.6":                                    PlatformPowerShell,
		"curl/8.21.0": PlatformPOSIX,
		"Wget/1.21":   PlatformPOSIX,
		"":            PlatformPOSIX,
	} {
		require.Equal(t, want, PlatformFromUserAgent(ua), "user agent %q", ua)
	}
}

// PowerShell escapes a single quote by doubling it; the POSIX backslash form
// would leave the string open and turn setup into command execution.
func TestConnectScript_QuotesInjectedValuesForPowerShell(t *testing.T) {
	script := RenderScript(PlatformPowerShell, "https://x.test", `k'; rm -rf /; echo '`, []string{ToolCodex})

	require.Contains(t, script, "''")
	require.NotContains(t, script, "'"+`+"`+"\\"+`"+`+"''")
}
