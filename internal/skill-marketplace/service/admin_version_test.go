package service_test

import (
	"encoding/json"
	"fmt"
	"testing"

	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func insertVersion(t *testing.T, db *gorm.DB, skillID int64, version, status string) int64 {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO skill_versions (skill_id, version, status, skill_md_content, manifest_json, created_by)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		skillID, version, status, "# Test\ncontent", []byte(`{"slug":"test-slug","version":"1.0.0"}`), 1,
	).Error)
	var id int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&id).Error)
	return id
}

func versionManifestJSON(t *testing.T, db *gorm.DB, versionID int64) string {
	t.Helper()
	var raw string
	require.NoError(t, db.Raw(`SELECT manifest_json FROM skill_versions WHERE id = ?`, versionID).Scan(&raw).Error)
	return raw
}

func versionSkillMDContent(t *testing.T, db *gorm.DB, versionID int64) string {
	t.Helper()
	var content string
	require.NoError(t, db.Raw(`SELECT skill_md_content FROM skill_versions WHERE id = ?`, versionID).Scan(&content).Error)
	return content
}

func validManifestJSON(slug, version string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"slug": %q,
		"version": %q,
		"requires_deeprouter_key": true,
		"deeprouter_routing_endpoint": "https://deeprouter.co/v1/routing/chat/completions"
	}`, slug, version))
}

// ── ListVersions ─────────────────────────────────────────────────────────────

func TestListVersions_ReturnsAllVersionsNewestFirst(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)

	skillID := insertSkill(t, db, "list-versions", "draft")
	v1 := insertVersion(t, db, skillID, "1.0.0", "archived")
	v2 := insertVersion(t, db, skillID, "1.1.0", "active")
	v3 := insertVersion(t, db, skillID, "1.2.0", "draft")

	versions, err := svc.ListVersions(skillID)
	require.NoError(t, err)
	require.Len(t, versions, 3)
	// created_at ties (same-second inserts under SQLite) break on id DESC,
	// so the most recently inserted row (v3) must come first regardless.
	assert.Equal(t, v3, versions[0].ID)
	assert.Equal(t, v2, versions[1].ID)
	assert.Equal(t, v1, versions[2].ID)
}

func TestListVersions_EmptyWhenNoVersions(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)

	skillID := insertSkill(t, db, "no-versions", "draft")

	versions, err := svc.ListVersions(skillID)
	require.NoError(t, err)
	assert.Empty(t, versions)
}

func TestListVersions_OmitsPackageZip(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)

	skillID := insertSkill(t, db, "omit-zip", "published")
	versionID := insertVersion(t, db, skillID, "1.0.0", "active")
	require.NoError(t, db.Exec(
		`UPDATE skill_versions SET package_zip = ? WHERE id = ?`,
		[]byte("not-actually-a-zip"), versionID,
	).Error)

	versions, err := svc.ListVersions(skillID)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assert.Nil(t, versions[0].PackageZip)
}

// ── UploadVersion ────────────────────────────────────────────────────────────

func TestUploadVersion_Success(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")

	version, err := svc.UploadVersion(skillID, mktsvc.UploadVersionRequest{
		Version:        "1.0.0",
		SkillMDContent: "# Test Skill\n\nDoes things.",
		ManifestJSON:   validManifestJSON("test-slug", "1.0.0"),
		Changelog:      "initial release",
	}, 1)

	require.NoError(t, err)
	assert.Equal(t, "draft", version.Status)
	assert.Equal(t, "1.0.0", version.Version)
	assert.Equal(t, int64(1), logCount(t, db, skillID, "version_upload"))
}

func TestUploadVersion_InvalidSemver(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")

	_, err := svc.UploadVersion(skillID, mktsvc.UploadVersionRequest{
		Version:        "v1",
		SkillMDContent: "content",
		ManifestJSON:   validManifestJSON("test-slug", "v1"),
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrInvalidVersionFormat)
}

func TestUploadVersion_ManifestMissingRequiredField(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")

	_, err := svc.UploadVersion(skillID, mktsvc.UploadVersionRequest{
		Version:        "1.0.0",
		SkillMDContent: "content",
		ManifestJSON:   json.RawMessage(`{"slug": "test-slug", "version": "1.0.0"}`), // missing requires_deeprouter_key
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrManifestInvalid)
}

func TestUploadVersion_ManifestSlugMismatch(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")

	_, err := svc.UploadVersion(skillID, mktsvc.UploadVersionRequest{
		Version:        "1.0.0",
		SkillMDContent: "content",
		ManifestJSON:   validManifestJSON("a-different-slug", "1.0.0"),
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrManifestSlugMismatch)
}

func TestUploadVersion_DuplicateVersionConflicts(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	insertVersion(t, db, skillID, "1.0.0", "draft")

	_, err := svc.UploadVersion(skillID, mktsvc.UploadVersionRequest{
		Version:        "1.0.0",
		SkillMDContent: "content",
		ManifestJSON:   validManifestJSON("test-slug", "1.0.0"),
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrVersionAlreadyExists)
}

func TestUploadVersion_ManifestVersionMismatch(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")

	_, err := svc.UploadVersion(skillID, mktsvc.UploadVersionRequest{
		Version:        "1.0.0",
		SkillMDContent: "content",
		ManifestJSON:   validManifestJSON("test-slug", "9.9.9"), // manifest disagrees with req.Version
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrManifestVersionMismatch)
}

func TestUploadVersion_SkillNotFound(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)

	_, err := svc.UploadVersion(9999, mktsvc.UploadVersionRequest{
		Version:        "1.0.0",
		SkillMDContent: "content",
		ManifestJSON:   validManifestJSON("test-slug", "1.0.0"),
	}, 1)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ── UpdateVersion ────────────────────────────────────────────────────────────

func TestUpdateVersion_DraftSucceeds_NoLog(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft")

	newContent := "# Updated\ncontent"
	version, err := svc.UpdateVersion(skillID, versionID, mktsvc.UpdateVersionRequest{
		SkillMDContent: &newContent,
	}, 1)

	require.NoError(t, err)
	assert.Equal(t, newContent, version.SkillMDContent)
	// PRD §5.6's action enum has no "edit" entry — editing a draft is not audited.
	assert.Equal(t, int64(0), logCount(t, db, skillID, "version_upload"))
}

func TestUpdateVersion_ManifestValidSucceeds(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft") // hardcoded manifest slug/version: test-slug / 1.0.0

	newManifest := validManifestJSON("test-slug", "1.0.0")
	version, err := svc.UpdateVersion(skillID, versionID, mktsvc.UpdateVersionRequest{
		ManifestJSON: &newManifest,
	}, 1)

	require.NoError(t, err)
	assert.JSONEq(t, string(newManifest), string(version.ManifestJSON))
}

func TestUpdateVersion_ManifestSlugMismatchRejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft")

	badManifest := validManifestJSON("a-different-slug", "1.0.0")
	_, err := svc.UpdateVersion(skillID, versionID, mktsvc.UpdateVersionRequest{
		ManifestJSON: &badManifest,
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrManifestSlugMismatch)
	// Rejected before the DB write — the original manifest must survive untouched.
	assert.JSONEq(t, `{"slug":"test-slug","version":"1.0.0"}`, versionManifestJSON(t, db, versionID))
}

func TestUpdateVersion_ManifestVersionMismatchRejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft")

	badManifest := validManifestJSON("test-slug", "9.9.9") // version.Version stays "1.0.0"
	_, err := svc.UpdateVersion(skillID, versionID, mktsvc.UpdateVersionRequest{
		ManifestJSON: &badManifest,
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrManifestVersionMismatch)
}

func TestUpdateVersion_ManifestMissingRequiredFieldRejected(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft")

	badManifest := json.RawMessage(`{"slug":"test-slug","version":"1.0.0"}`) // missing requires_deeprouter_key
	_, err := svc.UpdateVersion(skillID, versionID, mktsvc.UpdateVersionRequest{
		ManifestJSON: &badManifest,
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrManifestInvalid)
}

func TestUpdateVersion_InvalidManifestAlsoBlocksOtherFieldChanges(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft")

	newContent := "# Should not be saved"
	badManifest := validManifestJSON("wrong-slug", "1.0.0")
	_, err := svc.UpdateVersion(skillID, versionID, mktsvc.UpdateVersionRequest{
		SkillMDContent: &newContent,
		ManifestJSON:   &badManifest,
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrManifestSlugMismatch)
	assert.Equal(t, "# Test\ncontent", versionSkillMDContent(t, db, versionID), "no field should be saved when manifest validation fails")
}

func TestUpdateVersion_ActiveBlocked(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "active")

	newContent := "# Updated"
	_, err := svc.UpdateVersion(skillID, versionID, mktsvc.UpdateVersionRequest{
		SkillMDContent: &newContent,
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrVersionNotDraft)
}

func TestUpdateVersion_ArchivedBlocked(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "archived")

	newContent := "# Updated"
	_, err := svc.UpdateVersion(skillID, versionID, mktsvc.UpdateVersionRequest{
		SkillMDContent: &newContent,
	}, 1)

	require.ErrorIs(t, err, mktsvc.ErrVersionNotDraft)
}

// ── DeleteVersion ────────────────────────────────────────────────────────────

func TestDeleteVersion_DraftSucceeds(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft")

	err := svc.DeleteVersion(skillID, versionID, 1)

	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM skill_versions WHERE id = ?`, versionID).Scan(&count).Error)
	assert.Equal(t, int64(0), count)
	assert.Equal(t, int64(1), logCount(t, db, skillID, "version_delete"))
}

func TestDeleteVersion_ActiveBlocked(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "active")

	err := svc.DeleteVersion(skillID, versionID, 1)

	require.ErrorIs(t, err, mktsvc.ErrVersionNotDraft)
	var count int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM skill_versions WHERE id = ?`, versionID).Scan(&count).Error)
	assert.Equal(t, int64(1), count, "active version must not be deleted")
}

func TestDeleteVersion_ArchivedBlocked(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "archived")

	err := svc.DeleteVersion(skillID, versionID, 1)

	require.ErrorIs(t, err, mktsvc.ErrVersionNotDraft)
}

// ── ActivateVersion ──────────────────────────────────────────────────────────

func skillActiveVersionID(t *testing.T, db *gorm.DB, skillID int64) *int64 {
	t.Helper()
	var id *int64
	require.NoError(t, db.Raw(`SELECT active_version_id FROM skills WHERE id = ?`, skillID).Scan(&id).Error)
	return id
}

func versionStatus(t *testing.T, db *gorm.DB, versionID int64) string {
	t.Helper()
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM skill_versions WHERE id = ?`, versionID).Scan(&status).Error)
	return status
}

func countActiveVersions(t *testing.T, db *gorm.DB, skillID int64) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM skill_versions WHERE skill_id = ? AND status = 'active'`, skillID,
	).Scan(&n).Error)
	return n
}

func latestLogDetails(t *testing.T, db *gorm.DB, skillID int64, action string) map[string]any {
	t.Helper()
	var raw string
	require.NoError(t, db.Raw(
		`SELECT details FROM skill_admin_logs WHERE skill_id = ? AND action = ? ORDER BY id DESC LIMIT 1`,
		skillID, action,
	).Scan(&raw).Error)
	var details map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &details))
	return details
}

func TestActivateVersion_FirstActivation_Success(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft")

	version, err := svc.ActivateVersion(skillID, versionID, 1)

	require.NoError(t, err)
	assert.Equal(t, "active", version.Status)
	assert.NotEmpty(t, version.PackageSHA256)
	assert.NotNil(t, version.PackageBuiltAt)

	activeID := skillActiveVersionID(t, db, skillID)
	require.NotNil(t, activeID)
	assert.Equal(t, versionID, *activeID)
	assert.Equal(t, int64(1), logCount(t, db, skillID, "version_activate"))

	details := latestLogDetails(t, db, skillID, "version_activate")
	assert.Nil(t, details["previous_active_version_id"], "no previous active version on first activation")
}

func TestActivateVersion_ArchivesPreviousActive(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	v1 := insertVersion(t, db, skillID, "1.0.0", "draft")
	v2 := insertVersion(t, db, skillID, "1.1.0", "draft")

	_, err := svc.ActivateVersion(skillID, v1, 1)
	require.NoError(t, err)

	_, err = svc.ActivateVersion(skillID, v2, 1)
	require.NoError(t, err)

	assert.Equal(t, "archived", versionStatus(t, db, v1))
	assert.Equal(t, "active", versionStatus(t, db, v2))
	assert.Equal(t, int64(1), countActiveVersions(t, db, skillID), "exactly one active version at a time")

	activeID := skillActiveVersionID(t, db, skillID)
	require.NotNil(t, activeID)
	assert.Equal(t, v2, *activeID)

	details := latestLogDetails(t, db, skillID, "version_activate")
	assert.Equal(t, float64(v1), details["previous_active_version_id"], "log must record what it switched away from")
}

func TestActivateVersion_ReactivatingArchivedVersion_IsRollback(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	v1 := insertVersion(t, db, skillID, "1.0.0", "draft")
	v2 := insertVersion(t, db, skillID, "1.1.0", "draft")

	_, err := svc.ActivateVersion(skillID, v1, 1)
	require.NoError(t, err)
	_, err = svc.ActivateVersion(skillID, v2, 1)
	require.NoError(t, err)

	// Roll back to v1.
	_, err = svc.ActivateVersion(skillID, v1, 1)
	require.NoError(t, err)

	assert.Equal(t, "active", versionStatus(t, db, v1))
	assert.Equal(t, "archived", versionStatus(t, db, v2))
	assert.Equal(t, int64(1), countActiveVersions(t, db, skillID))

	activeID := skillActiveVersionID(t, db, skillID)
	require.NotNil(t, activeID)
	assert.Equal(t, v1, *activeID)
}

func TestActivateVersion_ReactivatingSameVersion_DoesNotArchiveItself(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	versionID := insertVersion(t, db, skillID, "1.0.0", "draft")

	_, err := svc.ActivateVersion(skillID, versionID, 1)
	require.NoError(t, err)

	_, err = svc.ActivateVersion(skillID, versionID, 1)
	require.NoError(t, err)

	assert.Equal(t, "active", versionStatus(t, db, versionID))
	assert.Equal(t, int64(1), countActiveVersions(t, db, skillID))
}

func TestActivateVersion_SecurityGuardBlocksActivation_NoPartialState(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")

	require.NoError(t, db.Exec(
		`INSERT INTO skill_versions (skill_id, version, status, skill_md_content, manifest_json, created_by)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		skillID, "1.0.0", "draft",
		"Oops, my key is sk-abcdefghijklmnopqrstuvwx1234",
		[]byte(`{"slug":"test-slug","version":"1.0.0"}`), 1,
	).Error)
	var versionID int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&versionID).Error)

	_, err := svc.ActivateVersion(skillID, versionID, 1)

	require.Error(t, err)
	assert.Equal(t, "draft", versionStatus(t, db, versionID), "guard failure must roll back the status change too")
	assert.Nil(t, skillActiveVersionID(t, db, skillID))
	assert.Equal(t, int64(0), logCount(t, db, skillID, "version_activate"))
}

func TestActivateVersion_VersionNotFound(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")

	_, err := svc.ActivateVersion(skillID, 9999, 1)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// TestActivateVersion_RepeatedFlips_InvariantHolds sequentially flips
// activation back and forth many times. It cannot exercise real row-level
// locking (SQLite has no equivalent of PostgreSQL's SELECT ... FOR UPDATE —
// see the P2 stage⑤ design note), but it does thoroughly exercise the
// archive/activate state-machine logic itself across repeated transitions.
// Real concurrent-transaction locking behavior must be verified against a
// real Postgres instance before merge.
func TestActivateVersion_RepeatedFlips_InvariantHolds(t *testing.T) {
	db := setupDB(t)
	svc := mktsvc.NewAdminVersionService(db)
	skillID := insertSkill(t, db, "test-slug", "draft")
	v1 := insertVersion(t, db, skillID, "1.0.0", "draft")
	v2 := insertVersion(t, db, skillID, "1.1.0", "draft")

	targets := []int64{v1, v2, v1, v2, v2, v1}
	for i, target := range targets {
		_, err := svc.ActivateVersion(skillID, target, 1)
		require.NoError(t, err, "flip #%d", i)
		assert.Equal(t, int64(1), countActiveVersions(t, db, skillID), "flip #%d", i)
		activeID := skillActiveVersionID(t, db, skillID)
		require.NotNil(t, activeID, "flip #%d", i)
		assert.Equal(t, target, *activeID, "flip #%d", i)
	}
}
