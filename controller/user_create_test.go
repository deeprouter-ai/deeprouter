package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type createUserAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func TestRootCanCreateCommonAndAdminUsers(t *testing.T) {
	db := setupCreateUserTestDB(t)

	for _, tc := range []struct {
		name string
		role int
	}{
		{name: "common", role: common.RoleCommonUser},
		{name: "admin", role: common.RoleAdminUser},
	} {
		t.Run(tc.name, func(t *testing.T) {
			username := "created-" + tc.name
			ctx, recorder := createUserContext(t, map[string]any{
				"username":     username,
				"display_name": "Created " + tc.name,
				"password":     "password123",
				"role":         tc.role,
			}, common.RoleRootUser)

			CreateUser(ctx)
			response := decodeCreateUserResponse(t, recorder)
			if !response.Success {
				t.Fatalf("expected create success, message=%q", response.Message)
			}

			var user model.User
			if err := db.Where("username = ?", username).First(&user).Error; err != nil {
				t.Fatalf("load created user: %v", err)
			}
			if user.Role != tc.role {
				t.Fatalf("expected role %d, got %d", tc.role, user.Role)
			}
			if user.Group != "default" {
				t.Fatalf("expected default group, got %q", user.Group)
			}
			if user.Password == "password123" {
				t.Fatal("created password should be hashed before storage")
			}
		})
	}
}

func TestRootCannotCreateAnotherRootUser(t *testing.T) {
	setupCreateUserTestDB(t)

	ctx, recorder := createUserContext(t, map[string]any{
		"username":     "created-root",
		"display_name": "Created Root",
		"password":     "password123",
		"role":         common.RoleRootUser,
	}, common.RoleRootUser)

	CreateUser(ctx)
	response := decodeCreateUserResponse(t, recorder)
	if response.Success {
		t.Fatal("root caller must not be allowed to create another root user")
	}
}

func setupCreateUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
	})
	return db
}

func createUserContext(t *testing.T, body map[string]any, role int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	payload, err := common.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("role", role)
	return ctx, recorder
}

func decodeCreateUserResponse(t *testing.T, recorder *httptest.ResponseRecorder) createUserAPIResponse {
	t.Helper()
	var response createUserAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}
