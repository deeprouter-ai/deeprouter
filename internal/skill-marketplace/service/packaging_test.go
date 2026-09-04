package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
)

func testSkillAndVersion() (*model.Skill, *model.SkillVersion) {
	skill := &model.Skill{ID: 1, Slug: "code-review-expert"}
	version := &model.SkillVersion{
		ID:             42,
		SkillID:        1,
		Version:        "1.0.0",
		SkillMDContent: "# Code Review Expert\n\nReviews code for style issues.",
		ManifestJSON: json.RawMessage(`{
			"slug": "code-review-expert",
			"version": "1.0.0",
			"requires_deeprouter_key": true,
			"deeprouter_routing_endpoint": "https://deeprouter.co/v1/routing/chat/completions"
		}`),
	}
	return skill, version
}

func readZipFile(t *testing.T, zr *zip.Reader, name string) string {
	t.Helper()
	f, err := zr.Open(name)
	if err != nil {
		t.Fatalf("zip missing file %s: %v", name, err)
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return buf.String()
}

func TestBuildSkillPackage_Success(t *testing.T) {
	skill, version := testSkillAndVersion()

	zipBytes, sha256Hex, err := BuildSkillPackage(skill, version)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sum := sha256.Sum256(zipBytes)
	if sha256Hex != hex.EncodeToString(sum[:]) {
		t.Fatalf("returned sha256 does not match the actual ZIP bytes")
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("BuildSkillPackage did not return a valid ZIP: %v", err)
	}

	root := skill.Slug + "/"

	skillMD := readZipFile(t, zr, root+"SKILL.md")
	if skillMD != version.SkillMDContent {
		t.Fatalf("SKILL.md content mismatch")
	}

	runner := readZipFile(t, zr, root+"runtime/deeprouter_skill_runner.py")
	if !strings.Contains(runner, deepRouterRoutingEndpoint) {
		t.Fatalf("packaged runner does not contain the DR routing endpoint")
	}

	readme := readZipFile(t, zr, root+"runtime/README.md")
	if !strings.Contains(readme, skill.Slug) {
		t.Fatalf("packaged README does not have the slug substituted in")
	}
	if strings.Contains(readme, "{slug}") {
		t.Fatalf("packaged README still has the unsubstituted {slug} placeholder")
	}

	manifestRaw := readZipFile(t, zr, root+"manifest.json")
	var manifest map[string]interface{}
	if err := json.Unmarshal([]byte(manifestRaw), &manifest); err != nil {
		t.Fatalf("packaged manifest.json is not valid JSON: %v", err)
	}
	if int64(manifest["skill_id"].(float64)) != skill.ID {
		t.Fatalf("manifest.json skill_id not injected correctly, got %v", manifest["skill_id"])
	}
	if int64(manifest["skill_version_id"].(float64)) != version.ID {
		t.Fatalf("manifest.json skill_version_id not injected correctly, got %v", manifest["skill_version_id"])
	}
	if manifest["slug"] != skill.Slug {
		t.Fatalf("manifest.json lost the Admin-supplied slug field")
	}
}

func TestBuildSkillPackage_SecurityGuardBlocksPackaging(t *testing.T) {
	skill, version := testSkillAndVersion()
	version.SkillMDContent = "Oops, my key is sk-abcdefghijklmnopqrstuvwx1234"

	zipBytes, sha256Hex, err := BuildSkillPackage(skill, version)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if zipBytes != nil || sha256Hex != "" {
		t.Fatalf("expected no ZIP bytes to be produced when a guard fails")
	}
}

func TestBuildSkillPackage_InvalidManifestJSON(t *testing.T) {
	skill, version := testSkillAndVersion()
	version.ManifestJSON = json.RawMessage(`not valid json`)

	_, _, err := BuildSkillPackage(skill, version)
	if err == nil {
		t.Fatalf("expected error for invalid manifest_json, got nil")
	}
}

func TestValidateSkillPackageSecurity(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"clean text", "This skill reviews code for style issues.", false},
		{"openai key", "Here is my key: sk-abcdefghijklmnopqrstuvwx1234", true},
		{"anthropic key", "key=sk-ant-api03-abcdefghijklmnopqrstuvwxyz0123456789", true},
		{"aws access key", "AWS_ACCESS_KEY_ID=AKIAABCDEFGHIJKLMNOP", true},
		{"aws temp key", "token: ASIAABCDEFGHIJKLMNOP", true},
		{"short sk- prefix is not a match", "sk-short", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSkillPackageSecurity(c.content)
			if c.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateSkillPackageSecurity_ScansAllProvidedPieces(t *testing.T) {
	err := validateSkillPackageSecurity(
		"clean SKILL.md content",
		`{"slug": "demo"}`,
		"clean runner content",
		"README with a leak: sk-abcdefghijklmnopqrstuvwx1234",
	)
	if err == nil {
		t.Fatalf("expected error when any provided piece contains a key pattern")
	}
}

func TestValidateSkillPackageRuntimeDependency(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "calls DR routing endpoint",
			content: `DEEPROUTER_ROUTING_URL = "https://deeprouter.co/v1/routing/chat/completions"`,
			wantErr: false,
		},
		{
			name:    "calls a different provider",
			content: `requests.post("https://api.openai.com/v1/chat/completions")`,
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSkillPackageRuntimeDependency(c.content)
			if c.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
