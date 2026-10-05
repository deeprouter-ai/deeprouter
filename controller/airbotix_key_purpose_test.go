package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/alias_setting"
	"github.com/stretchr/testify/require"
)

func TestListModelsMediaKeyCreateUpdateAndDirectory(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	seedListModelsUser(t, db, 7101, "default", false)
	withTieredBillingConfig(t,
		map[string]string{"new-video-a": "tiered_expr", "new-video-b": "tiered_expr", "other-group-video": "tiered_expr", "gpt-image-1": "tiered_expr", "eleven_multilingual_v2": "tiered_expr"},
		map[string]string{"new-video-a": "p", "new-video-b": "p", "other-group-video": "p", "gpt-image-1": "p", "eleven_multilingual_v2": "p"})
	withSelfUseModeDisabled(t)
	for i, row := range []struct {
		name, group string
		channel     int
	}{
		{"new-video-a", "default", constant.ChannelTypeDoubaoVideo},
		{"new-video-b", "default", constant.ChannelTypeSora},
		{"other-group-video", "other", constant.ChannelTypeSora},
		{"unpriced-video", "default", constant.ChannelTypeSora},
		{"gpt-4o-mini", "default", constant.ChannelTypeOpenAI},
		{"gpt-image-1", "default", constant.ChannelTypeOpenAI},
		{"eleven_multilingual_v2", "default", constant.ChannelTypeElevenLabs},
	} {
		require.NoError(t, db.Create(&model.Channel{Id: i + 1, Type: row.channel, Status: common.ChannelStatusEnabled, Name: row.name}).Error)
		require.NoError(t, db.Create(&model.Ability{ChannelId: i + 1, Group: row.group, Model: row.name, Enabled: true}).Error)
	}
	model.InvalidatePricingCache()
	t.Cleanup(model.InvalidatePricingCache)
	for purpose, want := range map[string][]string{"video": {"new-video-a", "new-video-b"}, "image": {"gpt-image-1"}, "voice": {"eleven_multilingual_v2"}} {
		got, err := mediaModelsForUser(7101, purpose)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	require.NoError(t, alias_setting.InitAliasSettings())
	purposeCtx, purposeRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self/purposes", nil, 7101)
	GetApiKeyPurposes(purposeCtx)
	var metadata struct {
		Purposes []alias_setting.PurposeSummary `json:"purposes"`
	}
	require.NoError(t, common.Unmarshal(decodeAPIResponse(t, purposeRecorder).Data, &metadata))
	for _, card := range metadata.Purposes {
		if card.ID == "video" {
			require.NotNil(t, card.Available)
			require.True(t, *card.Available)
			require.Equal(t, []string{"new-video-a", "new-video-b"}, card.AvailableModels)
			require.Empty(t, card.AvailableBrands)
		}
	}
	body := map[string]any{"name": "video-key", "simple_purpose": "video", "unlimited_quota": true, "expired_time": -1, "group": "other", "cross_group_retry": true}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 7101)
	AddToken(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success, recorder.Body.String())
	var token model.Token
	require.NoError(t, db.Where("user_id = ?", 7101).First(&token).Error)
	require.Equal(t, "new-video-a,new-video-b", token.ModelLimits)
	require.True(t, token.ModelLimitsEnabled)
	require.Empty(t, token.Group)
	require.False(t, token.CrossGroupRetry)
	// Listing and relay permissions agree on these exact models.
	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/v1/models", nil, 7101)
	ctx.Set(string(constant.ContextKeyTokenModelLimitEnabled), true)
	ctx.Set(string(constant.ContextKeyTokenModelLimit), token.GetModelLimitsMap())
	ListModels(ctx, constant.ChannelTypeOpenAI)
	require.Len(t, decodeListModelsResponse(t, recorder), 2)
	// Changing the purpose through the real update path refreshes permissions.
	body["id"] = token.Id
	body["simple_purpose"] = "voice"
	ctx, recorder = newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 7101)
	UpdateToken(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success, recorder.Body.String())
	require.NoError(t, db.First(&token, token.Id).Error)
	require.Equal(t, "eleven_multilingual_v2", token.ModelLimits)
}

func TestListModelsMediaKeyEmptyCatalogFailsClosed(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	seedListModelsUser(t, db, 7102, "empty", false)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", map[string]any{"simple_purpose": "video", "unlimited_quota": true}, 7102)
	AddToken(ctx)
	require.False(t, decodeAPIResponse(t, recorder).Success)
	var count int64
	require.NoError(t, db.Model(&model.Token{}).Count(&count).Error)
	require.Zero(t, count)
}

// The cards are rendered in the page's UI language, which the saved user
// setting lags after a language switch — so an explicit ?lang= must beat
// whatever the request context resolves to (here an Accept-Language of zh-CN).
func TestListModelsPurposeCardsFollowExplicitLang(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	seedListModelsUser(t, db, 7103, "default", false)
	require.NoError(t, alias_setting.InitAliasSettings())

	chatLabel := func(target string) string {
		ctx, recorder := newAuthenticatedContext(t, http.MethodGet, target, nil, 7103)
		ctx.Request.Header.Set("Accept-Language", "zh-CN")
		GetApiKeyPurposes(ctx)
		var metadata struct {
			Purposes []alias_setting.PurposeSummary `json:"purposes"`
		}
		require.NoError(t, common.Unmarshal(decodeAPIResponse(t, recorder).Data, &metadata))
		for _, card := range metadata.Purposes {
			if card.ID == "chat" {
				return card.Label
			}
		}
		t.Fatalf("chat card missing from %s", target)
		return ""
	}

	require.Equal(t, "聊天 / 写作", chatLabel("/api/user/self/api-key-purposes"))
	require.Equal(t, "Chat / Writing", chatLabel("/api/user/self/api-key-purposes?lang=en"))
	require.Equal(t, "聊天 / 写作", chatLabel("/api/user/self/api-key-purposes?lang=zh"))
}
