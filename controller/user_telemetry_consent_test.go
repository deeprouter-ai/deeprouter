package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type telemetryConsentAPIResponse struct {
	Success bool                     `json:"success"`
	Message string                   `json:"message"`
	Data    TelemetryConsentResponse `json:"data"`
}

func TestTelemetryConsentCurrentUserCanReadEnableAndDisable(t *testing.T) {
	db := setupTelemetryConsentTestDB(t)
	createTelemetryConsentTestUser(t, db, 42, false)

	ctx, recorder := telemetryConsentContext(t, http.MethodGet, "/api/user/telemetry-consent", nil, 42)
	GetTelemetryConsent(ctx)
	response := decodeTelemetryConsentResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message=%q", response.Message)
	}
	if response.Data.Tier2TelemetryConsent {
		t.Fatal("new test user should start without telemetry consent")
	}
	if response.Data.Tier2TelemetryConsentedAt != nil {
		t.Fatal("new test user should start without consent timestamp")
	}

	ctx, recorder = telemetryConsentContext(t, http.MethodPut, "/api/user/telemetry-consent", map[string]any{
		"tier2_telemetry_consent": true,
	}, 42)
	UpdateTelemetryConsent(ctx)
	response = decodeTelemetryConsentResponse(t, recorder)
	if !response.Success || !response.Data.Tier2TelemetryConsent {
		t.Fatalf("expected consent enabled, response=%+v", response)
	}
	if response.Data.Tier2TelemetryConsentedAt == nil {
		t.Fatal("enabling consent should set consent timestamp")
	}

	ctx, recorder = telemetryConsentContext(t, http.MethodPut, "/api/user/telemetry-consent", map[string]any{
		"tier2_telemetry_consent": false,
	}, 42)
	UpdateTelemetryConsent(ctx)
	response = decodeTelemetryConsentResponse(t, recorder)
	if !response.Success || response.Data.Tier2TelemetryConsent {
		t.Fatalf("expected consent disabled, response=%+v", response)
	}
	if response.Data.Tier2TelemetryConsentedAt == nil {
		t.Fatal("disabling consent should retain prior consent timestamp for audit context")
	}
}

func TestTelemetryConsentRejectsMissingConsentValue(t *testing.T) {
	db := setupTelemetryConsentTestDB(t)
	createTelemetryConsentTestUser(t, db, 42, false)

	ctx, recorder := telemetryConsentContext(t, http.MethodPut, "/api/user/telemetry-consent", map[string]any{}, 42)
	UpdateTelemetryConsent(ctx)
	response := decodeTelemetryConsentResponse(t, recorder)
	if response.Success {
		t.Fatal("missing tier2_telemetry_consent should fail")
	}
}

func TestTelemetryConsentOnlyUpdatesCurrentUser(t *testing.T) {
	db := setupTelemetryConsentTestDB(t)
	createTelemetryConsentTestUser(t, db, 42, false)
	createTelemetryConsentTestUser(t, db, 99, false)

	ctx, recorder := telemetryConsentContext(t, http.MethodPut, "/api/user/telemetry-consent", map[string]any{
		"tier2_telemetry_consent": true,
		"user_id":                 99,
	}, 42)
	UpdateTelemetryConsent(ctx)
	response := decodeTelemetryConsentResponse(t, recorder)
	if !response.Success || !response.Data.Tier2TelemetryConsent {
		t.Fatalf("expected current user consent enabled, response=%+v", response)
	}

	var other model.User
	if err := db.Select("id", "tier2_telemetry_consent").Where("id = ?", 99).First(&other).Error; err != nil {
		t.Fatalf("load other user: %v", err)
	}
	if other.Tier2TelemetryConsent {
		t.Fatal("request body user_id must not allow updating another user's consent")
	}
}

func TestAdminUpdateUserCannotEnableTelemetryConsent(t *testing.T) {
	db := setupTelemetryConsentTestDB(t)
	createTelemetryConsentTestUser(t, db, 42, false)

	ctx, recorder := telemetryConsentContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":                      42,
		"username":                "telemetry-user-42",
		"display_name":            "Updated User",
		"role":                    common.RoleCommonUser,
		"status":                  common.UserStatusEnabled,
		"group":                   "default",
		"tier2_telemetry_consent": true,
	}, 1)
	ctx.Set("role", common.RoleRootUser)
	UpdateUser(ctx)
	response := decodeTelemetryConsentResponse(t, recorder)
	if !response.Success {
		t.Fatalf("admin update should otherwise succeed, message=%q", response.Message)
	}

	var user model.User
	if err := db.Select("id", "tier2_telemetry_consent").Where("id = ?", 42).First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.Tier2TelemetryConsent {
		t.Fatal("admin user update must not enable another user's telemetry consent")
	}
}

func setupTelemetryConsentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
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

func createTelemetryConsentTestUser(t *testing.T, db *gorm.DB, id int, consent bool) {
	t.Helper()
	user := model.User{
		Id:                    id,
		Username:              fmt.Sprintf("telemetry-user-%d", id),
		DisplayName:           "Telemetry User",
		Password:              "hashed",
		Status:                common.UserStatusEnabled,
		Role:                  common.RoleCommonUser,
		Group:                 "default",
		AffCode:               fmt.Sprintf("aff-%d", id),
		Tier2TelemetryConsent: consent,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %d: %v", id, err)
	}
}

func telemetryConsentContext(t *testing.T, method, target string, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("id", userID)
	return ctx, recorder
}

func decodeTelemetryConsentResponse(t *testing.T, recorder *httptest.ResponseRecorder) telemetryConsentAPIResponse {
	t.Helper()
	var response telemetryConsentAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}
