package controller

import (
	"maps"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/alias_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Regression tests for the purpose, brand and price tier of a Simple-mode key
// (meta-repo docs/adlc/tasks/fix-key-purpose-validation-and-whitelists-task.md).
// None of the three was checked on the server. A purpose the registry does not
// know found no rule list, "no rule list" was read as "no limit", and one
// mistyped word made a key that could call every model.

// keyPurposeDB is a database with the key table and one personal account, and
// the purpose registry loaded.
func keyPurposeDB(t *testing.T, userID int) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	seedListModelsUser(t, db, userID, "default", false)
	require.NoError(t, alias_setting.InitAliasSettings())
	return db
}

// keyBody is a request to create or change a key with the given binding.
func keyBody(binding map[string]any) map[string]any {
	body := map[string]any{"name": "a key", "unlimited_quota": true, "expired_time": -1}
	maps.Copy(body, binding)
	return body
}

// unknownBindings are bindings with a value the registry does not know, and
// the refusal each is answered with.
var unknownBindings = map[string]struct {
	binding map[string]any
	refusal string
}{
	"a mistyped purpose":             {map[string]any{"simple_purpose": "codng"}, "token.purpose_unknown"},
	"a persona, which is no purpose": {map[string]any{"simple_purpose": "dev"}, "token.purpose_unknown"},
	"a purpose in capitals":          {map[string]any{"simple_purpose": "Chat"}, "token.purpose_unknown"},
	"a mistyped brand":               {map[string]any{"simple_purpose": "chat", "simple_brand": "claud"}, "token.brand_unknown"},
	"a mistyped price tier":          {map[string]any{"simple_purpose": "all", "simple_price_tier": "standrad"}, "token.price_tier_unknown"},
	"a brand with no purpose":        {map[string]any{"simple_brand": "nobody"}, "token.brand_unknown"},
	"a price tier with no purpose":   {map[string]any{"simple_price_tier": "free"}, "token.price_tier_unknown"},
	"a price tier nothing reads":     {map[string]any{"simple_purpose": "chat", "simple_price_tier": "free"}, "token.price_tier_unknown"},
}

// Acceptance: 创建 … key 时，不认识的用途、品牌或价格档被拒绝并返回明确的错误，不会得到一把
// 不受限的 key. Red before the fix: every one of these answered success and
// left a key behind — the mistyped purpose and the mistyped tier a key with no
// model limit at all.
func TestKeyPurpose_UnknownValuesAreRefusedOnCreate(t *testing.T) {
	db := keyPurposeDB(t, 7201)
	for name, unknown := range unknownBindings {
		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", keyBody(unknown.binding), 7201)
		AddToken(ctx)
		response := decodeAPIResponse(t, recorder)
		require.False(t, response.Success, name)
		require.Equal(t, unknown.refusal, response.Message, name)
	}
	var keys int64
	require.NoError(t, db.Model(&model.Token{}).Count(&keys).Error)
	require.Zero(t, keys, "a refused request leaves no key behind")
}

// Acceptance: … 修改 key 时 …. A key that is limited to the chat models stays
// exactly that when a change names a value nobody knows: it is neither widened
// nor rebound.
func TestKeyPurpose_UnknownValuesAreRefusedOnUpdate(t *testing.T) {
	db := keyPurposeDB(t, 7202)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", keyBody(map[string]any{"simple_purpose": "chat", "simple_brand": "claude"}), 7202)
	AddToken(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success, recorder.Body.String())
	var before model.Token
	require.NoError(t, db.Where("user_id = ?", 7202).First(&before).Error)
	require.True(t, before.ModelLimitsEnabled)
	require.NotEmpty(t, before.ModelLimits)

	for name, unknown := range unknownBindings {
		body := keyBody(unknown.binding)
		body["id"] = before.Id
		ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 7202)
		UpdateToken(ctx)
		response := decodeAPIResponse(t, recorder)
		require.False(t, response.Success, name)
		require.Equal(t, unknown.refusal, response.Message, name)

		var after model.Token
		require.NoError(t, db.First(&after, before.Id).Error)
		require.Equal(t, before.ModelLimits, after.ModelLimits, name)
		require.True(t, after.ModelLimitsEnabled, name)
		require.Equal(t, "chat", after.SimplePurpose, name)
		require.Equal(t, "claude", after.SimpleBrand, name)
	}
}

// What was allowed stays allowed: every registered purpose, brand and price
// tier, a key with no binding at all — and "everything" at the top tier, which
// is the one binding that means no model limit and says so on its card.
func TestKeyPurpose_KnownValuesAreStillAccepted(t *testing.T) {
	db := keyPurposeDB(t, 7203)
	for name, known := range map[string]struct {
		binding map[string]any
		limited bool
	}{
		"no binding: an Advanced-mode key":  {map[string]any{}, false},
		"chat":                              {map[string]any{"simple_purpose": "chat"}, true},
		"coding, with a brand":              {map[string]any{"simple_purpose": "coding", "simple_brand": "deepseek"}, true},
		"chat, with every brand it offers":  {map[string]any{"simple_purpose": "chat", "simple_brand": "gemini"}, true},
		"everything, at the default tier":   {map[string]any{"simple_purpose": "all"}, true},
		"everything, economy":               {map[string]any{"simple_purpose": "all", "simple_price_tier": "economy"}, true},
		"everything, premium, with a brand": {map[string]any{"simple_purpose": "all", "simple_price_tier": "premium", "simple_brand": "openai"}, true},
		"everything at the top tier":        {map[string]any{"simple_purpose": "all", "simple_price_tier": "ultra"}, false},
	} {
		require.NoError(t, db.Where("user_id = ?", 7203).Delete(&model.Token{}).Error)
		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", keyBody(known.binding), 7203)
		AddToken(ctx)
		require.True(t, decodeAPIResponse(t, recorder).Success, "%s: %s", name, recorder.Body.String())
		var key model.Token
		require.NoError(t, db.Where("user_id = ?", 7203).First(&key).Error, name)
		require.Equal(t, known.limited, key.ModelLimitsEnabled, name)
		require.Equal(t, known.limited, key.ModelLimits != "", name)
	}
}

// The starter key a deployment may hand out at sign-up (GENERATE_DEFAULT_TOKEN)
// copied the persona the wizard captured — casual, dev, team — into the key's
// purpose, where nothing recognises it, and any brand it was sent. A persona
// is not a purpose: the key is bound to none, and keeps only a brand the
// registry knows.
func TestKeyPurpose_StarterKeyIsBoundToNoPersona(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		setForTest(t, &constant.GenerateDefaultToken, true)
		require.NoError(t, alias_setting.InitAliasSettings())
		for username, want := range map[string]struct {
			persona string
			brand   string
			keeps   string
		}{
			"casual-claude": {"casual", "claude", "claude"},
			"dev-deepseek":  {"dev", "deepseek", "deepseek"},
			"team-nobody":   {"team", "nobody", ""},
			"chat-as-such":  {"chat", "", ""},
			"plain":         {"", "", ""},
		} {
			response, _ := env.register(t, map[string]any{
				"username": username, "password": "password123",
				"persona": want.persona, "brand_preference": want.brand,
			})
			require.True(t, response.Success, "%s: %s", username, response.Message)
			var key model.Token
			require.NoError(t, env.db.Where("user_id = ?", env.userByName(t, username).Id).First(&key).Error, username)
			require.Empty(t, key.SimplePurpose, username)
			require.Equal(t, want.keeps, key.SimpleBrand, username)
			require.False(t, key.ModelLimitsEnabled, "%s: a starter key is not limited to models", username)
		}
	})
}
