package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/alias_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P5 (meta-repo docs/enterprise-org-prd.md §3): an
// organization's keys over HTTP — the personal key endpoints that are closed
// to them, and the organization endpoints that are the only way to change them.

// seedKey writes a key held by holder straight into the table: an organization
// key when orgID is not zero, a personal one otherwise.
func (env orgTestEnv) seedKey(t *testing.T, holder model.User, orgID int, name string) model.Token {
	t.Helper()
	value, err := common.GenerateKey()
	require.NoError(t, err)
	key := model.Token{
		UserId:       holder.Id,
		OrgId:        orgID,
		Name:         name,
		Key:          value,
		Status:       common.TokenStatusEnabled,
		CreatedTime:  1700000000,
		AccessedTime: 1700000000,
		ExpiredTime:  -1,
		RemainQuota:  750,
		UsedQuota:    250,
	}
	require.NoError(t, env.db.Create(&key).Error)
	return env.key(t, key.Id)
}

// key reads a key row back from the database, deleted or not.
func (env orgTestEnv) key(t *testing.T, id int) model.Token {
	t.Helper()
	var key model.Token
	require.NoError(t, env.db.Unscoped().First(&key, id).Error)
	return key
}

// callAt runs a handler as userID against a path of choice — the personal key
// handlers read a query string — and returns the envelope and the raw response.
func callAt(t *testing.T, handler gin.HandlerFunc, method string, target string, body any, userID int, params ...gin.Param) (tokenAPIResponse, *httptest.ResponseRecorder) {
	t.Helper()
	ctx, recorder := newAuthenticatedContext(t, method, target, body, userID)
	ctx.Params = params
	handler(ctx)
	return decodeAPIResponse(t, recorder), recorder
}

// Acceptance: 组织 key 的持有人无法通过个人 key 端点修改、解冻、删除或取回该 key
// （额度、限流、模型白名单、状态均不可改）；组织 key 的变更只能走组织端点 — and:
// 上游"取完整 key"端点对组织 key 返回拒绝.
func TestOrgKeys_PersonalEndpointsAreClosedToOrganizationKeys(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		alice := env.seedMember(t, owner, "alice", orgmodel.RoleStaff)
		// A key handed to a member, frozen and limited by an administrator —
		// everything its holder might want to undo.
		teamKey := env.seedKey(t, alice, owner.OrgId, "team key")
		require.NoError(t, env.db.Model(&model.Token{}).Where("id = ?", teamKey.Id).Updates(map[string]any{
			"status": common.TokenStatusDisabled, "model_limits_enabled": true, "model_limits": "claude-*", "rpm_limit": 10,
		}).Error)
		teamKey = env.key(t, teamKey.Id)
		ownersKey := env.seedKey(t, owner, owner.OrgId, "the owner's key")
		// A member makes no personal key anymore (P6), but one made before
		// then may still sit next to the organization's.
		alicesOwn := env.seedKey(t, alice, 0, "alice's own")
		ownersOwn := env.seedKey(t, owner, 0, "the owner's own")

		for _, holder := range []struct {
			user     model.User
			orgKey   model.Token
			personal model.Token
		}{{alice, teamKey, alicesOwn}, {owner, ownersKey, ownersOwn}} {
			id := strconv.Itoa(holder.orgKey.Id)
			both := map[string]any{"ids": []int{holder.personal.Id, holder.orgKey.Id}}
			attempts := map[string]func() (tokenAPIResponse, *httptest.ResponseRecorder){
				"read its value": func() (tokenAPIResponse, *httptest.ResponseRecorder) {
					return callAt(t, GetTokenKey, http.MethodPost, "/api/token/"+id+"/key", nil, holder.user.Id, idParam(holder.orgKey.Id))
				},
				"read its value in a batch": func() (tokenAPIResponse, *httptest.ResponseRecorder) {
					return callAt(t, GetTokenKeysBatch, http.MethodPost, "/api/token/batch/keys", both, holder.user.Id)
				},
				"raise its quota and lift its limits": func() (tokenAPIResponse, *httptest.ResponseRecorder) {
					return callAt(t, UpdateToken, http.MethodPut, "/api/token/", map[string]any{
						"id": holder.orgKey.Id, "name": "mine now", "status": common.TokenStatusEnabled,
						"remain_quota": 999999, "unlimited_quota": true, "expired_time": -1,
						"model_limits_enabled": false, "model_limits": "", "rpm_limit": 0, "tpm_limit": 0, "monthly_limit": 0,
					}, holder.user.Id)
				},
				"unfreeze it": func() (tokenAPIResponse, *httptest.ResponseRecorder) {
					return callAt(t, UpdateToken, http.MethodPut, "/api/token/?status_only=true",
						map[string]any{"id": holder.orgKey.Id, "status": common.TokenStatusEnabled}, holder.user.Id)
				},
				"freeze it": func() (tokenAPIResponse, *httptest.ResponseRecorder) {
					return callAt(t, UpdateToken, http.MethodPut, "/api/token/?status_only=true",
						map[string]any{"id": holder.orgKey.Id, "status": common.TokenStatusDisabled}, holder.user.Id)
				},
				"delete it": func() (tokenAPIResponse, *httptest.ResponseRecorder) {
					return callAt(t, DeleteToken, http.MethodDelete, "/api/token/"+id, nil, holder.user.Id, idParam(holder.orgKey.Id))
				},
				"delete it in a batch": func() (tokenAPIResponse, *httptest.ResponseRecorder) {
					return callAt(t, DeleteTokenBatch, http.MethodPost, "/api/token/batch", both, holder.user.Id)
				},
			}
			for name, attempt := range attempts {
				response, recorder := attempt()
				require.Equal(t, http.StatusForbidden, recorder.Code, "%s: %s", holder.user.Username, name)
				require.False(t, response.Success, "%s: %s", holder.user.Username, name)
				require.Equal(t, msgOrgKeyManagedByOrg, response.Message, "%s: %s", holder.user.Username, name)
				require.NotContains(t, recorder.Body.String(), holder.orgKey.Key, "%s: %s", holder.user.Username, name)
				// A batch that names an organization key is refused whole: the
				// personal key beside it is neither revealed nor deleted.
				require.NotContains(t, recorder.Body.String(), holder.personal.Key, "%s: %s", holder.user.Username, name)
				require.Equal(t, holder.orgKey, env.key(t, holder.orgKey.Id), "%s: %s", holder.user.Username, name)
				require.Equal(t, holder.personal, env.key(t, holder.personal.Id), "%s: %s", holder.user.Username, name)
			}

			// Looking is still theirs: the key is listed, masked, and says it is
			// the organization's.
			listed, recorder := callAt(t, GetAllTokens, http.MethodGet, "/api/token/?p=1&size=10", nil, holder.user.Id)
			require.True(t, listed.Success, listed.Message)
			require.Contains(t, recorder.Body.String(), `"org_id":`+strconv.Itoa(owner.OrgId))
			require.Contains(t, recorder.Body.String(), model.MaskTokenKey(holder.orgKey.Key))
			require.NotContains(t, recorder.Body.String(), holder.orgKey.Key)
			one, recorder := callAt(t, GetToken, http.MethodGet, "/api/token/"+id, nil, holder.user.Id, idParam(holder.orgKey.Id))
			require.True(t, one.Success, one.Message)
			require.NotContains(t, recorder.Body.String(), holder.orgKey.Key)
		}

		// A key that is not the caller's is "not found", as it always was: the
		// refusal above is only ever told to the holder.
		bob := env.seedMember(t, owner, "bob", orgmodel.RoleStaff)
		solo := env.seedUser(t, "solo")
		for _, stranger := range []model.User{bob, solo} {
			response, recorder := callAt(t, GetTokenKey, http.MethodPost, "/api/token/x/key", nil, stranger.Id, idParam(teamKey.Id))
			require.False(t, response.Success, stranger.Username)
			require.Equal(t, http.StatusOK, recorder.Code, stranger.Username)
			require.NotEqual(t, msgOrgKeyManagedByOrg, response.Message, stranger.Username)
			require.NotContains(t, recorder.Body.String(), teamKey.Key, stranger.Username)
		}
	})
}

// The other half of the same acceptance item, and the regression baseline of
// the whole feature: a personal key answers every one of those endpoints
// exactly as before — a member's as much as a personal account's.
func TestOrgKeys_PersonalEndpointsStillServePersonalKeys(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		for _, user := range []model.User{env.seedMember(t, owner, "alice", orgmodel.RoleStaff), env.seedUser(t, "solo")} {
			key := env.seedKey(t, user, 0, "mine")
			spare := env.seedKey(t, user, 0, "spare")
			id := strconv.Itoa(key.Id)

			response, _ := callAt(t, GetTokenKey, http.MethodPost, "/api/token/"+id+"/key", nil, user.Id, idParam(key.Id))
			require.True(t, response.Success, response.Message)
			require.JSONEq(t, `{"key":"`+key.Key+`"}`, string(response.Data))

			response, _ = callAt(t, UpdateToken, http.MethodPut, "/api/token/", map[string]any{
				"id": key.Id, "name": "renamed", "remain_quota": 5000, "expired_time": -1, "rpm_limit": 7,
			}, user.Id)
			require.True(t, response.Success, response.Message)
			changed := env.key(t, key.Id)
			require.Equal(t, []any{"renamed", 5000, 7}, []any{changed.Name, changed.RemainQuota, changed.RpmLimit})

			response, _ = callAt(t, UpdateToken, http.MethodPut, "/api/token/?status_only=true",
				map[string]any{"id": key.Id, "status": common.TokenStatusDisabled}, user.Id)
			require.True(t, response.Success, response.Message)
			require.Equal(t, common.TokenStatusDisabled, env.key(t, key.Id).Status)

			response, _ = callAt(t, DeleteTokenBatch, http.MethodPost, "/api/token/batch", map[string]any{"ids": []int{spare.Id}}, user.Id)
			require.True(t, response.Success, response.Message)
			require.JSONEq(t, `1`, string(response.Data))
			response, _ = callAt(t, DeleteToken, http.MethodDelete, "/api/token/"+id, nil, user.Id, idParam(key.Id))
			require.True(t, response.Success, response.Message)
			require.Zero(t, env.count(t, &model.Token{}, "user_id = ?", user.Id), user.Username)
		}
	})
}

// The batch reveal is the one personal endpoint whose query names the key
// column in the database's own quoting, which only the gateway's start-up
// sets. Its happy path is checked here, where that has run.
func TestOrgKeys_PersonalBatchRevealStillServesPersonalKeys(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	const userID = 7401
	mine := model.Token{UserId: userID, Name: "mine", Key: "personal-key-one", Status: common.TokenStatusEnabled}
	spare := model.Token{UserId: userID, Name: "spare", Key: "personal-key-two", Status: common.TokenStatusEnabled}
	teamKey := model.Token{UserId: userID, OrgId: 9, Name: "team key", Key: "organization-key", Status: common.TokenStatusEnabled}
	for _, key := range []*model.Token{&mine, &spare, &teamKey} {
		require.NoError(t, db.Create(key).Error)
	}

	response, _ := callAt(t, GetTokenKeysBatch, http.MethodPost, "/api/token/batch/keys", map[string]any{"ids": []int{mine.Id, spare.Id}}, userID)
	require.True(t, response.Success, response.Message)
	require.JSONEq(t, `{"keys":{"`+strconv.Itoa(mine.Id)+`":"personal-key-one","`+strconv.Itoa(spare.Id)+`":"personal-key-two"}}`, string(response.Data))

	// One organization key among them closes the whole batch.
	response, recorder := callAt(t, GetTokenKeysBatch, http.MethodPost, "/api/token/batch/keys", map[string]any{"ids": []int{mine.Id, teamKey.Id}}, userID)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, msgOrgKeyManagedByOrg, response.Message)
	require.NotContains(t, recorder.Body.String(), "organization-key")
	require.NotContains(t, recorder.Body.String(), "personal-key-one")
}

// Acceptance: 组织 key 的明文不在任何界面显示 … 服务账号的 key 在归到服务账号名下时
// 向操作者展示一次 — as the organization endpoints answer it.
func TestOrgKeys_AnswerTheValueOnlyForAServiceAccount(t *testing.T) {
	forEachOrgDialect(t, func(t *testing.T, env orgTestEnv) {
		owner := env.seedOwner(t, "founder", "Acme")
		alice := env.seedMember(t, owner, "alice", orgmodel.RoleStaff)
		general := env.department(t, owner.OrgId, "General")
		created, _ := callOrg(t, CreateOrgServiceAccount, http.MethodPost, map[string]any{"name": "CI"}, owner.Id)
		require.True(t, created.Success, created.Message)
		var bot orgservice.MemberView
		require.NoError(t, common.Unmarshal(created.Data, &bot))

		type grant struct {
			Id     int    `json:"id"`
			Key    string `json:"key"`
			Value  string `json:"value"`
			Holder string `json:"holder"`
		}
		// answered runs a handler that may answer with a key's value, and
		// returns what it said next to what the database holds.
		answered := func(handler gin.HandlerFunc, body any, params ...gin.Param) (grant, model.Token, string) {
			response, status := callOrg(t, handler, http.MethodPost, body, owner.Id, params...)
			require.True(t, response.Success, response.Message)
			require.Equal(t, http.StatusOK, status)
			var answer grant
			require.NoError(t, common.Unmarshal(response.Data, &answer))
			return answer, env.key(t, answer.Id), string(response.Data)
		}

		// A person's key: masked, and no value in the answer at all.
		personal, stored, raw := answered(CreateOrgKey, map[string]any{"name": "For Alice", "holder_id": alice.Id, "unlimited_quota": true})
		require.Equal(t, "alice", personal.Holder)
		require.Empty(t, personal.Value)
		require.NotContains(t, raw, `"value"`)
		require.NotContains(t, raw, stored.Key)
		require.Equal(t, model.MaskTokenKey(stored.Key), personal.Key)
		require.Equal(t, []any{alice.Id, owner.OrgId, owner.Id}, []any{stored.UserId, stored.OrgId, stored.CreatedBy})

		// A service account's key: the value, this once.
		service, stored, _ := answered(CreateOrgKey, map[string]any{"name": "For CI", "holder_id": bot.Id, "unlimited_quota": true})
		require.Equal(t, "CI", service.Holder)
		require.Equal(t, stored.Key, service.Value)
		require.Len(t, service.Value, 48)
		require.Equal(t, model.MaskTokenKey(stored.Key), service.Key)

		// The same on rotation.
		before := env.key(t, personal.Id)
		rotated, stored, raw := answered(RotateOrgKey, nil, idParam(personal.Id))
		require.NotEqual(t, before.Key, stored.Key)
		require.Empty(t, rotated.Value)
		require.NotContains(t, raw, stored.Key)
		before = env.key(t, service.Id)
		rotated, stored, _ = answered(RotateOrgKey, nil, idParam(service.Id))
		require.NotEqual(t, before.Key, stored.Key)
		require.Equal(t, stored.Key, rotated.Value)

		// Afterwards nothing shows a value again: not the list, not a change.
		listed, _ := callOrg(t, ListOrgKeys, http.MethodGet, nil, owner.Id)
		require.True(t, listed.Success, listed.Message)
		updated, _ := callOrg(t, UpdateOrgKey, http.MethodPut, map[string]any{"name": "For the pipeline"}, owner.Id, idParam(service.Id))
		require.True(t, updated.Success, updated.Message)
		for _, id := range []int{personal.Id, service.Id} {
			require.NotContains(t, string(listed.Data), env.key(t, id).Key)
			require.NotContains(t, string(updated.Data), env.key(t, id).Key)
		}
		var views []orgservice.KeyView
		require.NoError(t, common.Unmarshal(listed.Data, &views))
		require.Len(t, views, 2)
		require.Equal(t, []any{service.Id, "CI", true, general.Id}, []any{views[0].Id, views[0].Holder, views[0].HolderIsService, views[0].DepartmentId})

		// What the key form is built from.
		templates, _ := callOrg(t, ListOrgKeyTemplates, http.MethodGet, nil, owner.Id)
		require.JSONEq(t, `[{"key":"creative","purposes":["image","video","chat"]},{"key":"coding","purposes":["coding"]}]`, string(templates.Data))
		holders, _ := callOrg(t, ListOrgKeyHolders, http.MethodGet, nil, owner.Id)
		department := `"department_id":` + strconv.Itoa(general.Id) + `,"department":"General"`
		staff := `"role_id":` + strconv.Itoa(alice.OrgRoleId) + `,"role":"staff"`
		require.JSONEq(t, `[
			{"id":`+strconv.Itoa(owner.Id)+`,"name":"founder",`+department+`,"role_id":`+strconv.Itoa(owner.OrgRoleId)+`,"role":"owner","is_service":false,"is_owner":true},
			{"id":`+strconv.Itoa(alice.Id)+`,"name":"alice",`+department+`,`+staff+`,"is_service":false,"is_owner":false},
			{"id":`+strconv.Itoa(bot.Id)+`,"name":"CI",`+department+`,`+staff+`,"is_service":true,"is_owner":false}
		]`, string(holders.Data))

		// Freezing, unfreezing and deleting answer with nothing but success.
		for _, handler := range []gin.HandlerFunc{FreezeOrgKey, UnfreezeOrgKey, DeleteOrgKey} {
			method := http.MethodPost
			response, status := callOrg(t, handler, method, nil, owner.Id, idParam(personal.Id))
			require.True(t, response.Success, response.Message)
			require.Equal(t, http.StatusOK, status)
			require.Equal(t, "null", string(response.Data))
		}
		require.True(t, env.key(t, personal.Id).DeletedAt.Valid)
	})
}

// orgWithCatalogue points the platform at a database holding a small model
// catalogue and an organization owned by a new user, and returns that owner's
// id. The catalogue is what a policy template's media purposes are read from:
// one image model and two video models in the owner's group, a video model
// that has no price, one in another group, and a voice model.
func orgWithCatalogue(t *testing.T, ownerID int) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	require.NoError(t, orgmodel.Migrate(db))
	seedListModelsUser(t, db, ownerID, "default", false)
	priced := map[string]string{}
	for _, name := range []string{"new-video-a", "new-video-b", "other-group-video", "gpt-image-1", "eleven_multilingual_v2"} {
		priced[name] = "tiered_expr"
	}
	exprs := map[string]string{}
	for name := range priced {
		exprs[name] = "p"
	}
	withTieredBillingConfig(t, priced, exprs)
	withSelfUseModeDisabled(t)
	for i, row := range []struct {
		name, group string
		channel     int
	}{
		{"new-video-a", "default", constant.ChannelTypeDoubaoVideo},
		{"new-video-b", "default", constant.ChannelTypeSora},
		{"other-group-video", "other", constant.ChannelTypeSora},
		{"unpriced-video", "default", constant.ChannelTypeSora},
		{"gpt-image-1", "default", constant.ChannelTypeOpenAI},
		{"eleven_multilingual_v2", "default", constant.ChannelTypeElevenLabs},
	} {
		require.NoError(t, db.Create(&model.Channel{Id: i + 1, Type: row.channel, Status: common.ChannelStatusEnabled, Name: row.name}).Error)
		require.NoError(t, db.Create(&model.Ability{ChannelId: i + 1, Group: row.group, Model: row.name, Enabled: true}).Error)
	}
	model.InvalidatePricingCache()
	t.Cleanup(model.InvalidatePricingCache)
	require.NoError(t, alias_setting.InitAliasSettings())
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := orgservice.CreateForOwnerTx(tx, ownerID, "Acme", "en")
		return err
	}))
	return db
}

// Acceptance: 创建 key 时可：选策略模板（一键填充模型白名单）. The template's
// purposes go through the sources a personal key's purpose goes through — the
// live catalogue for image and video, the rule table for chat and coding — and
// nothing here keeps a model list of its own.
func TestOrgKeys_PolicyTemplatesResolveThroughTheLiveCatalogue(t *testing.T) {
	const ownerID = 7201
	db := orgWithCatalogue(t, ownerID)

	create := func(template string) model.Token {
		response, _ := callOrg(t, CreateOrgKey, http.MethodPost, map[string]any{"name": "key", "policy_template": template, "unlimited_quota": true}, ownerID)
		require.True(t, response.Success, response.Message)
		var view orgservice.KeyView
		require.NoError(t, common.Unmarshal(response.Data, &view))
		var stored model.Token
		require.NoError(t, db.First(&stored, view.Id).Error)
		require.Equal(t, stored.GetModelLimits(), view.ModelLimits, "the answer shows what was stored")
		return stored
	}
	sortedUnion := func(lists ...[]string) []string {
		union := []string{}
		for _, list := range lists {
			for _, name := range list {
				if !slices.Contains(union, name) {
					union = append(union, name)
				}
			}
		}
		slices.Sort(union)
		return union
	}
	chatRules, limited := alias_setting.ModelWhitelistForToken("chat", "", "")
	require.True(t, limited)
	codingRules, limited := alias_setting.ModelWhitelistForToken("coding", "", "")
	require.True(t, limited)
	media := []string{"gpt-image-1", "new-video-a", "new-video-b"}

	// 创意生成包 = 图片 + 视频 + 对话.
	creative := create("creative")
	require.Equal(t, "creative", creative.PolicyTemplate)
	require.True(t, creative.ModelLimitsEnabled)
	require.Equal(t, sortedUnion(media, chatRules), creative.GetModelLimits())
	for _, name := range media {
		require.Contains(t, creative.GetModelLimits(), name)
	}
	// Only what this holder can actually be served: nothing unpriced, nothing
	// from another group — and no voice, which the template does not include.
	for _, name := range []string{"unpriced-video", "other-group-video", "eleven_multilingual_v2"} {
		require.NotContains(t, creative.GetModelLimits(), name)
	}
	require.True(t, model.MatchModelLimit(creative.GetModelLimitsMap(), "new-video-a"))
	require.False(t, model.MatchModelLimit(creative.GetModelLimitsMap(), "other-group-video"))

	// coding 包 = 编程.
	coding := create("coding")
	require.Equal(t, sortedUnion(codingRules), coding.GetModelLimits())
	for _, name := range media {
		require.NotContains(t, coding.GetModelLimits(), name)
	}

	// No template: every model.
	open := create("")
	require.False(t, open.ModelLimitsEnabled)
	require.Empty(t, open.ModelLimits)

	// Re-applying a template picks up what the catalogue gained since.
	require.NoError(t, db.Create(&model.Channel{Id: 50, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Name: "gpt-image-2"}).Error)
	require.NoError(t, db.Create(&model.Ability{ChannelId: 50, Group: "default", Model: "gpt-image-2", Enabled: true}).Error)
	withTieredBillingConfig(t,
		map[string]string{"new-video-a": "tiered_expr", "new-video-b": "tiered_expr", "gpt-image-1": "tiered_expr", "gpt-image-2": "tiered_expr"},
		map[string]string{"new-video-a": "p", "new-video-b": "p", "gpt-image-1": "p", "gpt-image-2": "p"})
	model.InvalidatePricingCache()
	response, _ := callOrg(t, UpdateOrgKey, http.MethodPut, map[string]any{"policy_template": "creative"}, ownerID, idParam(creative.Id))
	require.True(t, response.Success, response.Message)
	var reapplied model.Token
	require.NoError(t, db.First(&reapplied, creative.Id).Error)
	require.Equal(t, sortedUnion(media, []string{"gpt-image-2"}, chatRules), reapplied.GetModelLimits())

	// An unknown template is refused, and nothing is created.
	var before int64
	require.NoError(t, db.Model(&model.Token{}).Count(&before).Error)
	response, status := callOrg(t, CreateOrgKey, http.MethodPost, map[string]any{"name": "key", "policy_template": "everything"}, ownerID)
	require.False(t, response.Success)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, msgOrgKeyTemplateUnknown, response.Message)
	var after int64
	require.NoError(t, db.Model(&model.Token{}).Count(&after).Error)
	require.Equal(t, before, after)
}

// PRD §5 (D33): 不套模板时可以手动指定模型. What the form offers is what the
// holder can actually be served — read from the live catalogue, like a
// template's media purposes — and a hand-picked list is held to it.
func TestOrgKeys_AHandPickedListIsHeldToWhatTheHolderCanBeServed(t *testing.T) {
	const ownerID = 7501
	db := orgWithCatalogue(t, ownerID)
	// A model two channels serve is one model.
	require.NoError(t, db.Create(&model.Channel{Id: 60, Type: constant.ChannelTypeSora, Status: common.ChannelStatusEnabled, Name: "new-video-a again"}).Error)
	require.NoError(t, db.Create(&model.Ability{ChannelId: 60, Group: "default", Model: "new-video-a", Enabled: true}).Error)
	offered := func() string {
		response, _ := callAt(t, ListOrgKeyModels, http.MethodGet, "/api/org/key-models?holder_id="+strconv.Itoa(ownerID), nil, ownerID)
		require.True(t, response.Success, response.Message)
		return string(response.Data)
	}
	stored := func(id int) model.Token {
		var key model.Token
		require.NoError(t, db.First(&key, id).Error)
		return key
	}
	keyCount := func() int64 {
		var n int64
		require.NoError(t, db.Model(&model.Token{}).Count(&n).Error)
		return n
	}

	// Enabled in the owner's group and priced, in name order: nothing unpriced,
	// nothing from another group. Voice is a model like any other here.
	require.JSONEq(t, `["eleven_multilingual_v2","gpt-image-1","new-video-a","new-video-b"]`, offered())

	response, _ := callOrg(t, CreateOrgKey, http.MethodPost, map[string]any{
		"name": "Video only", "unlimited_quota": true, "model_limits": []string{"new-video-b", "new-video-a"},
	}, ownerID)
	require.True(t, response.Success, response.Message)
	var view orgservice.KeyView
	require.NoError(t, common.Unmarshal(response.Data, &view))
	require.Equal(t, []string{"new-video-a", "new-video-b"}, view.ModelLimits)
	require.Empty(t, view.PolicyTemplate)
	key := stored(view.Id)
	require.Empty(t, key.PolicyTemplate)
	require.True(t, key.ModelLimitsEnabled)
	require.Equal(t, "new-video-a,new-video-b", key.ModelLimits)
	require.True(t, model.MatchModelLimit(key.GetModelLimitsMap(), "new-video-b"))
	require.False(t, model.MatchModelLimit(key.GetModelLimitsMap(), "gpt-image-1"))

	// What the holder cannot be served is refused, and nothing is created.
	for name, list := range map[string][]string{
		"a model without a price":     {"unpriced-video"},
		"a model of another group":    {"new-video-a", "other-group-video"},
		"a model that does not exist": {"gpt-image-2"},
		"a rule":                      {"new-video-*"},
		"everything":                  {"*"},
	} {
		before := keyCount()
		response, status := callOrg(t, CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "model_limits": list}, ownerID)
		require.False(t, response.Success, name)
		require.Equal(t, http.StatusOK, status, name)
		require.Equal(t, msgOrgKeyModelsUnavailable, response.Message, name)
		require.Equal(t, before, keyCount(), name)
	}
	// So is a template and a list at once.
	response, status := callOrg(t, CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "policy_template": "creative", "model_limits": []string{"new-video-a"}}, ownerID)
	require.False(t, response.Success)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, msgOrgKeyModelsWithTemplate, response.Message)

	change := func(body map[string]any) tokenAPIResponse {
		response, _ := callOrg(t, UpdateOrgKey, http.MethodPut, body, ownerID, idParam(view.Id))
		return response
	}
	// Changing the list.
	response = change(map[string]any{"model_limits": []string{"gpt-image-1"}})
	require.True(t, response.Success, response.Message)
	require.Equal(t, "gpt-image-1", stored(view.Id).ModelLimits)
	// A list that is null was not said: the rest of the change goes through
	// and the list stays.
	response = change(map[string]any{"name": "Images only", "model_limits": nil})
	require.True(t, response.Success, response.Message)
	key = stored(view.Id)
	require.Equal(t, []any{"Images only", "gpt-image-1", true}, []any{key.Name, key.ModelLimits, key.ModelLimitsEnabled})
	// A model that left the catalogue stays on the key through a change that
	// only adds to the list…
	require.NoError(t, db.Model(&model.Ability{}).Where("model = ?", "gpt-image-1").Update("enabled", false).Error)
	require.JSONEq(t, `["eleven_multilingual_v2","new-video-a","new-video-b"]`, offered())
	response = change(map[string]any{"model_limits": []string{"gpt-image-1", "new-video-b"}})
	require.True(t, response.Success, response.Message)
	require.Equal(t, "gpt-image-1,new-video-b", stored(view.Id).ModelLimits)
	// …but a new key cannot be given it.
	response, _ = callOrg(t, CreateOrgKey, http.MethodPost, map[string]any{"name": "K", "model_limits": []string{"gpt-image-1"}}, ownerID)
	require.Equal(t, msgOrgKeyModelsUnavailable, response.Message)
	// To a template, which takes the list's place, and to no limit at all.
	response = change(map[string]any{"policy_template": "creative"})
	require.True(t, response.Success, response.Message)
	key = stored(view.Id)
	require.Equal(t, "creative", key.PolicyTemplate)
	require.Contains(t, key.GetModelLimits(), "new-video-a")
	require.NotContains(t, key.GetModelLimits(), "gpt-image-1")
	response = change(map[string]any{"model_limits": []string{}})
	require.True(t, response.Success, response.Message)
	key = stored(view.Id)
	require.Equal(t, []any{"", "", false}, []any{key.PolicyTemplate, key.ModelLimits, key.ModelLimitsEnabled})

	// The list is asked for a member: nobody, or somebody else's member, is not one.
	for _, query := range []string{"", "?holder_id=", "?holder_id=abc", "?holder_id=0", "?holder_id=-3", "?holder_id=424242"} {
		response, recorder := callAt(t, ListOrgKeyModels, http.MethodGet, "/api/org/key-models"+query, nil, ownerID)
		require.False(t, response.Success, query)
		require.Equal(t, http.StatusOK, recorder.Code, query)
		require.Equal(t, msgOrgMemberNotFound, response.Message, query)
	}
}

// The refusal for a hand-picked model has to say which model: the message is
// built from the names the service returns, under the word the three locale
// files use for them.
func TestOrgKeys_TheRefusalNamesTheModelsThatCannotBeTaken(t *testing.T) {
	refusal := &orgservice.KeyModelsUnavailableError{Models: []string{"gpt-5-typo", "retired-model"}}
	require.Equal(t, map[string]any{"Models": "gpt-5-typo, retired-model"}, orgRefusalDetails(refusal))
	require.Equal(t, map[string]any{"Models": "gpt-5-typo, retired-model"}, orgRefusalDetails(fmt.Errorf("creating a key: %w", refusal)))
	for _, other := range []error{orgservice.ErrForbidden, orgservice.ErrKeyModelsWithTemplate, errors.New("anything else")} {
		require.Nil(t, orgRefusalDetails(other))
	}
	for _, locale := range []string{"en", "zh-CN", "zh-TW"} {
		raw, err := os.ReadFile("../i18n/locales/" + locale + ".yaml")
		require.NoError(t, err)
		named := false
		for _, line := range strings.Split(string(raw), "\n") {
			if text, isKey := strings.CutPrefix(line, msgOrgKeyModelsUnavailable+": "); isKey {
				named = strings.Contains(text, "{{.Models}}")
			}
		}
		require.True(t, named, "the %s message does not name the models", locale)
	}
}

// Acceptance: 一键 rotation … 旧值立即失效; 一键吊销（冻结）与解冻; and a deleted key.
// Asked of model.ValidateUserToken, the function the gateway asks on every
// request a key makes.
func TestOrgKeys_TheGatewayStopsHonouringAKeyAtOnce(t *testing.T) {
	const ownerID = 7301
	db := orgWithCatalogue(t, ownerID)
	created, _ := callOrg(t, CreateOrgServiceAccount, http.MethodPost, map[string]any{"name": "CI"}, ownerID)
	require.True(t, created.Success, created.Message)
	var bot orgservice.MemberView
	require.NoError(t, common.Unmarshal(created.Data, &bot))

	type grant struct {
		Id    int    `json:"id"`
		Value string `json:"value"`
	}
	call := func(handler gin.HandlerFunc, method string, body any, params ...gin.Param) grant {
		response, _ := callOrg(t, handler, method, body, ownerID, params...)
		require.True(t, response.Success, response.Message)
		var answer grant
		if string(response.Data) != "null" {
			require.NoError(t, common.Unmarshal(response.Data, &answer))
		}
		return answer
	}
	// honoured asks the gateway's question; it returns the key the value
	// belongs to, or zero when the gateway turns it away.
	honoured := func(value string) int {
		key, err := model.ValidateUserToken(value)
		if err != nil {
			require.ErrorIs(t, err, model.ErrTokenInvalid)
			return 0
		}
		return key.Id
	}

	key := call(CreateOrgKey, http.MethodPost, map[string]any{"name": "CI key", "holder_id": bot.Id, "unlimited_quota": true})
	control := call(CreateOrgKey, http.MethodPost, map[string]any{"name": "untouched", "holder_id": bot.Id, "unlimited_quota": true})
	require.Equal(t, key.Id, honoured(key.Value))

	rotated := call(RotateOrgKey, http.MethodPost, nil, idParam(key.Id))
	require.Zero(t, honoured(key.Value), "the old value is dead the moment rotation answers")
	require.Equal(t, key.Id, honoured(rotated.Value), "and the new one is the same key")

	call(FreezeOrgKey, http.MethodPost, nil, idParam(key.Id))
	require.Zero(t, honoured(rotated.Value), "a frozen key is turned away")
	call(UnfreezeOrgKey, http.MethodPost, nil, idParam(key.Id))
	require.Equal(t, key.Id, honoured(rotated.Value))

	call(DeleteOrgKey, http.MethodDelete, nil, idParam(key.Id))
	require.Zero(t, honoured(rotated.Value), "a deleted key is gone")
	require.Zero(t, honoured(key.Value))

	require.Equal(t, control.Id, honoured(control.Value), "none of it touched the key beside it")
	var kept model.Token
	require.NoError(t, db.Unscoped().First(&kept, key.Id).Error)
	require.True(t, kept.DeletedAt.Valid, "the row stays, for the usage it accounts for")
}
