package packageassets

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the runner exactly as Claude Code would: as a
// subprocess, asserting on stdout/stderr/exit code — the black-box contract
// AC-10 is written against. They cover the 5 startup validations and the
// CONFIG_INVALID timeout check; the actual DeepRouter API call cannot be
// tested here (the endpoint is hardcoded on purpose — see packaging.go) and
// is left to manual verification against a real environment before merge.

func pythonBin() string {
	if p := os.Getenv("PYTHON_BIN"); p != "" {
		return p
	}
	return "python3"
}

func requirePython(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(pythonBin()); err != nil {
		t.Skipf("%s not found on PATH, skipping runner subprocess tests", pythonBin())
	}
}

// writePackage lays out a package directory exactly like BuildSkillPackage
// does inside the ZIP: manifest.json + SKILL.md at the root, the runner
// under runtime/. Pass nil/"" to omit a file (to test the "missing" cases).
func writePackage(t *testing.T, dir string, manifest map[string]any, skillMD string) {
	t.Helper()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "deeprouter_skill_runner.py"), []byte(RunnerScript), 0o644); err != nil {
		t.Fatal(err)
	}
	if manifest != nil {
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if skillMD != "" {
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeRawManifest(t *testing.T, dir, raw string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

type runResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// buildEnv starts from the real environment, forces DEEPROUTER_API_KEY empty
// by default (so a developer's real key never accidentally makes a test hit
// the live API), then applies extraEnv on top — letting individual tests set
// their own fake key. Using a map (not a plain slice append) avoids
// duplicate-key entries, whose precedence isn't consistently defined across
// platforms.
func buildEnv(extraEnv []string) []string {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	env["DEEPROUTER_API_KEY"] = ""
	for _, kv := range extraEnv {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	list := make([]string, 0, len(env))
	for k, v := range env {
		list = append(list, k+"="+v)
	}
	return list
}

func runRunner(t *testing.T, dir string, extraEnv []string) runResult {
	t.Helper()
	cmd := exec.Command(pythonBin(), filepath.Join(dir, "runtime", "deeprouter_skill_runner.py"), "--input", "hello")
	cmd.Env = buildEnv(extraEnv)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to execute python runner: %v", err)
		}
	}
	return runResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode}
}

func assertErrorCode(t *testing.T, res runResult, wantCode string) {
	t.Helper()
	if res.exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d (stdout=%q stderr=%q)", res.exitCode, res.stdout, res.stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.stderr), &payload); err != nil {
		t.Fatalf("stderr is not valid JSON: %v (stderr=%q)", err, res.stderr)
	}
	if payload["code"] != wantCode {
		t.Fatalf("expected error code %q, got %v (stderr=%q)", wantCode, payload["code"], res.stderr)
	}
	if _, ok := payload["message"]; !ok {
		t.Fatalf("error JSON missing required 'message' field: %q", res.stderr)
	}
}

func validManifest() map[string]any {
	return map[string]any{
		"skill_id":                1,
		"skill_version_id":        42,
		"requires_deeprouter_key": true,
	}
}

func TestRunner_ManifestMissing(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	writePackage(t, dir, nil, "# Skill\ncontent")

	res := runRunner(t, dir, nil)
	assertErrorCode(t, res, "PACKAGE_INVALID")
}

func TestRunner_ManifestInvalidJSON(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	writePackage(t, dir, nil, "# Skill\ncontent")
	writeRawManifest(t, dir, "not valid json")

	res := runRunner(t, dir, nil)
	assertErrorCode(t, res, "PACKAGE_INVALID")
}

func TestRunner_ManifestMissingRequiredField(t *testing.T) {
	requirePython(t)
	cases := []string{"skill_id", "skill_version_id", "requires_deeprouter_key"}
	for _, missing := range cases {
		t.Run(missing, func(t *testing.T) {
			dir := t.TempDir()
			manifest := validManifest()
			delete(manifest, missing)
			writePackage(t, dir, manifest, "# Skill\ncontent")

			res := runRunner(t, dir, nil)
			assertErrorCode(t, res, "PACKAGE_INVALID")
		})
	}
}

func TestRunner_ManifestContainsSensitiveField(t *testing.T) {
	requirePython(t)
	sensitiveFields := []string{"user_id", "tenant_id", "kids_mode", "is_kids_session", "billing_user_id"}
	for _, field := range sensitiveFields {
		t.Run(field, func(t *testing.T) {
			dir := t.TempDir()
			manifest := validManifest()
			manifest[field] = "should-not-be-here"
			writePackage(t, dir, manifest, "# Skill\ncontent")

			res := runRunner(t, dir, nil)
			assertErrorCode(t, res, "PACKAGE_INVALID")
		})
	}
}

func TestRunner_SkillMDMissing(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	writePackage(t, dir, validManifest(), "")

	res := runRunner(t, dir, nil)
	assertErrorCode(t, res, "PACKAGE_INVALID")
}

func TestRunner_SkillMDEmpty(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	writePackage(t, dir, validManifest(), "   \n  ")

	res := runRunner(t, dir, nil)
	assertErrorCode(t, res, "PACKAGE_INVALID")
}

func TestRunner_APIKeyMissing(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	writePackage(t, dir, validManifest(), "# Skill\ncontent")

	res := runRunner(t, dir, []string{"DEEPROUTER_API_KEY="})
	assertErrorCode(t, res, "AUTH_REQUIRED")
}

func TestRunner_TimeoutEnvInvalid(t *testing.T) {
	requirePython(t)
	dir := t.TempDir()
	writePackage(t, dir, validManifest(), "# Skill\ncontent")

	res := runRunner(t, dir, []string{
		"DEEPROUTER_API_KEY=sk-dr-test-key",
		"DEEPROUTER_EXECUTION_TIMEOUT_SECONDS=not-a-number",
	})
	assertErrorCode(t, res, "CONFIG_INVALID")
}
