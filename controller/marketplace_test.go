/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

// Coverage: controller/marketplace.go had no test file at all before this —
// the user-facing marketplace endpoints (list/detail/download/My Skills) were
// only ever exercised through internal/skill-marketplace/service's own
// tests, which stop at the service layer and never prove what HTTP status
// the controller actually maps a given service error to. PRD §13 added one
// new branch to DownloadMarketplaceSkill (ErrReferenceListingNotDownloadable
// -> 400); this file covers just that branch, the same narrow-scope
// rationale admin_marketplace_test.go states for itself.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupDownloadControllerTestDB seeds only the skills table — Download()
// (internal/skill-marketplace/service/download.go) checks listing_type
// immediately after loading the skill row and returns before touching
// skill_versions, users, or skill_purchases, so a reference-listing rejection
// never reaches them.
func setupDownloadControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE skills (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			slug              TEXT UNIQUE NOT NULL,
			name              TEXT NOT NULL DEFAULT '',
			description       TEXT NOT NULL DEFAULT '',
			tags              TEXT NOT NULL DEFAULT '{}',
			status            TEXT NOT NULL DEFAULT 'draft',
			monetization_type TEXT NOT NULL DEFAULT 'free',
			price_usd         REAL NOT NULL DEFAULT 0,
			featured_flag     INTEGER DEFAULT 0,
			featured_rank     INTEGER DEFAULT 0,
			active_version_id INTEGER,
			listing_type      TEXT NOT NULL DEFAULT 'hosted',
			source_url        TEXT,
			created_by        INTEGER NOT NULL,
			created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`).Error)

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

func insertPublishedReferenceSkill(t *testing.T, db *gorm.DB, slug, sourceURL string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO skills (slug, name, description, status, listing_type, source_url, created_by)
		 VALUES (?, ?, ?, 'published', 'reference', ?, ?)`,
		slug, "Test Skill", "d", sourceURL, 1,
	).Error)
}

func TestDownloadMarketplaceSkill_ReferenceListing_Returns400(t *testing.T) {
	db := setupDownloadControllerTestDB(t)
	insertPublishedReferenceSkill(t, db, "ref-skill", "https://github.com/o/r")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	ctx.Params = gin.Params{{Key: "slug", Value: "ref-skill"}}
	ctx.Set("id", 1)

	DownloadMarketplaceSkill(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	resp := decodeMarketplaceResponse(t, recorder)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "reference")
}

func TestDownloadMarketplaceSkill_NotFound_Returns404(t *testing.T) {
	setupDownloadControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	ctx.Params = gin.Params{{Key: "slug", Value: "does-not-exist"}}
	ctx.Set("id", 1)

	DownloadMarketplaceSkill(ctx)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
