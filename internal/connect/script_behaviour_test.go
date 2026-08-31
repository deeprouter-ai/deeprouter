package connect

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// These tests run the real scripts. Nothing here inspects the template as text:
// the whole point of the card is what the script does to a machine, and every
// wrong assumption this feature has cost so far was one that read fine and
// behaved differently (PRD §0.1 is fifteen of them).
//
// Both shells run the same cases. `sh` is present wherever Go builds, including
// Git Bash on Windows. PowerShell is opt-in — see runnerFor.

// ---------------------------------------------------------------------------
// A gateway that can be told how to misbehave
// ---------------------------------------------------------------------------

type modelReply struct {
	status  int
	message string
	// after lets a model answer normally for a while and then start failing,
	// which is the only way to tell a probe apart from the verification that
	// follows it — both call the same model on the same endpoint.
	after int
}

type fakeGateway struct {
	server *httptest.Server

	mu   sync.Mutex
	hits []string       // "METHOD /path model" per request, in order
	seen map[string]int // calls per model, for modelReply.after

	// models is what GET /v1/models lists, and replies decides what each one
	// says when actually called. A model missing from replies answers 200.
	models  []map[string]any
	replies map[string]modelReply
}

func newFakeGateway(t *testing.T) *fakeGateway {
	t.Helper()
	g := &fakeGateway{
		replies: map[string]modelReply{},
		seen:    map[string]int{},
		models: []map[string]any{
			{"id": "gpt-4o-mini", "supported_endpoint_types": []string{"openai"}},
			{"id": "claude-sonnet-4-6", "supported_endpoint_types": []string{"anthropic", "openai"}},
		},
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		g.record("GET /v1/models")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": g.models})
	})

	chat := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &req)
		model := req.Model
		if model == "" {
			// The Gemini endpoint carries the model in the path instead.
			model = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1beta/models/"), ":generateContent")
		}
		g.record(fmt.Sprintf("%s %s %s", r.Method, r.URL.Path, model))

		if reply, ok := g.replyFor(model); ok {
			// Declared, like gin's c.JSON does. Without it PowerShell picks its
			// own encoding for the body and the gateway's Chinese wording -
			// which every rule in PRD §6 keys off - stops matching.
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(reply.status)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": reply.message},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "x", "choices": []map[string]any{{"message": map[string]string{"content": "ok"}}},
		})
	}
	mux.HandleFunc("/v1/chat/completions", chat)
	mux.HandleFunc("/v1/messages", chat)
	mux.HandleFunc("/v1/responses", chat)
	mux.HandleFunc("/v1beta/", chat)

	g.server = httptest.NewServer(mux)
	t.Cleanup(g.server.Close)
	return g
}

func (g *fakeGateway) record(s string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.hits = append(g.hits, s)
}

func (g *fakeGateway) replyFor(model string) (modelReply, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seen[model]++
	r, ok := g.replies[model]
	if !ok || g.seen[model] <= r.after {
		return modelReply{}, false
	}
	return r, true
}

func (g *fakeGateway) fail(model string, status int, message string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies[model] = modelReply{status: status, message: message}
}

// failAfter lets the model answer n times first, so a probe can succeed and the
// verification that follows it can fail.
func (g *fakeGateway) failAfter(model string, n, status int, message string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies[model] = modelReply{status: status, message: message, after: n}
}

func (g *fakeGateway) calls() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.hits...)
}

func (g *fakeGateway) called(substr string) bool {
	for _, h := range g.calls() {
		if strings.Contains(h, substr) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// A machine we are allowed to break
// ---------------------------------------------------------------------------

type runner struct {
	platform Platform
	name     string
}

// runnerFor returns the shells this machine can actually run the scripts in.
//
// PowerShell is opt-in through DEEPROUTER_TEST_POWERSHELL=1 because the script
// really does write to HKCU\Environment — that is the mechanism under test
// (PRD §3 D2) — and `go test` must never reach into the environment of whoever
// happens to run it. The harness snapshots and restores those names, but the
// opt-in is what makes an accident impossible rather than merely unlikely.
func runnersFor(t *testing.T) []runner {
	t.Helper()
	out := []runner{}
	if _, err := exec.LookPath("sh"); err == nil {
		out = append(out, runner{PlatformPOSIX, "posix"})
	}
	if os.Getenv("DEEPROUTER_TEST_POWERSHELL") == "1" && runtime.GOOS == "windows" {
		if _, err := exec.LookPath("powershell"); err == nil {
			out = append(out, runner{PlatformPowerShell, "powershell"})
		}
	}
	if len(out) == 0 {
		t.Skip("no shell available to run the scripts")
	}
	return out
}

type sandbox struct {
	t       *testing.T
	home    string
	bin     string
	gateway *fakeGateway
	run     runner
}

func newSandbox(t *testing.T, r runner, g *fakeGateway) *sandbox {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(home, "fakebin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	s := &sandbox{t: t, home: home, bin: bin, gateway: g, run: r}
	if r.platform == PlatformPowerShell {
		snapshotWindowsEnv()
	}
	return s
}

// The variables the setup script may set, and therefore the ones this harness
// has to put back.
var windowsEnvNames = []string{
	"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_MODEL",
	"ANTHROPIC_SMALL_FAST_MODEL", "DEEPROUTER_API_KEY",
	"GEMINI_API_KEY", "GOOGLE_GEMINI_BASE_URL",
}

var (
	windowsEnvOnce   sync.Once
	windowsEnvBefore map[string]string
)

// snapshotWindowsEnv reads the machine's own values once per test binary.
//
// Without putting them back, a developer running these tests would lose their
// own ANTHROPIC_AUTH_TOKEN - the exact harm PRD 4.2.1 exists to prevent, turned
// on its author. Once, and batched into a single PowerShell call, because
// SetEnvironmentVariable(..., "User") broadcasts WM_SETTINGCHANGE to every
// top-level window and blocks until they answer: doing it per variable per test
// measured at 52 seconds a case, which was the harness being slow, not the
// script (the script itself runs in 0.4s).
func snapshotWindowsEnv() {
	windowsEnvOnce.Do(func() {
		windowsEnvBefore = map[string]string{}
		script := ""
		for _, n := range windowsEnvNames {
			script += fmt.Sprintf("Write-Output ('%s=' + [Environment]::GetEnvironmentVariable('%s','User'));", n, n)
		}
		out, err := psCommand(script).Output()
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(out), "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
			if !ok {
				continue
			}
			// A value this suite could have written is a leftover from a run that
			// did not get to restore, not the machine's own setting. Recording it
			// as the original is how one interrupted run poisons every later one:
			// each snapshot faithfully preserves the previous run's test data, and
			// the real value is gone for good. Treat it as unset instead.
			if isTestWrittenValue(v) {
				v = ""
			}
			windowsEnvBefore[k] = v
		}
	})
}

// psCommand runs a PowerShell one-liner from a scratch directory. The working
// directory matters: PowerShell writes a module analysis cache on startup, and
// with the package directory as cwd it lands inside internal/connect/ and shows
// up as an untracked folder in the repo.
func psCommand(script string) *exec.Cmd {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Dir = os.TempDir()
	return cmd
}

// isTestWrittenValue recognises the values this suite writes. They are the only
// things it is ever allowed to throw away.
func isTestWrittenValue(v string) bool {
	return v == "test-key" || v == "deeprouter-auto" ||
		strings.HasPrefix(v, "http://127.0.0.1:") || strings.HasPrefix(v, "http://[::1]:")
}

// restoreWindowsEnv puts every snapshotted value back in one call.
func restoreWindowsEnv() {
	if windowsEnvBefore == nil {
		return
	}
	script := ""
	for _, n := range windowsEnvNames {
		if v := windowsEnvBefore[n]; v != "" {
			script += fmt.Sprintf("[Environment]::SetEnvironmentVariable('%s','%s','User');", n, strings.ReplaceAll(v, "'", "''"))
		} else {
			script += fmt.Sprintf("[Environment]::SetEnvironmentVariable('%s',$null,'User');", n)
		}
	}
	_ = psCommand(script).Run()
}

// TestMain restores the machine's own environment after everything has run,
// whether the tests passed, failed, or gave up on the way.
func TestMain(m *testing.M) {
	code := m.Run()
	restoreWindowsEnv()
	os.Exit(code)
}

// sandboxEnv inherits the real environment and replaces only the few names that
// decide where the script writes.
//
// Building a list from scratch looks tidier and was measured to be wrong: with a
// stripped environment PowerShell 5.1 fails to start its crypto provider
// (0x8009001d), and a missing PATHEXT made Get-Command miss a .cmd file
// entirely - so "the tool is installed" tests were passing for the wrong
// reason. What the script actually reads is overridden here; the rest of the
// environment is none of this harness's business.
func sandboxEnv(overrides map[string]string) []string {
	out := []string{}
	for _, kv := range os.Environ() {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		replaced := false
		for name := range overrides {
			if strings.EqualFold(k, name) {
				replaced = true
			}
		}
		if !replaced {
			out = append(out, kv)
		}
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

// pathWithoutRealTools drops every PATH entry that provides one of the four
// tools, so whether the person running the tests has Claude Code installed
// cannot change the result. Everything else stays: the scripts genuinely need
// awk, sed, curl and friends, and pretending otherwise would test a shell that
// does not exist.
func pathWithoutRealTools() string {
	names := []string{"claude", "codex", "gemini", "opencode"}
	kept := []string{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		clean := true
		for _, n := range names {
			for _, ext := range []string{"", ".exe", ".cmd", ".bat", ".ps1"} {
				if _, err := os.Stat(filepath.Join(dir, n+ext)); err == nil {
					clean = false
				}
			}
		}
		if clean {
			kept = append(kept, dir)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

// installFakeTool puts something on PATH that answers to the tool's name. It is
// what "installed" means to the script (PRD §0.1 F9), and nothing more.
func (s *sandbox) installFakeTool(exe string) {
	s.t.Helper()
	if s.run.platform == PlatformPowerShell {
		p := filepath.Join(s.bin, exe+".cmd")
		require.NoError(s.t, os.WriteFile(p, []byte("@echo off\r\nexit /b 0\r\n"), 0o755))
		return
	}
	p := filepath.Join(s.bin, exe)
	require.NoError(s.t, os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755))
}

func (s *sandbox) path(parts ...string) string {
	return filepath.Join(append([]string{s.home}, parts...)...)
}

func (s *sandbox) write(rel, content string) {
	s.t.Helper()
	p := s.path(filepath.FromSlash(rel))
	require.NoError(s.t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(s.t, os.WriteFile(p, []byte(content), 0o644))
}

func (s *sandbox) read(rel string) string {
	s.t.Helper()
	b, err := os.ReadFile(s.path(filepath.FromSlash(rel)))
	require.NoError(s.t, err)
	return string(b)
}

func (s *sandbox) exists(rel string) bool {
	_, err := os.Stat(s.path(filepath.FromSlash(rel)))
	return err == nil
}

// setup renders and runs the install script; output is returned for assertions.
func (s *sandbox) setup(tools []string, args ...string) string {
	return s.exec(RenderScript(s.run.platform, s.gateway.server.URL, "test-key", tools), args...)
}

func (s *sandbox) uninstall() string {
	return s.exec(RenderUninstall(s.run.platform))
}

func (s *sandbox) exec(script string, args ...string) string {
	s.t.Helper()
	dir := s.t.TempDir()
	var cmd *exec.Cmd
	if s.run.platform == PlatformPowerShell {
		f := filepath.Join(dir, "run.ps1")
		require.NoError(s.t, os.WriteFile(f, []byte(script), 0o644))
		cmd = exec.Command("powershell", append(
			[]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", f}, args...)...)
	} else {
		f := filepath.Join(dir, "run.sh")
		require.NoError(s.t, os.WriteFile(f, []byte(script), 0o755))
		cmd = exec.Command("sh", append([]string{f}, args...)...)
	}

	// Run from the throwaway directory, not the package directory. A shell that
	// resolves something relative should not be able to drop it into the repo -
	// an earlier version of this harness left a PowerShell module cache in
	// internal/connect/ that way.
	cmd.Dir = dir
	cmd.Env = sandboxEnv(map[string]string{
		"HOME":            s.home,
		"USERPROFILE":     s.home,
		"PATH":            s.bin + string(os.PathListSeparator) + pathWithoutRealTools(),
		"SHELL":           "/bin/bash",
		"XDG_CONFIG_HOME": filepath.Join(s.home, ".config"),
	})
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// ---------------------------------------------------------------------------
// Detection
// ---------------------------------------------------------------------------

// 🔴 The single case where the script working correctly still costs the user
// money: replacing a subscription login with metered API billing (PRD §4.2.1).
func TestScript_SkipsToolsWithAnExistingLogin(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")
			s.write(".claude/.credentials.json", `{"claudeAiOauth":{"accessToken":"x"}}`)

			out := s.setup([]string{ToolClaudeCode})

			require.Contains(t, out, "already signed in")
			require.Contains(t, out, "paying for both")
			// Each shell is told its own flag spelling - PowerShell users type
			// -Force, and printing the POSIX form there would be advice that
			// does not match anything they have seen before.
			wantFlag := "--force claude-code"
			if r.platform == PlatformPowerShell {
				wantFlag = "-Force claude-code"
			}
			require.Contains(t, out, wantFlag, "the way out has to be in the message")
			// Existence is the whole test; the file itself is never read or moved.
			require.Equal(t, `{"claudeAiOauth":{"accessToken":"x"}}`,
				s.read(".claude/.credentials.json"))
			require.False(t, s.exists(".deeprouter/env.sh"))
		})
	}
}

func TestScript_ForceOverridesAnExistingLogin(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")
			s.write(".claude/.credentials.json", `{"claudeAiOauth":{"accessToken":"x"}}`)

			flag := "--force"
			if r.platform == PlatformPowerShell {
				flag = "-Force"
			}
			out := s.setup([]string{ToolClaudeCode}, flag, "claude-code")

			require.Contains(t, out, "Claude Code")
			require.NotContains(t, out, "already signed in")
			// Even forced, the credentials file is still not ours to touch.
			require.Equal(t, `{"claudeAiOauth":{"accessToken":"x"}}`,
				s.read(".claude/.credentials.json"))
		})
	}
}

// 🔴 PRD §0.1 F9. Installing the ChatGPT desktop app leaves a full
// ~/.codex/config.toml behind without ever providing a `codex` command, and it
// is not a rare install. Treating the directory as proof would configure a tool
// that is not there and then tell the user to run a command that does not exist.
func TestScript_ConfigDirectoryIsNotProofOfInstallation(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			existing := "[marketplaces.openai-bundled]\nenabled = true\n"
			s.write(".codex/config.toml", existing)
			// Deliberately no fake `codex` on PATH.

			out := s.setup([]string{ToolCodex})

			require.Contains(t, out, "config directory found")
			require.Contains(t, out, "skipped")
			require.Equal(t, existing, s.read(".codex/config.toml"))
			require.False(t, s.exists(".codex/deeprouter.config.toml"))
		})
	}
}

func TestScript_SaysSoWhenNothingIsInstalled(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)

			out := s.setup([]string{ToolClaudeCode, ToolCodex})

			require.Contains(t, out, "None of the supported tools")
			// PRD §6: never a silent success.
			require.NotContains(t, out, "Done.")
		})
	}
}

// 🔴 Ticking a tool is intent; the page cannot know what is installed and the
// script cannot know what was wanted. A tool that was not ticked must not be
// read, written or even mentioned.
func TestScript_LeavesUntickedToolsCompletelyAlone(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			for _, exe := range []string{"claude", "codex", "gemini", "opencode"} {
				s.installFakeTool(exe)
			}
			gemini := `{"mine":"keep me"}`
			opencode := `{"theme":"mine"}`
			codex := "model = \"gpt-5-codex\"\n"
			s.write(".gemini/settings.json", gemini)
			s.write(".config/opencode/opencode.json", opencode)
			s.write(".codex/config.toml", codex)

			out := s.setup([]string{ToolClaudeCode})

			require.Contains(t, out, "Claude Code")
			require.Equal(t, gemini, s.read(".gemini/settings.json"))
			require.Equal(t, opencode, s.read(".config/opencode/opencode.json"))
			require.Equal(t, codex, s.read(".codex/config.toml"))
			require.NotContains(t, out, "Gemini CLI")
			require.NotContains(t, out, "OpenCode")
		})
	}
}

// ---------------------------------------------------------------------------
// Choosing a model
// ---------------------------------------------------------------------------

// 🔴 PRD §4.3 / §0.1 F19. All three of these arrive as 4xx from one deployment
// and all three mean "try the next one" — only running out of candidates is
// worth telling the user about. Reporting the first as a failure would send
// somebody to top up an account that is fine.
func TestScript_WalksPastEveryKindOf4xx(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			g.models = []map[string]any{
				{"id": "gpt-4o-nano", "supported_endpoint_types": []string{"openai"}},
				{"id": "gpt-4o-mini", "supported_endpoint_types": []string{"openai"}},
				{"id": "claude-flash", "supported_endpoint_types": []string{"openai"}},
				{"id": "gpt-4o", "supported_endpoint_types": []string{"openai"}},
			}
			g.fail("deeprouter-auto", 403, "该令牌无权访问模型 deeprouter-auto")
			g.fail("gpt-4o-nano", 403, "该令牌无权访问模型 gpt-4o-nano")
			g.fail("gpt-4o-mini", 403, "预扣费额度失败, 用户剩余额度: ＄0.03, 需要预扣费额度: ＄0.81 (request id: 202608280519382651562728268d9d6rLNmpAtq)")
			g.fail("claude-flash", 404, "model claude-flash does not exist")

			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")
			out := s.setup([]string{ToolOpenCode})

			require.Contains(t, out, "gpt-4o", "the fourth candidate is the one that answers")
			require.True(t, g.called("gpt-4o-nano"))
			require.True(t, g.called("claude-flash"))
			require.Contains(t, s.read(".config/opencode/opencode.json"), "gpt-4o")
			// None of the three skipped candidates may reach the user as advice.
			require.NotContains(t, out, "Top up")
		})
	}
}

// 🔴 PRD §0.1 F19: gpt-4o-mini-tts answers a chat request with a 403 reserve
// failure, not with "wrong kind of model". Left in the running it reads as
// "you are out of money" and sends the user to pay for a model that will never
// hold a conversation.
func TestScript_NeverPicksAModelThatCannotChat(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			g.models = []map[string]any{
				{"id": "gpt-4o-mini-tts", "supported_endpoint_types": []string{"openai"}},
				{"id": "gpt-4o-audio-preview", "supported_endpoint_types": []string{"openai"}},
				{"id": "text-embedding-3-small", "supported_endpoint_types": []string{"openai"}},
				{"id": "gpt-4o-mini", "supported_endpoint_types": []string{"openai"}},
			}
			g.fail("deeprouter-auto", 403, "该令牌无权访问模型")

			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")
			s.setup([]string{ToolOpenCode})

			require.False(t, g.called("gpt-4o-mini-tts"), "a TTS model must never be probed")
			require.False(t, g.called("gpt-4o-audio-preview"))
			require.Contains(t, s.read(".config/opencode/opencode.json"), "gpt-4o-mini")
		})
	}
}

// PRD §4.3 step 1: deeprouter-auto is the only option that gets smart routing,
// and whether a deployment has it changes with the deployment, so it is always
// asked first — and never assumed.
func TestScript_PrefersSmartRoutingWhenItAnswers(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")

			out := s.setup([]string{ToolClaudeCode})

			require.Contains(t, out, "deeprouter-auto")
			require.Equal(t, "deeprouter-auto", s.envValues(t)["ANTHROPIC_MODEL"],
				"the chosen model has to reach the tool, not just the report")
		})
	}
}

// 🔴 Codex speaks Responses and Gemini speaks Gemini; neither request body has
// a `messages` field, so the router reads no conversation and quietly answers
// with a default model instead of failing. A named model is the honest choice
// until that is fixed (PRD §4.3).
func TestScript_DoesNotGiveCodexTheAutoModel(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("codex")

			s.setup([]string{ToolCodex})

			toml := s.read(".codex/config.toml")
			require.NotContains(t, toml, "deeprouter-auto")
			require.Contains(t, toml, "gpt-4o-mini")
		})
	}
}

// ---------------------------------------------------------------------------
// Writing configuration
// ---------------------------------------------------------------------------

// 🔴 The most important promise in the PRD: wiping somebody's configuration is
// far worse than not configuring anything.
func TestScript_MergesAndOverwritesOnlyTheDefaultModel(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("gemini")
			s.installFakeTool("opencode")
			s.write(".gemini/settings.json",
				`{"ui":{"theme":"dark"},"model":{"maxSessionTurns":42},"mine":[1,2,3]}`)
			s.write(".config/opencode/opencode.json",
				`{"theme":"tokyonight","keybinds":{"leader":"ctrl+x"},"model":"google/nano-banana-pro"}`)

			s.setup([]string{ToolGeminiCLI, ToolOpenCode})

			var gemini map[string]any
			require.NoError(t, json.Unmarshal([]byte(s.read(".gemini/settings.json")), &gemini))
			// Ours went in...
			require.Equal(t, "gemini-api-key",
				gemini["security"].(map[string]any)["auth"].(map[string]any)["selectedType"])
			// ...and theirs is still there, including a sibling of a key we set.
			require.Equal(t, "dark", gemini["ui"].(map[string]any)["theme"])
			require.EqualValues(t, 42, gemini["model"].(map[string]any)["maxSessionTurns"])
			require.Len(t, gemini["mine"], 3)

			var oc map[string]any
			require.NoError(t, json.Unmarshal([]byte(s.read(".config/opencode/opencode.json")), &oc))
			require.Equal(t, "tokyonight", oc["theme"])
			require.Equal(t, "ctrl+x", oc["keybinds"].(map[string]any)["leader"])
			require.NotNil(t, oc["provider"].(map[string]any)["deeprouter"])
			// Their default is the one thing the script replaces on purpose:
			// left in place, OpenCode keeps starting on a model the user has
			// no key for (seen live 2026-08-28: Nano Banana Pro demanding a
			// Google key, read as "setup failed").
			require.Equal(t, "deeprouter/deeprouter-auto", oc["model"])
		})
	}
}

// PRD §4.3: a file that does not parse is left exactly as it is, that tool is
// abandoned, and the others still get configured.
func TestScript_LeavesBrokenConfigUntouched(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("gemini")
			s.installFakeTool("opencode")
			broken := `{"ui": {"theme": "dark",,,}`
			s.write(".gemini/settings.json", broken)

			out := s.setup([]string{ToolGeminiCLI, ToolOpenCode})

			require.Equal(t, broken, s.read(".gemini/settings.json"), "not one byte")
			require.Contains(t, out, "not valid JSON")
			// The other tool is unaffected by its neighbour's bad file.
			require.Contains(t, s.read(".config/opencode/opencode.json"), "deeprouter")
		})
	}
}

// 🔴 PRD Q7, measured. Somebody who already configured Codex has an opinion
// about their default model; hijacking it is rude, and telling them nothing
// leaves them thinking setup failed.
func TestScript_CodexKeepsAnExistingConfigAndTellsYouTheCommand(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("codex")
			mine := "model = \"gpt-5-codex\"\napproval_policy = \"never\"\n"
			s.write(".codex/config.toml", mine)

			out := s.setup([]string{ToolCodex})

			require.Equal(t, mine, s.read(".codex/config.toml"), "their file, untouched")
			profile := s.read(".codex/deeprouter.config.toml")
			require.Contains(t, profile, `wire_api = "responses"`)
			require.NotContains(t, profile, `wire_api = "chat"`)
			require.Contains(t, out, "codex --profile deeprouter")
			// A real user typed plain codex after installing, met the ChatGPT
			// login screen, and concluded setup had failed - the quiet
			// one-line hint was printed and still not seen (2026-08-28). The
			// warning now has to be impossible to skim past.
			require.Contains(t, out, "CODEX - READ THIS")
			require.Contains(t, out, "STILL show the ChatGPT login screen")

			// 🔴 `--profile <name>` layers $CODEX_HOME/<name>.config.toml over the
			// base config, so these keys have to sit at the top level. Wrapping
			// them in a [profiles.deeprouter] section buries them in a nested
			// profile nothing activates: Codex loads the file, reports
			// `provider: openai`, and goes to api.openai.com. Measured on real
			// codex-cli 0.149.1 - the file looked right in every way a unit test
			// can see, which is exactly why this asserts the shape.
			require.NotContains(t, profile, "[profiles.",
				"the settings must be top-level, not nested in a profile section")
			// \r? because the Windows script writes CRLF on purpose (notepad
			// shows it correctly), and Go's multiline $ anchors before \n only.
			require.Regexp(t, `(?m)^model_provider = "deeprouter"\r?$`, profile)
			require.Regexp(t, `(?m)^model = ".+"\r?$`, profile)
		})
	}
}

func TestScript_CodexWritesTheMainFileWhenThereIsNone(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("codex")

			out := s.setup([]string{ToolCodex})

			require.Contains(t, s.read(".codex/config.toml"), "[model_providers.deeprouter]")
			require.False(t, s.exists(".codex/deeprouter.config.toml"))
			require.NotContains(t, out, "--profile deeprouter")
		})
	}
}

// 🔴 A provider entry alone is not enough for OpenCode. With no top-level
// "model", it starts on whatever its own catalog picks - seen live on
// 2026-08-28 as Nano Banana Pro demanding a Google key, which the user read
// as "setup failed". Same lesson as F24: the config file looks right in
// every unit-test view, and only launching the real tool shows the gap.
func TestScript_OpenCodeGetsTheDefaultModelToo(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")

			s.setup([]string{ToolOpenCode})

			var oc map[string]any
			require.NoError(t, json.Unmarshal([]byte(s.read(".config/opencode/opencode.json")), &oc))
			require.Equal(t, "deeprouter/deeprouter-auto", oc["model"])
		})
	}
}

// 🔴 With no wire at all, the old copy said "check that your key is allowed
// to use at least one chat model" - blaming the account for a network
// problem. Found live in a --network none container (PRD 6, L210 row 11).
func TestScript_SaysNetworkWhenItCannotReachTheGateway(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")

			// Port 9 (discard) refuses connections on any sane machine.
			out := s.exec(RenderScript(r.platform, "http://127.0.0.1:9", "test-key", []string{ToolOpenCode}))

			require.Contains(t, out, "Cannot reach DeepRouter")
			require.Contains(t, out, "network connection")
			require.NotContains(t, out, "no model on this account")
			require.NotContains(t, out, "at least one chat model")
		})
	}
}

// 🔴 A read-only config used to produce the worst possible answer: a raw
// "Permission denied" from the shell, then [ ok ] and "1 tool(s) configured"
// - a false success the user has no way to see through. Found live on a
// real Linux container (PRD 6, L210 row 5).
func TestScript_ReadOnlyConfigIsReportedNotFakedOver(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			// Root ignores mode bits, so chmod 0444 stops nothing and the
			// write "succeeds" - a false failure of this test, not of the
			// script. CI runs unprivileged and still covers it; this guard is
			// for whoever runs the suite in a docker container as root.
			if os.Geteuid() == 0 {
				t.Skip("running as root: file modes are not enforced")
			}
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")
			s.write(".config/opencode/opencode.json", `{"theme":"mine"}`)
			target := filepath.Join(s.home, ".config", "opencode", "opencode.json")
			require.NoError(t, os.Chmod(target, 0o444))
			t.Cleanup(func() { _ = os.Chmod(target, 0o644) })

			out := s.setup([]string{ToolOpenCode})

			if r.platform == PlatformPowerShell {
				require.Contains(t, out, "could not rewrite")
			} else {
				require.Contains(t, out, "cannot write")
			}
			require.NotContains(t, out, "1 tool(s) configured")
			// And the file is exactly what the user had.
			require.Equal(t, `{"theme":"mine"}`, s.read(".config/opencode/opencode.json"))
		})
	}
}

// 🔴 Four tools, four different base URLs, and none of them complains at write
// time when it is wrong (PRD §3). Each one here was measured the hard way.
func TestScript_EachToolGetsItsOwnBaseURLShape(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			for _, exe := range []string{"claude", "codex", "gemini", "opencode"} {
				s.installFakeTool(exe)
			}
			base := g.server.URL

			s.setup([]string{ToolClaudeCode, ToolCodex, ToolGeminiCLI, ToolOpenCode})

			env := s.envValues(t)
			// Anthropic native: no /v1, the CLI appends it (§0.1 V2).
			require.Equal(t, base, env["ANTHROPIC_BASE_URL"])
			// Gemini: no /v1beta either, same trap in the other dialect (F11).
			require.Equal(t, base, env["GOOGLE_GEMINI_BASE_URL"])
			require.NotContains(t, env["GOOGLE_GEMINI_BASE_URL"], "v1beta")
			// OpenAI-compatible: /v1 present.
			require.Contains(t, s.read(".codex/config.toml"), base+"/v1")
			require.Contains(t, s.read(".config/opencode/opencode.json"), base+"/v1")
		})
	}
}

// envValues reads back what the script persisted, whichever way it did it.
func (s *sandbox) envValues(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	if s.run.platform == PlatformPowerShell {
		script := ""
		for _, n := range windowsEnvNames {
			script += fmt.Sprintf("Write-Output ('%s=' + [Environment]::GetEnvironmentVariable('%s','User'));", n, n)
		}
		b, err := psCommand(script).Output()
		require.NoError(t, err)
		for _, line := range strings.Split(string(b), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && v != "" {
				out[k] = v
			}
		}
		return out
	}
	for _, line := range strings.Split(s.read(".deeprouter/env.sh"), "\n") {
		line = strings.TrimPrefix(strings.TrimSpace(line), "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[k] = strings.Trim(v, `"`)
	}
	return out
}

// ---------------------------------------------------------------------------
// The shell file, and running twice
// ---------------------------------------------------------------------------

// PRD §3 D2: one line referencing a file we own. Appending exports directly
// would pile up on every run and leave uninstall picking through a file that
// is not ours.
func TestScript_AddsExactlyOneLineAndStaysIdempotent(t *testing.T) {
	for _, r := range runnersFor(t) {
		if r.platform == PlatformPowerShell {
			continue // Windows has no shell file at all; see the registry test
		}
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")
			s.write(".bashrc", "export EDITOR=vim\nalias ll='ls -l'\n")

			s.setup([]string{ToolClaudeCode})
			first := s.read(".bashrc")
			firstEnv := s.read(".deeprouter/env.sh")

			s.setup([]string{ToolClaudeCode})

			require.Equal(t, first, s.read(".bashrc"), "a second run must add nothing")
			require.Equal(t, firstEnv, s.read(".deeprouter/env.sh"))
			require.Equal(t, 1, strings.Count(s.read(".bashrc"), ".deeprouter/env.sh"))
			require.Contains(t, s.read(".bashrc"), "export EDITOR=vim")
			require.Contains(t, s.read(".bashrc"), "alias ll='ls -l'")
		})
	}
}

// A file whose last line has no newline is common, and appending to it blindly
// glues our line onto the end of theirs — which breaks their shell, not ours.
func TestScript_DoesNotGlueOntoALastLineWithoutNewline(t *testing.T) {
	for _, r := range runnersFor(t) {
		if r.platform == PlatformPowerShell {
			continue
		}
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")
			s.write(".bashrc", "export EDITOR=vim")

			s.setup([]string{ToolClaudeCode})

			require.Contains(t, s.read(".bashrc"), "export EDITOR=vim\n")
			require.NotContains(t, s.read(".bashrc"), "vim.")
			require.NotContains(t, s.read(".bashrc"), "vim ")
		})
	}
}

// 🔴 fish does not understand `export`. Getting this wrong shows the user a
// syntax error in every new terminal they open, forever (PRD §3 D2).
func TestScript_UsesFishSyntaxUnderFish(t *testing.T) {
	for _, r := range runnersFor(t) {
		if r.platform == PlatformPowerShell {
			continue
		}
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")

			script := RenderScript(r.platform, g.server.URL, "test-key", []string{ToolClaudeCode})
			dir := t.TempDir()
			f := filepath.Join(dir, "run.sh")
			require.NoError(t, os.WriteFile(f, []byte(script), 0o755))
			cmd := exec.Command("sh", f)
			cmd.Env = sandboxEnv(map[string]string{
				"HOME":            s.home,
				"PATH":            s.bin + string(os.PathListSeparator) + pathWithoutRealTools(),
				"SHELL":           "/usr/bin/fish",
				"XDG_CONFIG_HOME": filepath.Join(s.home, ".config"),
			})
			_, _ = cmd.CombinedOutput()

			envFish := s.read(".deeprouter/env.fish")
			require.Contains(t, envFish, "set -x ANTHROPIC_BASE_URL")
			require.NotContains(t, envFish, "export ")
			require.Contains(t, s.read(".config/fish/config.fish"), "source ")
			require.NotContains(t, s.read(".config/fish/config.fish"), ". \"")
		})
	}
}

// PRD §5.6: the file holds a plaintext key, so other users on the machine must
// not be able to read it.
func TestScript_EnvFileIsNotReadableByOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not survive on Windows filesystems")
	}
	for _, r := range runnersFor(t) {
		if r.platform == PlatformPowerShell {
			continue
		}
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")

			s.setup([]string{ToolClaudeCode})

			info, err := os.Stat(s.path(".deeprouter", "env.sh"))
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	}
}

// ---------------------------------------------------------------------------
// The manifest, and undoing everything
// ---------------------------------------------------------------------------

type manifest struct {
	BaseURL string `json:"base_url"`
	Tools   []struct {
		Name           string `json:"name"`
		File           string `json:"file"`
		PreExisting    bool   `json:"pre_existing"`
		OriginalBackup any    `json:"original_backup"`
	} `json:"tools"`
	EnvVars []string `json:"env_vars"`
}

func (s *sandbox) manifest(t *testing.T) manifest {
	t.Helper()
	var m manifest
	require.NoError(t, json.Unmarshal([]byte(s.read(".deeprouter/installed.json")), &m))
	return m
}

// 🔴 Without this file uninstall is guesswork, and on Windows it is the only
// record at all — nothing is written to disk for the environment variables
// (PRD §4.6).
func TestScript_ManifestRecordsWhatUninstallNeeds(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("gemini")
			s.installFakeTool("codex")
			s.installFakeTool("claude")
			s.write(".gemini/settings.json", `{"mine":true}`)

			s.setup([]string{ToolGeminiCLI, ToolCodex, ToolClaudeCode})
			m := s.manifest(t)

			byName := map[string]int{}
			for i, tool := range m.Tools {
				byName[tool.Name] = i
			}
			gemini := m.Tools[byName["gemini-cli"]]
			require.True(t, gemini.PreExisting, "their settings.json was already there")
			require.NotNil(t, gemini.OriginalBackup)

			codex := m.Tools[byName["codex"]]
			require.False(t, codex.PreExisting, "we created config.toml ourselves")
			require.Nil(t, codex.OriginalBackup)

			require.Contains(t, m.EnvVars, "DEEPROUTER_API_KEY")
			require.Contains(t, m.EnvVars, "GEMINI_API_KEY")

			// 🔴 Claude Code is the row with no file, and an empty field is
			// exactly what a tab-delimited `read` loses: tab is an IFS
			// whitespace character, so the gap closed up and its pre_existing
			// flag was written where the filename belongs. The manifest read
			// as valid JSON the whole time, which is why only looking at the
			// file it produced caught it.
			claude := m.Tools[byName["claude-code"]]
			require.Empty(t, claude.File, "Claude Code is configured by environment alone")
			require.True(t, claude.PreExisting,
				"an empty file field must not swallow the flag after it")
		})
	}
}

// 🔴 The single most valuable test in the card: it is the only thing that
// proves the backup-and-restore chain end to end. Anything less checks that
// uninstall ran, not that it worked.
func TestUninstall_ReturnsTheMachineToHowItWas(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			for _, exe := range []string{"claude", "codex", "gemini", "opencode"} {
				s.installFakeTool(exe)
			}
			before := map[string]string{
				".gemini/settings.json":          `{"ui":{"theme":"dark"},"mine":[1,2]}`,
				".config/opencode/opencode.json": `{"theme":"tokyonight"}`,
				".codex/config.toml":             "model = \"gpt-5-codex\"\n",
				".bashrc":                        "export EDITOR=vim\nalias ll='ls -l'\n",
			}
			for path, content := range before {
				s.write(path, content)
			}

			s.setup([]string{ToolClaudeCode, ToolCodex, ToolGeminiCLI, ToolOpenCode})
			// Sanity: it really did change things, so the comparison below means
			// something.
			require.NotEqual(t, before[".gemini/settings.json"], s.read(".gemini/settings.json"))

			out := s.uninstall()

			for path, content := range before {
				if path == ".bashrc" && r.platform == PlatformPowerShell {
					continue // no shell file on Windows
				}
				require.Equal(t, content, s.read(path), "%s must be byte-identical to before setup", path)
			}
			require.False(t, s.exists(".deeprouter"), "our own directory has to go")
			// The profile file we created is deleted, not left as an empty shell.
			require.False(t, s.exists(".codex/deeprouter.config.toml"))
			require.Contains(t, out, "Done.")

			if r.platform == PlatformPowerShell {
				require.Empty(t, s.envValues(t), "every variable we set must be gone")
			}
		})
	}
}

// A file we created has no original to restore, so the way back is to remove
// it. Leaving an empty shell behind would look like a configuration the user
// never made.
func TestUninstall_DeletesFilesWeCreatedRatherThanEmptyingThem(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("codex")
			s.installFakeTool("gemini")

			s.setup([]string{ToolCodex, ToolGeminiCLI})
			require.True(t, s.exists(".codex/config.toml"))
			require.True(t, s.exists(".gemini/settings.json"))

			s.uninstall()

			require.False(t, s.exists(".codex/config.toml"))
			require.False(t, s.exists(".gemini/settings.json"))
		})
	}
}

// 🔴 Two runs leave two .bak- files and only the earliest is really theirs.
// This is exactly what pre_existing and original_backup exist to get right
// (PRD §4.6).
func TestUninstall_RestoresTheStateBeforeTheFirstInstall(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("gemini")
			original := `{"ui":{"theme":"the-original"}}`
			s.write(".gemini/settings.json", original)

			s.setup([]string{ToolGeminiCLI})
			s.setup([]string{ToolGeminiCLI}) // a second run, a second backup
			s.uninstall()

			require.Equal(t, original, s.read(".gemini/settings.json"))
		})
	}
}

// PRD §4.6: with no manifest there is no record of what to undo, and guessing
// would mean deleting things we never wrote.
func TestUninstall_DoesNothingWhenItWasNeverInstalled(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			mine := "export EDITOR=vim\n"
			s.write(".bashrc", mine)
			s.write(".gemini/settings.json", `{"mine":true}`)

			out := s.uninstall()

			require.Contains(t, out, "No DeepRouter setup was found")
			require.Equal(t, mine, s.read(".bashrc"))
			require.Equal(t, `{"mine":true}`, s.read(".gemini/settings.json"))
		})
	}
}

// Only our line comes out. Everything else in that file is the user's.
func TestUninstall_RemovesOnlyOurLineFromTheShellFile(t *testing.T) {
	for _, r := range runnersFor(t) {
		if r.platform == PlatformPowerShell {
			continue
		}
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")
			mine := "export EDITOR=vim\nalias ll='ls -l'\nexport PATH=$PATH:/opt/bin\n"
			s.write(".bashrc", mine)

			s.setup([]string{ToolClaudeCode})
			s.uninstall()

			require.Equal(t, mine, s.read(".bashrc"))
		})
	}
}

// ---------------------------------------------------------------------------
// Verification and failure wording
// ---------------------------------------------------------------------------

// PRD §4.4: four base URLs, four protocols, none of which complains at write
// time when it is wrong. One blanket request would only prove the key is live.
func TestScript_VerifiesEachProtocolSeparately(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			for _, exe := range []string{"claude", "codex", "gemini", "opencode"} {
				s.installFakeTool(exe)
			}

			s.setup([]string{ToolClaudeCode, ToolCodex, ToolGeminiCLI, ToolOpenCode})

			for _, path := range []string{
				"/v1/messages", "/v1/chat/completions", "/v1/responses", "/v1beta/models/",
			} {
				require.True(t, g.called(path), "%s was never exercised", path)
			}
		})
	}
}

// 🔴 PRD §6 and §0.1 F5. This 403 means "not enough balance", and Claude Code
// shows it as "Please run /login" — so the user re-authenticates forever
// instead of topping up. Translating it is the reason the script sends its own
// verification request at all.
func TestScript_TranslatesTheReserveFailureIntoMoney(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			g.models = []map[string]any{
				{"id": "only-model", "supported_endpoint_types": []string{"openai"}},
			}
			g.fail("deeprouter-auto", 403, "该令牌无权访问模型")
			g.fail("only-model", 403, "预扣费额度失败, 用户剩余额度: ＄0.039042, 需要预扣费额度: ＄0.816000 (request id: 202608280519382651562728268d9d6rLNmpAtq)")

			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")
			out := s.setup([]string{ToolOpenCode})

			// The two figures the gateway gave, passed through untouched.
			require.Contains(t, out, "0.039042")
			require.Contains(t, out, "0.816000")
			require.Contains(t, out, "/topup")
			// PRD §6: no bare status codes anywhere in a failure message.
			require.NotContains(t, out, "403")
			require.NotContains(t, out, "HTTP")
		})
	}
}

func TestScript_SaysTheKeyIsBadRatherThanShowingA401(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			g.fail("deeprouter-auto", 401, "无效的令牌")

			s := newSandbox(t, r, g)
			s.installFakeTool("claude")
			out := s.setup([]string{ToolClaudeCode})

			require.Contains(t, out, "key was rejected")
			require.NotContains(t, out, "401")
			require.False(t, s.exists(".deeprouter/env.sh"), "nothing gets written on a bad key")
		})
	}
}

// A busy model is not a broken configuration, and saying so stops the user
// undoing work that is actually finished (PRD §6).
func TestScript_SaysConfigurationIsFineWhenTheModelIsBusy(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")
			// One good answer for the probe, 503 for the verification after it.
			g.failAfter("deeprouter-auto", 1, 503, "模型繁忙，请稍后重试")
			out := s.setup([]string{ToolOpenCode})

			require.Contains(t, out, "busy")
			require.Contains(t, out, "written and correct")
			require.NotContains(t, out, "503")
		})
	}
}

// ---------------------------------------------------------------------------
// Output the user acts on
// ---------------------------------------------------------------------------

// PRD §2.2: "restart your terminal" is not enough — a shell started from this
// window inherits this window's environment and reads nothing new.
func TestScript_ExplainsThatANewWindowMeansANewWindow(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")

			out := s.setup([]string{ToolClaudeCode})

			require.Contains(t, out, "Close this window completely")
			require.Contains(t, out, "not enough")
		})
	}
}

// 🔴 PRD §0.1 F15: the one thing this setup takes away. Unsaid, the user finds
// out later and has no way to connect it to us.
func TestScript_WarnsThatConnectorsGetDisabled(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")

			out := s.setup([]string{ToolClaudeCode})

			require.Contains(t, out, "connectors are disabled")
			require.Contains(t, out, "uninstalling brings them back")
		})
	}
}

func TestScript_AlwaysEndsWithTheWayOut(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")

			out := s.setup([]string{ToolOpenCode})

			require.Contains(t, out, "/uninstall")
			require.Contains(t, out, "works right now",
				"the start table must say OpenCode needs no new terminal")
		})
	}
}

// 🔴 PRD §5.7: the script holds the user's key and runs on their machine. It
// talks to the gateway that issued the token and to nothing else.
func TestScript_TalksOnlyToTheDeploymentThatIssuedTheToken(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			for _, exe := range []string{"claude", "codex", "gemini", "opencode"} {
				s.installFakeTool(exe)
			}

			script := RenderScript(r.platform, g.server.URL, "test-key",
				[]string{ToolClaudeCode, ToolCodex, ToolGeminiCLI, ToolOpenCode})
			s.exec(script)

			// Every address the script could reach is the injected one; the
			// no-hardcoded-URL test covers the template, this covers the run.
			require.NotEmpty(t, g.calls())
			for _, host := range []string{"api.openai.com", "anthropic.com", "google"} {
				require.NotContains(t, script, host)
			}
		})
	}
}

// PRD §4.2.2: with no terminal on stdin the script cannot ask anything, so
// every choice arrives as a flag and each one has to actually narrow the work.
func TestScript_OnlyNarrowsToTheNamedTool(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("codex")
			s.installFakeTool("gemini")
			mine := `{"mine":"untouched"}`
			s.write(".gemini/settings.json", mine)

			flag := "--only"
			if r.platform == PlatformPowerShell {
				flag = "-Only"
			}
			out := s.setup([]string{ToolCodex, ToolGeminiCLI}, flag, "codex")

			require.True(t, s.exists(".codex/config.toml"))
			require.Equal(t, mine, s.read(".gemini/settings.json"))
			require.NotContains(t, out, "Gemini CLI")
		})
	}
}

// A skipped tool did not fail — it was never configured. Counting it as a
// failed protocol would report a problem that does not exist (PRD §2.2).
func TestScript_SkippedToolsAreNotVerified(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("claude")
			s.installFakeTool("opencode")
			s.write(".claude/.credentials.json", `{"claudeAiOauth":{}}`)

			out := s.setup([]string{ToolClaudeCode, ToolOpenCode})

			require.False(t, g.called("/v1/messages"),
				"the Anthropic protocol belongs to the tool we skipped")
			require.True(t, g.called("/v1/chat/completions"))
			require.NotContains(t, out, "[fail]")
			require.Contains(t, out, "Done. 1 tool(s) configured.")
		})
	}
}

// PRD §6: one protocol failing is not the whole thing failing, and saying so
// would send the user to undo work that is fine.
func TestScript_ReportsPerProtocolNotAllOrNothing(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			s.installFakeTool("opencode")
			s.installFakeTool("claude")
			// Two probes and one verification succeed; the Anthropic
			// verification is the third call and is the one that fails.
			g.failAfter("deeprouter-auto", 2, 500, "upstream exploded")

			out := s.setup([]string{ToolClaudeCode, ToolOpenCode})

			require.Contains(t, out, "[ ok ]")
			require.Contains(t, out, "[fail]")
			require.Contains(t, out, "Done.", "the tools that worked still count")
		})
	}
}

// 🔴 PRD §5.2 and §7. The key is injected into the script body and must stay
// there: a terminal keeps scrollback, gets screenshotted, and gets pasted into
// chats. The command itself only ever carries a single-use token, and the run
// must not put the key back on screen — which is exactly what the wrong-shell
// bug did before P1 split the templates.
func TestScript_NeverEchoesTheKey(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)
			s := newSandbox(t, r, g)
			for _, exe := range []string{"claude", "codex", "gemini", "opencode"} {
				s.installFakeTool(exe)
			}
			const key = "sk-verysecret-do-not-print-me"

			script := RenderScript(r.platform, g.server.URL, key,
				[]string{ToolClaudeCode, ToolCodex, ToolGeminiCLI, ToolOpenCode})
			out := s.exec(script)

			require.NotContains(t, out, key, "the key must never reach the screen")
			// It does have to reach the files the tools read, or nothing works.
			require.Contains(t, s.read(".codex/config.toml"), "DEEPROUTER_API_KEY")
			require.Equal(t, key, s.envValues(t)["DEEPROUTER_API_KEY"])
			// And the failure report must not leak it either.
			require.NotContains(t, s.read(".deeprouter/installed.json"), key)
		})
	}
}

// PRD §4.3: "内容都是同一段" - the two Codex branches differ only in which file
// they write. Keeping them identical is what makes the profile file work at all,
// because Codex layers it verbatim over the base config.
func TestScript_CodexWritesTheSameContentEitherWay(t *testing.T) {
	for _, r := range runnersFor(t) {
		t.Run(r.name, func(t *testing.T) {
			g := newFakeGateway(t)

			fresh := newSandbox(t, r, g)
			fresh.installFakeTool("codex")
			fresh.setup([]string{ToolCodex})
			standalone := fresh.read(".codex/config.toml")

			layered := newSandbox(t, r, g)
			layered.installFakeTool("codex")
			layered.write(".codex/config.toml", "model = \"gpt-5-codex\"\n")
			layered.setup([]string{ToolCodex})
			profile := layered.read(".codex/deeprouter.config.toml")

			require.Equal(t, standalone, profile,
				"the same settings, only the filename changes")
		})
	}
}
