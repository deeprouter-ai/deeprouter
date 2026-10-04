package connect

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// download calls the installer endpoint the way the router does.
func download(t *testing.T, token, file string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/d/"+token+"/"+file, nil)
	c.Params = gin.Params{{Key: "token", Value: token}, {Key: "file", Value: file}}
	DownloadInstaller(c)
	return w
}

// 🔴 The trap this whole endpoint was designed around: downloading a file must
// not spend the token, or the file is dead before the user ever double-clicks it.
func TestDownloadInstaller_DoesNotConsumeTheToken(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)
	system_setting.ServerAddress = "https://example.test"

	token, err := Issue(Grant{UserID: 100, TokenID: 1, Tools: []string{ToolClaudeCode}})
	require.NoError(t, err)

	for _, file := range []string{InstallerFileWindows, InstallerFileUnix} {
		w := download(t, token, file)
		require.Equal(t, http.StatusOK, w.Code, file)
	}

	// Still redeemable afterwards, and it still names the same key.
	grant, err := Redeem(token)
	require.NoError(t, err, "downloading must leave the grant intact")
	require.Equal(t, 1, grant.TokenID)
}

// The point of the .cmd wrapper: it holds the window open, and it holds it open
// whether the run succeeded or failed (pause never looks at an exit code).
func TestRenderInstaller_WindowsWrapperHoldsTheWindow(t *testing.T) {
	out := RenderInstaller(PlatformPowerShell, "https://example.test/", "ABCDEFGHJK")

	require.True(t, strings.HasPrefix(out, "@echo off"))
	require.Contains(t, out, "irm 'https://example.test/i/ABCDEFGHJK' | iex")
	require.Contains(t, out, "-ExecutionPolicy Bypass")
	require.Contains(t, out, "pause")
	// pause is the last instruction: it must run after PowerShell exits, not
	// somewhere in the middle where a failing run would skip it.
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\r\n")
	require.Equal(t, "pause >nul", lines[len(lines)-1])
	// Batch files need CRLF, and a trailing slash on the base URL must not
	// produce "//i/".
	require.NotContains(t, strings.ReplaceAll(out, "\r\n", ""), "\n")
	require.NotContains(t, out, "//i/")
}

func TestRenderInstaller_UnixWrapperRunsTheSameScript(t *testing.T) {
	out := RenderInstaller(PlatformPOSIX, "https://example.test", "ABCDEFGHJK")

	require.Contains(t, out, "curl -fsSL 'https://example.test/i/ABCDEFGHJK' | sh")
	require.Contains(t, out, "bash ~/Downloads/"+InstallerFileUnix)
	// A shell script with CR in it fails in ways that read as a syntax error.
	require.NotContains(t, out, "\r")
}

// 🔴 PRD §11.4: the key never reaches the file. Only the token does.
func TestRenderInstaller_CarriesNoKey(t *testing.T) {
	for _, platform := range []Platform{PlatformPowerShell, PlatformPOSIX} {
		out := RenderInstaller(platform, "https://example.test", "ABCDEFGHJK")
		require.NotContains(t, out, "sk-")
		require.NotContains(t, out, "DR_API_KEY")
		require.NotContains(t, out, "DrApiKey")
	}
}

func TestDownloadInstaller_ServesAsAnAttachment(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)
	system_setting.ServerAddress = "https://example.test"

	token, err := Issue(Grant{UserID: 100, TokenID: 1, Tools: []string{ToolClaudeCode}})
	require.NoError(t, err)

	w := download(t, token, InstallerFileWindows)
	require.Equal(t, http.StatusOK, w.Code)
	// text/plain would make the browser display the file instead of saving it.
	require.Equal(t, `attachment; filename="`+InstallerFileWindows+`"`,
		w.Header().Get("Content-Disposition"))
	require.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestDownloadInstaller_RejectsJunk(t *testing.T) {
	useMemoryStore(t)
	system_setting.ServerAddress = "https://example.test"

	// A malformed token never reaches the store, and an unknown filename is not
	// a file we generate.
	require.Equal(t, http.StatusNotFound, download(t, "../../etc/passwd", InstallerFileWindows).Code)
	require.Equal(t, http.StatusNotFound, download(t, "ABCDEFGHJK", "setup.exe").Code)
}

// An unknown token still downloads: /i/:token is where expiry is reported, in a
// script that prints why. Refusing here would only move the failure to a place
// the user cannot read.
func TestDownloadInstaller_UnknownTokenStillServesAFile(t *testing.T) {
	useMemoryStore(t)
	system_setting.ServerAddress = "https://example.test"

	w := download(t, "ZZZZZZZZZZ", InstallerFileUnix)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "/i/ZZZZZZZZZZ")
}

// The TTL is quoted to the user in nine places (page copy, the video prompt, the
// dead-token script); this pins the one value they all describe.
func TestGrantTTL_IsThirtyMinutes(t *testing.T) {
	require.Equal(t, 30*time.Minute, GrantTTL)
}
