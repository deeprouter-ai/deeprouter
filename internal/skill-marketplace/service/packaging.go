package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	"github.com/QuantumNous/new-api/internal/skill-marketplace/packageassets"
)

var (
	ErrProviderKeyDetected      = errors.New("package contains a provider API key pattern")
	ErrRuntimeDependencyMissing = errors.New("package runner does not call the DeepRouter routing endpoint")
)

// Hardcoded to match the address deeprouter_skill_runner.py itself calls
// (see packageassets/deeprouter_skill_runner.py) — kept in sync manually
// since the runner is Python and this check is Go.
const deepRouterRoutingEndpoint = "deeprouter.co/v1/routing/chat/completions"

type providerKeyPattern struct {
	name string
	re   *regexp.Regexp
}

// PRD §9: reject a package that leaks a vendor API key. The three named
// examples are OpenAI, Anthropic and AWS; extend this slice if more vendor
// formats need catching later.
var providerKeyPatterns = []providerKeyPattern{
	{name: "anthropic", re: regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	{name: "openai", re: regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)},
	{name: "aws-access-key", re: regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
}

// validateSkillPackageSecurity scans the plain-text pieces that are about to
// be packaged (SKILL.md, manifest.json, the bundled runner/README) for a
// vendor API key pattern, before any ZIP bytes are produced — a match stops
// packaging entirely, so a failing package is never assembled or persisted.
func validateSkillPackageSecurity(contents ...string) error {
	for _, content := range contents {
		for _, p := range providerKeyPatterns {
			if p.re.MatchString(content) {
				return fmt.Errorf("%w: %s key pattern matched", ErrProviderKeyDetected, p.name)
			}
		}
	}
	return nil
}

// validateSkillPackageRuntimeDependency confirms the runner script actually
// calls DeepRouter's own routing endpoint, so a package can never silently
// redirect execution to bypass platform billing.
func validateSkillPackageRuntimeDependency(runnerContent string) error {
	if !strings.Contains(runnerContent, deepRouterRoutingEndpoint) {
		return ErrRuntimeDependencyMissing
	}
	return nil
}

// buildFinalManifest takes the Admin-uploaded manifest_json (which already
// has slug/version/requires_deeprouter_key/deeprouter_routing_endpoint) and
// injects the two fields only the server knows: skill_id and
// skill_version_id (PRD §4.1 — these are DB-assigned and never set by Admin).
func buildFinalManifest(skill *model.Skill, version *model.SkillVersion) ([]byte, error) {
	var manifest map[string]interface{}
	if err := common.Unmarshal(version.ManifestJSON, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest_json for version %d: %w", version.ID, err)
	}
	manifest["skill_id"] = skill.ID
	manifest["skill_version_id"] = version.ID
	return common.Marshal(manifest)
}

func renderReadme(slug string) string {
	return strings.ReplaceAll(packageassets.ReadmeTemplate, "{slug}", slug)
}

// BuildSkillPackage assembles a skill's ZIP package entirely in memory:
// Admin's SKILL.md, the injected manifest.json, and the shared
// runner+README under runtime/. Both security guards run against the plain
// text pieces before any ZIP bytes are written — on failure, no ZIP is ever
// produced, matching the "package not persisted" requirement in PRD §9.
//
// Pure function: no DB access. The caller (the activation transaction)
// decides what to do with the returned bytes.
func BuildSkillPackage(skill *model.Skill, version *model.SkillVersion) (zipBytes []byte, sha256Hex string, err error) {
	finalManifest, err := buildFinalManifest(skill, version)
	if err != nil {
		return nil, "", err
	}
	readme := renderReadme(skill.Slug)

	if err := validateSkillPackageSecurity(
		version.SkillMDContent,
		string(finalManifest),
		packageassets.RunnerScript,
		readme,
	); err != nil {
		return nil, "", err
	}
	if err := validateSkillPackageRuntimeDependency(packageassets.RunnerScript); err != nil {
		return nil, "", err
	}

	root := skill.Slug + "/"
	files := []struct {
		name    string
		content string
	}{
		{root + "manifest.json", string(finalManifest)},
		{root + "SKILL.md", version.SkillMDContent},
		{root + "runtime/deeprouter_skill_runner.py", packageassets.RunnerScript},
		{root + "runtime/README.md", readme},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, werr := zw.Create(f.name)
		if werr != nil {
			return nil, "", fmt.Errorf("zip create %s: %w", f.name, werr)
		}
		if _, werr := w.Write([]byte(f.content)); werr != nil {
			return nil, "", fmt.Errorf("zip write %s: %w", f.name, werr)
		}
	}
	if werr := zw.Close(); werr != nil {
		return nil, "", fmt.Errorf("zip close: %w", werr)
	}

	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:]), nil
}
