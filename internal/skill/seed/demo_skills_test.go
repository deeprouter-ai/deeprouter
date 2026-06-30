package seed

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/skill/enums"
	skillmodel "github.com/QuantumNous/new-api/internal/skill/model"
	"github.com/QuantumNous/new-api/internal/skill/tiers"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func seedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seed.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	if err := skillmodel.MigrateSkills(db); err != nil {
		t.Fatalf("MigrateSkills: %v", err)
	}
	if err := skillmodel.MigrateSkillVersions(db); err != nil {
		t.Fatalf("MigrateSkillVersions: %v", err)
	}
	return db
}

func TestSeedDemoSkills_CreatesPublishedPackagedSkills(t *testing.T) {
	db := seedTestDB(t)

	result, err := SeedDemoSkills(db, 1)
	if err != nil {
		t.Fatalf("SeedDemoSkills: %v", err)
	}
	if len(result.Outcomes) != 8 {
		t.Fatalf("expected 8 outcomes, got %d", len(result.Outcomes))
	}
	for _, o := range result.Outcomes {
		if o.Action != "created" {
			t.Fatalf("%s: expected created on first run, got %s", o.Slug, o.Action)
		}
	}

	var published int64
	db.Model(&skillmodel.Skill{}).Where("status = ?", enums.SkillStatusPublished).Count(&published)
	if published != 8 {
		t.Fatalf("expected 8 published skills, got %d", published)
	}

	wantSlugs := map[string]bool{
		"polished-writer": false, "faithful-translator": false, "code-helper": false, "data-analyst": false,
		"research-synthesizer-pro": false, "legal-clause-reviewer-pro": false, "pr-architecture-reviewer-pro": false, "financial-modeler-pro": false,
	}
	paidSlugs := map[string]bool{
		"research-synthesizer-pro":     true,
		"legal-clause-reviewer-pro":    true,
		"pr-architecture-reviewer-pro": true,
		"financial-modeler-pro":        true,
	}
	var skills []skillmodel.Skill
	if err := db.Find(&skills).Error; err != nil {
		t.Fatalf("load skills: %v", err)
	}
	for _, s := range skills {
		if _, ok := wantSlugs[s.Slug]; !ok {
			t.Fatalf("unexpected slug %q", s.Slug)
		}
		wantSlugs[s.Slug] = true

		// Published + has an active version.
		if s.Status != enums.SkillStatusPublished {
			t.Fatalf("%s: not published", s.Slug)
		}
		if s.ActiveVersionID == nil {
			t.Fatalf("%s: missing active_version_id", s.Slug)
		}
		if s.PublishedAt == nil {
			t.Fatalf("%s: missing published_at", s.Slug)
		}

		// model_whitelist must be valid platform tiers (D-09 rule 2 / DR-110).
		var wl []string
		if err := common.Unmarshal(s.ModelWhitelist, &wl); err != nil {
			t.Fatalf("%s: whitelist json: %v", s.Slug, err)
		}
		if _, ok := tiers.ValidateWhitelist(wl); !ok {
			t.Fatalf("%s: whitelist %v contains a non-tier alias", s.Slug, wl)
		}

		// Description carries the "## Work step" routing call so main's download
		// D-09 guard accepts the capability package (downloadability verified
		// end-to-end in internal/skill/handler seed→download test).
		if !strings.Contains(s.Description, "## Work step") || !strings.Contains(strings.ToLower(s.Description), "deeprouter") {
			t.Fatalf("%s: description missing DeepRouter work step", s.Slug)
		}
		if paidSlugs[s.Slug] {
			if s.RequiredPlan != enums.RequiredPlanPro {
				t.Fatalf("%s: required_plan = %q, want pro", s.Slug, s.RequiredPlan)
			}
			if s.MonetizationType != enums.MonetizationTypePlanIncluded {
				t.Fatalf("%s: monetization_type = %q, want plan_included", s.Slug, s.MonetizationType)
			}
			if s.PriceMarkup != 0 {
				t.Fatalf("%s: price_markup = %v, want 0", s.Slug, s.PriceMarkup)
			}
			if s.FreeQuotaPerMonth != nil {
				t.Fatalf("%s: free_quota_per_month must be nil", s.Slug)
			}
			if !sameStringList(s.ModelWhitelist, []string{"smart-tier"}) {
				t.Fatalf("%s: paid demo skill must be fixed to smart-tier", s.Slug)
			}
			if s.MaxInputTokens == nil || *s.MaxInputTokens < 24000 {
				t.Fatalf("%s: paid demo skill missing large-context max_input_tokens", s.Slug)
			}
		} else if s.RequiredPlan != enums.RequiredPlanFree || s.MonetizationType != enums.MonetizationTypeFree {
			t.Fatalf("%s: DR-51 free seed should stay free/free, got %s/%s", s.Slug, s.RequiredPlan, s.MonetizationType)
		}

		// Active version exists, is active, sha matches the stored template, and
		// the execution-critical snapshot fields are populated (DR-47).
		var v skillmodel.SkillVersion
		if err := db.Where("id = ?", *s.ActiveVersionID).First(&v).Error; err != nil {
			t.Fatalf("%s: load active version: %v", s.Slug, err)
		}
		if v.Status != enums.SkillVersionStatusActive {
			t.Fatalf("%s: active version status is %q", s.Slug, v.Status)
		}
		if v.InstructionTemplateSHA256 != computeTemplateSHA256(v.InstructionTemplate) {
			t.Fatalf("%s: sha mismatch", s.Slug)
		}
		if v.RequiredPlanSnapshot != s.RequiredPlan {
			t.Fatalf("%s: required_plan_snapshot %q != skill plan %q", s.Slug, v.RequiredPlanSnapshot, s.RequiredPlan)
		}
		if !sameStringList(v.ModelWhitelistSnapshot, wl) {
			t.Fatalf("%s: model_whitelist_snapshot does not match skill whitelist", s.Slug)
		}
		if v.MaxInputTokensSnapshot == nil || *v.MaxInputTokensSnapshot <= 0 {
			t.Fatalf("%s: missing max_input_tokens_snapshot", s.Slug)
		}
		if v.OutputSchema == nil || !strings.Contains(string(*v.OutputSchema), "properties") {
			t.Fatalf("%s: output_schema not populated", s.Slug)
		}
		if !strings.Contains(string(v.MonetizationSnapshot), "monetization_type") {
			t.Fatalf("%s: monetization_snapshot missing fields", s.Slug)
		}
	}
	for slug, seen := range wantSlugs {
		if !seen {
			t.Fatalf("missing seeded slug %q", slug)
		}
	}
}

func TestSeedDemoSkills_Idempotent(t *testing.T) {
	db := seedTestDB(t)

	if _, err := SeedDemoSkills(db, 1); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	result, err := SeedDemoSkills(db, 1)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	for _, o := range result.Outcomes {
		if o.Action != "up-to-date" {
			t.Fatalf("%s: re-run should be up-to-date, got %s", o.Slug, o.Action)
		}
	}

	// No duplicate skills or versions created.
	var skillCount, versionCount int64
	db.Model(&skillmodel.Skill{}).Count(&skillCount)
	db.Model(&skillmodel.SkillVersion{}).Count(&versionCount)
	if skillCount != 8 {
		t.Fatalf("expected 8 skills after re-seed, got %d", skillCount)
	}
	if versionCount != 8 {
		t.Fatalf("expected 8 versions after re-seed (no churn), got %d", versionCount)
	}
}

func TestMonetizationSnapshot_QuotaBranches(t *testing.T) {
	quota := 50
	withQuota, err := monetizationSnapshot(skillmodel.Skill{
		MonetizationType:  enums.MonetizationTypeTokenMarkup,
		PriceMarkup:       1.25,
		FreeQuotaPerMonth: &quota,
	})
	if err != nil {
		t.Fatalf("monetizationSnapshot: %v", err)
	}
	for _, want := range []string{"token_markup", "1.25", "free_quota_per_month", "50"} {
		if !strings.Contains(string(withQuota), want) {
			t.Fatalf("snapshot %q missing %q", string(withQuota), want)
		}
	}

	noQuota, err := monetizationSnapshot(skillmodel.Skill{MonetizationType: enums.MonetizationTypeFree})
	if err != nil {
		t.Fatalf("monetizationSnapshot: %v", err)
	}
	if strings.Contains(string(noQuota), "free_quota_per_month") {
		t.Fatalf("nil quota must be omitted, got %s", string(noQuota))
	}
}

func TestSeedDemoSkills_NewVersionOnTemplateChange(t *testing.T) {
	db := seedTestDB(t)
	if _, err := SeedDemoSkills(db, 1); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	// Mutate the active version's template so the next seed must create v2.
	var s skillmodel.Skill
	if err := db.Where("slug = ?", "code-helper").First(&s).Error; err != nil {
		t.Fatalf("load skill: %v", err)
	}
	if err := db.Model(&skillmodel.SkillVersion{}).
		Where("id = ?", *s.ActiveVersionID).
		Update("instruction_template_sha256", "deadbeef").Error; err != nil {
		t.Fatalf("mutate sha: %v", err)
	}

	result, err := SeedDemoSkills(db, 1)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	for _, o := range result.Outcomes {
		if o.Slug == "code-helper" {
			if o.Action != "updated" || o.VersionNumber != 2 {
				t.Fatalf("code-helper should become updated v2, got %s v%d", o.Action, o.VersionNumber)
			}
		}
	}

	// Exactly one active version remains for code-helper.
	var activeCount int64
	db.Model(&skillmodel.SkillVersion{}).
		Where("skill_id = ? AND status = ?", s.ID, enums.SkillVersionStatusActive).
		Count(&activeCount)
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active version, got %d", activeCount)
	}
}
