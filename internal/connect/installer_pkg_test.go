package connect

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/stretchr/testify/require"
)

func TestPkgFileName_CarriesTokenAndHost(t *testing.T) {
	name, err := PkgFileName("ABCD2345KL", "https://api.deeprouter.co/")
	require.NoError(t, err)
	require.Equal(t, "deeprouter-setup-ABCD2345KL-api.deeprouter.co.pkg", name)

	// Ports spell ':' as '_' — ':' does not survive macOS filenames.
	name, err = PkgFileName("ABCD2345KL", "http://localhost:3000")
	require.NoError(t, err)
	require.Equal(t, "deeprouter-setup-ABCD2345KL-localhost_3000.pkg", name)

	_, err = PkgFileName("ABCD2345KL", "not a url")
	require.Error(t, err)
}

// The pkg endpoint serves the same signed bytes for everyone; only the
// filename is personal. And like every download here, it must not consume the
// token — the token has to survive until the package runs on the Mac.
func TestDownloadInstaller_ServesThePkgWithoutConsumingTheToken(t *testing.T) {
	useMemoryStore(t)
	withKeysDB(t)
	system_setting.ServerAddress = "https://example.test"

	artifact := filepath.Join(t.TempDir(), "deeprouter-setup.pkg")
	require.NoError(t, os.WriteFile(artifact, []byte("signed-bytes"), 0o600))
	t.Setenv(pkgFileEnv, artifact)

	token, err := Issue(Grant{UserID: 100, TokenID: 1, Tools: []string{ToolClaudeCode}})
	require.NoError(t, err)

	// Both spellings download: the tokenized name (what the page links) and
	// the bare one (a hand-built URL); Content-Disposition always answers with
	// the tokenized name, which is the data channel the postinstall reads.
	tokenized := "deeprouter-setup-" + token + "-example.test.pkg"
	for _, file := range []string{tokenized, "deeprouter-setup.pkg"} {
		w := download(t, token, file)
		require.Equal(t, http.StatusOK, w.Code, file)
		require.Equal(t, "signed-bytes", w.Body.String())
		require.Equal(t, `attachment; filename="`+tokenized+`"`,
			w.Header().Get("Content-Disposition"))
		require.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	}

	// A pkg name carrying a DIFFERENT token than the path is nobody's file.
	w := download(t, token, "deeprouter-setup-ZZZZZZZZZZ-example.test.pkg")
	require.Equal(t, http.StatusNotFound, w.Code)

	grant, err := Redeem(token)
	require.NoError(t, err, "downloading the pkg must leave the grant intact")
	require.Equal(t, 1, grant.TokenID)
}

// No artifact configured → the endpoint stays dark. The .cmd/.sh routes keep
// working: they are generated, not shipped.
func TestDownloadInstaller_PkgIsDarkWithoutAnArtifact(t *testing.T) {
	useMemoryStore(t)
	system_setting.ServerAddress = "https://example.test"
	t.Setenv(pkgFileEnv, "")

	w := download(t, "ABCD2345KL", "deeprouter-setup.pkg")
	require.Equal(t, http.StatusNotFound, w.Code)
}

// The other half of the filename contract lives in sh: the postinstall must
// read back exactly what PkgFileName wrote, through a browser rename, a
// hyphenated host and a dev port. DR_PKG_PARSE_ONLY makes it print what it
// parsed and stop before touching the network.
func TestPkgPostinstall_ParsesItsOwnFilename(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh available to run the postinstall")
	}
	script, err := filepath.Abs(filepath.Join("macos-pkg", "scripts", "postinstall"))
	require.NoError(t, err)

	parse := func(t *testing.T, filename string) (string, bool) {
		t.Helper()
		cmd := exec.Command("sh", script)
		cmd.Env = append(os.Environ(),
			"DR_PKG_PARSE_ONLY=1",
			"PACKAGE_PATH=/tmp/下载/"+filename, // a Chinese path must not matter
			"DEEPROUTER_BASE_URL=",
		)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err == nil
	}

	// The round trip with the Go side, hyphenated host included — the token
	// alphabet has no '-', which is what makes the first '-' the split.
	for _, tc := range []struct{ file, want string }{
		{"deeprouter-setup-ABCD2345KL-api.deeprouter.co.pkg", "ABCD2345KL https://api.deeprouter.co"},
		{"deeprouter-setup-ABCD2345KL-deep-router.com.pkg", "ABCD2345KL https://deep-router.com"},
		{"deeprouter-setup-ABCD2345KL-localhost_3000.pkg", "ABCD2345KL http://localhost:3000"},
		// The browser's copy suffix must not break the parse.
		{"deeprouter-setup-ABCD2345KL-api.deeprouter.co (1).pkg", "ABCD2345KL https://api.deeprouter.co"},
	} {
		got, ok := parse(t, tc.file)
		require.True(t, ok, tc.file)
		require.Equal(t, tc.want, got, tc.file)
	}

	// Names that carry no usable token refuse loudly instead of half-running.
	for _, file := range []string{
		"deeprouter-setup.pkg",                  // bare: C-D was ignored
		"deeprouter-setup-lowercase99-host.pkg", // not our alphabet
		"deeprouter-setup-ABCD2345KL.pkg",       // token but no host
		"something-else.pkg",                    // not ours at all
	} {
		out, ok := parse(t, file)
		require.False(t, ok, file)
		require.Contains(t, out, "download a fresh installer", file)
	}
}
