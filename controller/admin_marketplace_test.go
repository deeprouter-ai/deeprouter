package controller

// Coverage: the status-code translation controller.go itself owns — the
// switch/if-else blocks that turn a service sentinel error into an HTTP
// status, param binding (skillIDParam/versionIDParam), and the JSON
// envelope shape. Service-layer business logic is already covered in
// internal/skill-marketplace/service's own tests; this file exists only
// for the layer above it, which had zero coverage before this.
//
// Deliberately NOT covered here: middleware.AdminAuth() itself — these
// tests call the handler functions directly, bypassing the router and its
// middleware chain entirely (same convention as user_create_test.go).
// Testing "does a non-admin get rejected" belongs to AdminAuth's own
// tests, not this file.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	mktmodel "github.com/QuantumNous/new-api/internal/skill-marketplace/model"
	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ── fixtures ──────────────────────────────────────────────────────────────────

func setupMarketplaceControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, status INTEGER NOT NULL DEFAULT 1)`,
		`CREATE TABLE skills (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			slug              TEXT UNIQUE NOT NULL,
			name              TEXT NOT NULL DEFAULT '',
			description       TEXT NOT NULL DEFAULT '',
			category          TEXT NOT NULL DEFAULT '',
			tags              TEXT NOT NULL DEFAULT '{}',
			status            TEXT NOT NULL DEFAULT 'draft',
			monetization_type TEXT NOT NULL DEFAULT 'free',
			price_usd         REAL NOT NULL DEFAULT 0,
			featured_flag     INTEGER DEFAULT 0,
			featured_rank     INTEGER DEFAULT 0,
			active_version_id INTEGER,
			created_by        INTEGER NOT NULL,
			created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE skill_versions (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			skill_id         INTEGER NOT NULL,
			version          TEXT NOT NULL,
			status           TEXT NOT NULL DEFAULT 'draft',
			skill_md_content TEXT NOT NULL DEFAULT '',
			manifest_json    BLOB NOT NULL DEFAULT '{}',
			package_zip      BLOB,
			package_sha256   TEXT,
			package_built_at DATETIME,
			changelog        TEXT NOT NULL DEFAULT '',
			created_by       INTEGER NOT NULL,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (skill_id, version)
		)`,
		`CREATE TABLE skill_admin_logs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id   INTEGER NOT NULL,
			skill_id   INTEGER,
			action     TEXT NOT NULL,
			details    TEXT DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, s := range stmts {
		require.NoError(t, db.Exec(s).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1)`).Error)

	previousDB := model.DB
	previousRedisEnabled := common.RedisEnabled
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedisEnabled
	})
	return db
}

func insertTestSkill(t *testing.T, db *gorm.DB, slug, status string) int64 {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, category, status, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		slug, "Test Skill", "d", "code", status, 1,
	).Error)
	var id int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&id).Error)
	return id
}

func insertTestVersion(t *testing.T, db *gorm.DB, skillID int64, version, status string) int64 {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO skill_versions (skill_id, version, status, skill_md_content, manifest_json, created_by)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		skillID, version, status, "# x", []byte(`{"slug":"test-slug","version":"1.0.0"}`), 1,
	).Error)
	var id int64
	require.NoError(t, db.Raw(`SELECT last_insert_rowid()`).Scan(&id).Error)
	return id
}

func setTestActiveVersion(t *testing.T, db *gorm.DB, skillID, versionID int64) {
	t.Helper()
	require.NoError(t, db.Exec(
		`UPDATE skills SET active_version_id = ? WHERE id = ?`, versionID, skillID,
	).Error)
}

// marketplaceContext builds a gin.Context carrying an optional JSON body,
// route params, and an authenticated admin id (c.GetInt("id")) — mirroring
// what middleware.AdminAuth() would have set, without running it.
func marketplaceContext(t *testing.T, method string, body any, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, "/", reader)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = params
	ctx.Set("id", 1)
	return ctx, recorder
}

type marketplaceResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func decodeMarketplaceResponse(t *testing.T, recorder *httptest.ResponseRecorder) marketplaceResponse {
	t.Helper()
	var resp marketplaceResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func intToStr(id int64) string {
	return strconv.FormatInt(id, 10)
}

// ── param helpers (skillIDParam / versionIDParam) ──────────────────────────────

func TestSkillIDParam_NonNumeric_Returns400(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	ctx, recorder := marketplaceContext(t, http.MethodGet, nil, gin.Params{{Key: "id", Value: "not-a-number"}})

	AdminGetSkill(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	resp := decodeMarketplaceResponse(t, recorder)
	assert.False(t, resp.Success)
	assert.Equal(t, "invalid skill id", resp.Message)
}

func TestVersionIDParam_NonNumeric_Returns400(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{
		{Key: "id", Value: "1"}, {Key: "vid", Value: "not-a-number"},
	})

	AdminActivateVersion(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	resp := decodeMarketplaceResponse(t, recorder)
	assert.Equal(t, "invalid version id", resp.Message)
}

// ── AdminListSkills ──────────────────────────────────────────────────────────

func TestAdminListSkills_Success(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	insertTestSkill(t, db, "list-me", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodGet, nil, nil)

	AdminListSkills(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeMarketplaceResponse(t, recorder)
	assert.True(t, resp.Success)
	var data mktsvc.ListSkillsResponse
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	assert.Equal(t, int64(1), data.Total)
}

func TestAdminListSkills_InvalidQueryParam_Returns400(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?page=not-a-number", nil)
	ctx.Set("id", 1)

	AdminListSkills(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.False(t, decodeMarketplaceResponse(t, recorder).Success)
}

// ── AdminGetSkill ────────────────────────────────────────────────────────────

func TestAdminGetSkill_NotFound_Returns404(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	ctx, recorder := marketplaceContext(t, http.MethodGet, nil, gin.Params{{Key: "id", Value: "9999"}})

	AdminGetSkill(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "skill not found", decodeMarketplaceResponse(t, recorder).Message)
}

func TestAdminGetSkill_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "get-me", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodGet, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminGetSkill(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, decodeMarketplaceResponse(t, recorder).Success)
}

// ── AdminCreateSkill ─────────────────────────────────────────────────────────

func TestAdminCreateSkill_InvalidJSON_Returns400(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{not json")))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)

	AdminCreateSkill(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAdminCreateSkill_DuplicateSlug_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	insertTestSkill(t, db, "dup-slug", "draft")
	req := mktsvc.CreateSkillRequest{Slug: "dup-slug", Name: "n", Description: "d", Category: "c"}
	ctx, recorder := marketplaceContext(t, http.MethodPost, req, nil)

	AdminCreateSkill(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Equal(t, "slug already exists", decodeMarketplaceResponse(t, recorder).Message)
}

func TestAdminCreateSkill_Success_Returns200(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	req := mktsvc.CreateSkillRequest{Slug: "new-skill", Name: "n", Description: "d", Category: "c"}
	ctx, recorder := marketplaceContext(t, http.MethodPost, req, nil)

	AdminCreateSkill(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, decodeMarketplaceResponse(t, recorder).Success)
}

func TestAdminCreateSkill_InvalidSlugFormat_Returns400(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	req := mktsvc.CreateSkillRequest{Slug: "Not Valid", Name: "n", Description: "d", Category: "c"}
	ctx, recorder := marketplaceContext(t, http.MethodPost, req, nil)

	AdminCreateSkill(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// ── AdminUpdateSkill ─────────────────────────────────────────────────────────

func TestAdminUpdateSkill_NotFound_Returns404(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	req := mktsvc.UpdateSkillRequest{Name: "x"}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{{Key: "id", Value: "9999"}})

	AdminUpdateSkill(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "skill not found", decodeMarketplaceResponse(t, recorder).Message)
}

func TestAdminUpdateSkill_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "update-me", "draft")
	req := mktsvc.UpdateSkillRequest{Name: "renamed"}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUpdateSkill(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestAdminUpdateSkill_SlugLockedOncePublished_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "pub-slug", "published")
	req := mktsvc.UpdateSkillRequest{Slug: "renamed-slug"}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUpdateSkill(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestAdminUpdateSkill_InvalidSlugFormat_Returns400(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "draft-slug", "draft")
	req := mktsvc.UpdateSkillRequest{Slug: "Not Valid"}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUpdateSkill(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAdminUpdateSkill_DuplicateSlug_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	insertTestSkill(t, db, "already-taken", "draft")
	id := insertTestSkill(t, db, "renaming-me", "draft")
	req := mktsvc.UpdateSkillRequest{Slug: "already-taken"}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUpdateSkill(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

// ── AdminPublishSkill ────────────────────────────────────────────────────────

func TestAdminPublishSkill_NotFound_Returns404(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{{Key: "id", Value: "9999"}})

	AdminPublishSkill(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminPublishSkill_NoActiveVersion_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "no-version", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminPublishSkill(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, decodeMarketplaceResponse(t, recorder).Message, "no active version")
}

func TestAdminPublishSkill_InvalidTransition_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	// No status besides draft/published/deprecated is reachable through the
	// API — using a raw status here to exercise the branch controller-side,
	// same as the service test that covers PublishSkill's own logic for it.
	id := insertTestSkill(t, db, "archived-skill", "archived")
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminPublishSkill(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestAdminPublishSkill_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "publish-me", "draft")
	versionID := insertTestVersion(t, db, id, "1.0.0", "active")
	setTestActiveVersion(t, db, id, versionID)
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminPublishSkill(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

// ── AdminDeprecateSkill ──────────────────────────────────────────────────────

func TestAdminDeprecateSkill_NotFound_Returns404(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{{Key: "id", Value: "9999"}})

	AdminDeprecateSkill(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminDeprecateSkill_InvalidTransition_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "draft-skill", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminDeprecateSkill(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestAdminDeprecateSkill_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "deprecate-me", "published")
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminDeprecateSkill(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

// ── AdminDeleteSkill ─────────────────────────────────────────────────────────

func TestAdminDeleteSkill_NotFound_Returns404(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	ctx, recorder := marketplaceContext(t, http.MethodDelete, nil, gin.Params{{Key: "id", Value: "9999"}})

	AdminDeleteSkill(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminDeleteSkill_PublishedBlocked_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "published-skill", "published")
	ctx, recorder := marketplaceContext(t, http.MethodDelete, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminDeleteSkill(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestAdminDeleteSkill_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "delete-me", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodDelete, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminDeleteSkill(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

// ── AdminUpdateSkillFeatured ─────────────────────────────────────────────────

func TestAdminUpdateSkillFeatured_NotFound_Returns404(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	req := mktsvc.FeaturedRequest{FeaturedFlag: true, FeaturedRank: 1}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{{Key: "id", Value: "9999"}})

	AdminUpdateSkillFeatured(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminUpdateSkillFeatured_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "feature-me", "published")
	req := mktsvc.FeaturedRequest{FeaturedFlag: true, FeaturedRank: 1}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUpdateSkillFeatured(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestAdminUpdateSkillFeatured_NotPublished_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "feature-draft", "draft")
	req := mktsvc.FeaturedRequest{FeaturedFlag: true, FeaturedRank: 1}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUpdateSkillFeatured(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

// ── AdminGetSkillLogs ────────────────────────────────────────────────────────

func TestAdminGetSkillLogs_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "logs-me", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodGet, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminGetSkillLogs(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, decodeMarketplaceResponse(t, recorder).Success)
}

// ── AdminListVersions ────────────────────────────────────────────────────────

func TestAdminListVersions_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "versions-me", "draft")
	insertTestVersion(t, db, id, "1.0.0", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodGet, nil, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminListVersions(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var versions []mktmodel.SkillVersion
	require.NoError(t, json.Unmarshal(decodeMarketplaceResponse(t, recorder).Data, &versions))
	assert.Len(t, versions, 1)
}

// ── AdminUploadVersion ───────────────────────────────────────────────────────

func validTestManifest(slug, version string) json.RawMessage {
	return json.RawMessage(`{"slug":"` + slug + `","version":"` + version + `","requires_deeprouter_key":true,"deeprouter_routing_endpoint":"https://deeprouter.co/v1/routing/chat/completions"}`)
}

func TestAdminUploadVersion_InvalidJSON_Returns400(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "upload-bad-json", "draft")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{not json")))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: intToStr(id)}}
	ctx.Set("id", 1)

	AdminUploadVersion(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAdminUploadVersion_SkillNotFound_Returns404(t *testing.T) {
	setupMarketplaceControllerTestDB(t)
	req := mktsvc.UploadVersionRequest{
		Version: "1.0.0", SkillMDContent: "# x", ManifestJSON: validTestManifest("x", "1.0.0"),
	}
	ctx, recorder := marketplaceContext(t, http.MethodPost, req, gin.Params{{Key: "id", Value: "9999"}})

	AdminUploadVersion(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminUploadVersion_InvalidVersionFormat_Returns400(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "bad-version-format", "draft")
	req := mktsvc.UploadVersionRequest{
		Version: "not-semver", SkillMDContent: "# x", ManifestJSON: validTestManifest("bad-version-format", "not-semver"),
	}
	ctx, recorder := marketplaceContext(t, http.MethodPost, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUploadVersion(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAdminUploadVersion_ManifestMissingField_Returns400(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "bad-manifest", "draft")
	req := mktsvc.UploadVersionRequest{
		Version: "1.0.0", SkillMDContent: "# x", ManifestJSON: json.RawMessage(`{"slug":"bad-manifest"}`),
	}
	ctx, recorder := marketplaceContext(t, http.MethodPost, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUploadVersion(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAdminUploadVersion_DuplicateVersion_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "dup-version", "draft")
	insertTestVersion(t, db, id, "1.0.0", "draft")
	req := mktsvc.UploadVersionRequest{
		Version: "1.0.0", SkillMDContent: "# x", ManifestJSON: validTestManifest("dup-version", "1.0.0"),
	}
	ctx, recorder := marketplaceContext(t, http.MethodPost, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUploadVersion(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestAdminUploadVersion_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "upload-ok", "draft")
	req := mktsvc.UploadVersionRequest{
		Version: "1.0.0", SkillMDContent: "# x", ManifestJSON: validTestManifest("upload-ok", "1.0.0"),
	}
	ctx, recorder := marketplaceContext(t, http.MethodPost, req, gin.Params{{Key: "id", Value: intToStr(id)}})

	AdminUploadVersion(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

// ── AdminUpdateVersion ───────────────────────────────────────────────────────

func TestAdminUpdateVersion_NotFound_Returns404(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "update-version-404", "draft")
	changelog := "x"
	req := mktsvc.UpdateVersionRequest{Changelog: &changelog}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: "9999"},
	})

	AdminUpdateVersion(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminUpdateVersion_ActiveBlocked_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "update-active-blocked", "published")
	versionID := insertTestVersion(t, db, id, "1.0.0", "active")
	changelog := "x"
	req := mktsvc.UpdateVersionRequest{Changelog: &changelog}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: intToStr(versionID)},
	})

	AdminUpdateVersion(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestAdminUpdateVersion_ManifestInvalid_Returns400(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "update-bad-manifest", "draft")
	versionID := insertTestVersion(t, db, id, "1.0.0", "draft")
	badManifest := json.RawMessage(`{"slug":"update-bad-manifest"}`)
	req := mktsvc.UpdateVersionRequest{ManifestJSON: &badManifest}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: intToStr(versionID)},
	})

	AdminUpdateVersion(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAdminUpdateVersion_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "update-version-ok", "draft")
	versionID := insertTestVersion(t, db, id, "1.0.0", "draft")
	changelog := "updated"
	req := mktsvc.UpdateVersionRequest{Changelog: &changelog}
	ctx, recorder := marketplaceContext(t, http.MethodPut, req, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: intToStr(versionID)},
	})

	AdminUpdateVersion(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

// ── AdminActivateVersion ─────────────────────────────────────────────────────

func TestAdminActivateVersion_NotFound_Returns404(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "activate-404", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: "9999"},
	})

	AdminActivateVersion(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminActivateVersion_SecurityGuardBlocked_Returns400(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "activate-leaks-key", "draft")
	// SKILL.md containing something that matches the Anthropic key pattern —
	// validateSkillPackageSecurity must block activation before any ZIP is
	// ever written (PRD §9's "no partial state" requirement).
	require.NoError(t, db.Exec(
		`UPDATE skill_versions SET skill_md_content = ? WHERE id = ?`,
		"leaked: sk-ant-abcdefghijklmnopqrstuvwxyz",
		insertTestVersion(t, db, id, "1.0.0", "draft"),
	).Error)
	var versionID int64
	require.NoError(t, db.Raw(`SELECT id FROM skill_versions WHERE skill_id = ?`, id).Scan(&versionID).Error)

	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: intToStr(versionID)},
	})

	AdminActivateVersion(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, decodeMarketplaceResponse(t, recorder).Message, "provider API key")
}

func TestAdminActivateVersion_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "activate-ok", "draft")
	versionID := insertTestVersion(t, db, id, "1.0.0", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodPost, nil, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: intToStr(versionID)},
	})

	AdminActivateVersion(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

// ── AdminDeleteVersion ───────────────────────────────────────────────────────

func TestAdminDeleteVersion_NotFound_Returns404(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "delete-version-404", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodDelete, nil, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: "9999"},
	})

	AdminDeleteVersion(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAdminDeleteVersion_ActiveBlocked_Returns409(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "delete-active-blocked", "published")
	versionID := insertTestVersion(t, db, id, "1.0.0", "active")
	ctx, recorder := marketplaceContext(t, http.MethodDelete, nil, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: intToStr(versionID)},
	})

	AdminDeleteVersion(ctx)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestAdminDeleteVersion_Success_Returns200(t *testing.T) {
	db := setupMarketplaceControllerTestDB(t)
	id := insertTestSkill(t, db, "delete-version-ok", "draft")
	versionID := insertTestVersion(t, db, id, "1.0.0", "draft")
	ctx, recorder := marketplaceContext(t, http.MethodDelete, nil, gin.Params{
		{Key: "id", Value: intToStr(id)}, {Key: "vid", Value: intToStr(versionID)},
	})

	AdminDeleteVersion(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
}
