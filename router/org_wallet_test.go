package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeProvider stands where a model provider would: it answers every chat
// request with the same short reply and the same usage, accepts every video
// task, and counts the calls, so a test can tell a request that was served
// from one that never left the gateway.
type fakeProvider struct {
	*httptest.Server
	calls atomic.Int64
}

// newFakeProvider starts the provider and stops it with the test.
func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	provider := &fakeProvider{}
	provider.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provider.calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "video") {
			_, _ = w.Write([]byte(`{"task_id":"provider-task-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1700000000,"model":"gpt-4o-mini",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`))
	}))
	t.Cleanup(provider.Close)
	return provider
}

// newRelayEngine is newOrgEngine with the relay routes on top, the things the
// relay expects the gateway's start-up to have done, and one channel that
// serves gpt-4o-mini from the fake provider to the default group.
func newRelayEngine(t *testing.T, db *gorm.DB, provider *fakeProvider) *gin.Engine {
	t.Helper()
	engine := newOrgEngine(t, db)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.UserSubscription{}, &model.Task{}))
	model.InitColumnNames()
	service.InitHttpClient()
	service.InitTokenEncoders()
	ratio_setting.InitRatioSettings()
	setGlobal(t, &common.MemoryCacheEnabled, false)
	setGlobal(t, &common.BatchUpdateEnabled, false)
	setGlobal(t, &common.LogConsumeEnabled, true)
	setGlobal(t, &constant.StreamingTimeout, 30)
	SetRelayRouter(engine)
	SetVideoRouter(engine)

	baseURL := provider.URL
	channel := &model.Channel{
		Type: constant.ChannelTypeOpenAI, Name: "fake provider", Key: "sk-fake-provider-key",
		Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-4o-mini", BaseURL: &baseURL,
	}
	require.NoError(t, channel.Insert())
	midjourney := &model.Channel{
		Type: constant.ChannelTypeMidjourney, Name: "fake midjourney", Key: "mj-fake-provider-key",
		Status: common.ChannelStatusEnabled, Group: "default", Models: "mj_imagine", BaseURL: &baseURL,
	}
	require.NoError(t, midjourney.Insert())
	video := &model.Channel{
		Type: constant.ChannelTypeMiniMax, Name: "fake video provider", Key: "video-fake-provider-key",
		Status: common.ChannelStatusEnabled, Group: "default", Models: "MiniMax-H3", BaseURL: &baseURL,
	}
	require.NoError(t, video.Insert())
	// A served request leaves work behind — metrics, the auto top-up check,
	// the low-balance reminder — that reads the globals set above. Registered
	// last, this runs first when the test ends: it lets that work finish
	// before any of them is put back.
	t.Cleanup(func() {
		deadline := time.Now().Add(5 * time.Second)
		for gopool.WorkerCount() > 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
	})
	return engine
}

// chat sends one chat request with a key the way an API client does — the key
// as its holder has it, "sk-" and all — and returns the answer.
func chat(t *testing.T, engine *gin.Engine, key string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"max_tokens":50}`
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(body)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+key)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

// TestOrgWallet_ThroughTheRealGateway walks Enterprise Org P7 (meta-repo
// docs/enterprise-org-prd.md §4, §7.4) end to end: a company signs up, hands a
// key to a member, and the member's request goes through the gateway's real
// key authentication, channel selection, relay and settlement to a provider
// that is the only thing faked. The company's balance pays, the usage is the
// member's and is stamped; an empty wallet stops every key of the company; and
// a personal account next door is billed as it always was.
func TestOrgWallet_ThroughTheRealGateway(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		provider := newFakeProvider(t)
		engine := newRelayEngine(t, db, provider)
		id := strconv.Itoa
		quotaOf := func(userID int) int {
			var user model.User
			require.NoError(t, db.Select("quota").First(&user, userID).Error)
			return user.Quota
		}
		setQuota := func(userID int, quota int) {
			require.NoError(t, db.Model(&model.User{}).Where("id = ?", userID).Update("quota", quota).Error)
		}
		// valueOf reads a key's value from the database, which is where a test
		// gets what no endpoint tells: a person's organization key is installed
		// by one-click setup and never shown.
		valueOf := func(keyID int) string {
			var key model.Token
			require.NoError(t, db.First(&key, keyID).Error)
			return "sk-" + key.Key
		}
		lastLog := func() model.Log {
			var log model.Log
			require.NoError(t, db.Where("type = ?", model.LogTypeConsume).Order("id desc").First(&log).Error)
			return log
		}
		// settled waits for the settlement, which runs after the answer is out.
		settled := func(what string, done func() bool) {
			t.Helper()
			deadline := time.Now().Add(5 * time.Second)
			for !done() {
				require.True(t, time.Now().Before(deadline), "timed out waiting for %s", what)
				time.Sleep(10 * time.Millisecond)
			}
		}

		owner := signUp(t, engine, map[string]any{"username": "founder", "password": "password123", "org_name": "Acme"}, "")
		var departments []struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		}
		owner.ok(t, http.MethodGet, "/api/org/departments", nil, &departments)
		departmentID := map[string]int{}
		for _, d := range departments {
			departmentID[d.Name] = d.Id
		}
		var roles []struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		}
		owner.ok(t, http.MethodGet, "/api/org/roles", nil, &roles)
		roleID := map[string]int{}
		for _, r := range roles {
			roleID[r.Name] = r.Id
		}
		var invite struct {
			Code string `json:"code"`
		}
		owner.ok(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[orgmodel.RoleStaff], "department_id": departmentID["Sales"]}, &invite)
		seller := signUp(t, engine, map[string]any{"username": "seller", "password": "password123", "org_invite": invite.Code}, "")
		var sellersKey struct {
			Id int `json:"id"`
		}
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Sales tools", "holder_id": seller.userID, "unlimited_quota": true}, &sellersKey)
		personal := signUp(t, engine, map[string]any{"username": "solo", "password": "password123"}, "")
		var personalKey struct {
			Id int `json:"id"`
		}
		personal.ok(t, http.MethodPost, "/api/token/", map[string]any{"name": "mine", "expired_time": -1, "unlimited_quota": true}, &personalKey)

		setQuota(owner.userID, 1000000)
		setQuota(seller.userID, 0)
		setQuota(personal.userID, 1000000)

		// --- 组织 key 的消耗从公司钱包扣除，staff 个人账户余额不变、不参与余额判定 ---
		answer := chat(t, engine, valueOf(sellersKey.Id))
		require.Equal(t, http.StatusOK, answer.Code, answer.Body.String())
		require.EqualValues(t, 1, provider.calls.Load())
		settled("the member's usage to be logged", func() bool {
			var n int64
			db.Model(&model.Log{}).Where("type = ? AND user_id = ?", model.LogTypeConsume, seller.userID).Count(&n)
			return n == 1
		})
		log := lastLog()
		require.Positive(t, log.Quota, "the request cost something")
		settled("the company wallet to be charged", func() bool { return quotaOf(owner.userID) == 1000000-log.Quota })
		assert.Equal(t, 0, quotaOf(seller.userID), "the member's own balance is untouched, and having none did not stop them")
		assert.Equal(t, 1000000, quotaOf(personal.userID))

		// --- 消费日志记录使用者的 user_id 与 token_id，并盖章 org_id 与当时的 department_id ---
		var org orgmodel.Organization
		require.NoError(t, db.First(&org).Error)
		assert.Equal(t, seller.userID, log.UserId)
		assert.Equal(t, sellersKey.Id, log.TokenId)
		assert.Equal(t, "gpt-4o-mini", log.ModelName)
		assert.Equal(t, 100, log.PromptTokens)
		assert.Equal(t, 50, log.CompletionTokens)
		assert.Equal(t, org.Id, log.OrgId)
		assert.Equal(t, departmentID["Sales"], log.DepartmentId)

		// The member is moved; what they spend next is stamped with the new
		// department, and the line already written keeps the old one.
		owner.ok(t, http.MethodPut, "/api/org/members/"+id(seller.userID), map[string]any{"department_id": departmentID["Product"]}, nil)
		require.Equal(t, http.StatusOK, chat(t, engine, valueOf(sellersKey.Id)).Code)
		settled("the second request to be logged", func() bool { return lastLog().Id != log.Id })
		assert.Equal(t, departmentID["Product"], lastLog().DepartmentId)
		var first model.Log
		require.NoError(t, db.First(&first, log.Id).Error)
		assert.Equal(t, departmentID["Sales"], first.DepartmentId)
		settled("the second charge", func() bool { return quotaOf(owner.userID) == 1000000-2*log.Quota })

		// --- 无组织的个人用户，行为与改动前完全一致 ---
		require.Equal(t, http.StatusOK, chat(t, engine, valueOf(personalKey.Id)).Code)
		settled("the personal request to be logged", func() bool { return lastLog().UserId == personal.userID })
		personalLog := lastLog()
		assert.Equal(t, log.Quota, personalLog.Quota, "the same request costs the same")
		assert.Zero(t, personalLog.OrgId)
		assert.Zero(t, personalLog.DepartmentId)
		settled("the personal balance to be charged", func() bool { return quotaOf(personal.userID) == 1000000-personalLog.Quota })
		assert.Equal(t, 1000000-2*log.Quota, quotaOf(owner.userID), "and it is nobody else's business")

		// --- 公司钱包余额不足时组织内所有 key 调用被拒 ---
		var ownersKey struct {
			Id int `json:"id"`
		}
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Owner's own", "unlimited_quota": true}, &ownersKey)
		setQuota(owner.userID, 0)
		setQuota(seller.userID, 500000) // the member is given plenty of their own: it must not help
		served := provider.calls.Load()
		for name, keyID := range map[string]int{"the member's key": sellersKey.Id, "the owner's own key": ownersKey.Id} {
			refused := chat(t, engine, valueOf(keyID))
			require.Equal(t, http.StatusForbidden, refused.Code, "%s: %s", name, refused.Body.String())
			assert.Contains(t, refused.Body.String(), "insufficient_user_quota", name)
			assert.Contains(t, refused.Body.String(), "org.wallet_insufficient", name)
			assert.NotContains(t, refused.Body.String(), "额度", "%s: the company's balance is not quoted", name)
		}
		assert.Equal(t, served, provider.calls.Load(), "a refused request never reaches the provider")
		assert.Equal(t, 500000, quotaOf(seller.userID), "and is not paid from the member's pocket instead")
		// Next door nothing changed.
		require.Equal(t, http.StatusOK, chat(t, engine, valueOf(personalKey.Id)).Code)

		// The owner adds credit, and the member's key works again at once.
		setQuota(owner.userID, 1000000)
		require.Equal(t, http.StatusOK, chat(t, engine, valueOf(sellersKey.Id)).Code)

		// --- a wallet that cannot be charged at all ---
		// The platform disables the owner's account: the company is suspended,
		// and its members' keys stop before anything is spent.
		require.NoError(t, db.Model(&model.User{}).Where("id = ?", owner.userID).Update("status", common.UserStatusDisabled).Error)
		served = provider.calls.Load()
		suspended := chat(t, engine, valueOf(sellersKey.Id))
		require.Equal(t, http.StatusForbidden, suspended.Code, suspended.Body.String())
		assert.Contains(t, suspended.Body.String(), "org.wallet_unavailable")
		assert.Equal(t, served, provider.calls.Load())
		require.Equal(t, http.StatusOK, chat(t, engine, valueOf(personalKey.Id)).Code, "a personal key is asked none of this")
		require.NoError(t, db.Model(&model.User{}).Where("id = ?", owner.userID).Update("status", common.UserStatusEnabled).Error)

		// --- a service account's key: it has no balance of its own and never will ---
		var bot struct {
			Id int `json:"id"`
		}
		owner.ok(t, http.MethodPost, "/api/org/service-accounts", map[string]any{"name": "CI", "department_id": departmentID["Engineering"]}, &bot)
		var botKey struct {
			Id    int    `json:"id"`
			Value string `json:"value"`
		}
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "CI key", "holder_id": bot.Id, "unlimited_quota": true}, &botKey)
		require.NotEmpty(t, botKey.Value, "a service account's key is shown once, to whoever made it")
		before := quotaOf(owner.userID)
		answer = chat(t, engine, botKey.Value)
		require.Equal(t, http.StatusOK, answer.Code, answer.Body.String())
		settled("the service account's usage to be logged", func() bool { return lastLog().UserId == bot.Id })
		assert.Equal(t, departmentID["Engineering"], lastLog().DepartmentId)
		settled("the company wallet to pay for it", func() bool { return quotaOf(owner.userID) == before-lastLog().Quota })
		assert.Equal(t, 0, quotaOf(bot.Id))

		// --- an asynchronous task: paid up front, refunded much later -------------
		// A video is paid in full when it is submitted and settled when the
		// provider is done — minutes later, by the poller, with no request to
		// ask who paid. The task has to remember.
		submitVideo := func(keyValue string) *httptest.ResponseRecorder {
			request := httptest.NewRequest(http.MethodPost, "/v1/video/generations",
				bytes.NewReader([]byte(`{"model":"MiniMax-H3","prompt":"a cat walking","seconds":"6"}`)))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+keyValue)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			return recorder
		}
		taskOf := func(userID int) *model.Task {
			var task model.Task
			require.NoError(t, db.Where("user_id = ?", userID).Order("id desc").First(&task).Error)
			return &task
		}
		setQuota(owner.userID, 5000000)
		setQuota(seller.userID, 0)
		setQuota(personal.userID, 5000000)
		submitted := submitVideo(valueOf(sellersKey.Id))
		require.Equal(t, http.StatusOK, submitted.Code, submitted.Body.String())
		task := taskOf(seller.userID)
		require.Positive(t, task.Quota, "a video costs something")
		assert.Equal(t, org.Id, task.PrivateData.OrgId, "the task remembers the organization")
		assert.Equal(t, departmentID["Product"], task.PrivateData.OrgDepartmentId, "and the department of that moment")
		assert.Equal(t, owner.userID, task.PrivateData.OrgWalletUserId, "and whose balance paid")
		assert.Equal(t, 5000000-task.Quota, quotaOf(owner.userID), "the company wallet paid for it in full, up front")
		assert.Equal(t, 0, quotaOf(seller.userID))
		videoLog := lastLog()
		assert.Equal(t, []int{seller.userID, org.Id, departmentID["Product"], task.Quota},
			[]int{videoLog.UserId, videoLog.OrgId, videoLog.DepartmentId, videoLog.Quota})

		// The provider fails the task. The refund goes back where the money
		// came from — not into the member's own account, who never paid.
		service.RefundTaskQuota(context.Background(), task, "the provider failed")
		assert.Equal(t, 5000000, quotaOf(owner.userID), "the company wallet is made whole")
		assert.Equal(t, 0, quotaOf(seller.userID), "and the member gains nothing")
		var refund model.Log
		require.NoError(t, db.Where("type = ?", model.LogTypeRefund).Order("id desc").First(&refund).Error)
		assert.Equal(t, []int{seller.userID, org.Id, departmentID["Product"], task.Quota},
			[]int{refund.UserId, refund.OrgId, refund.DepartmentId, refund.Quota})

		// A personal account's video is paid and refunded on its own balance.
		require.Equal(t, http.StatusOK, submitVideo(valueOf(personalKey.Id)).Code)
		personalTask := taskOf(personal.userID)
		assert.Zero(t, personalTask.PrivateData.OrgWalletUserId)
		assert.Equal(t, 5000000-personalTask.Quota, quotaOf(personal.userID))
		service.RefundTaskQuota(context.Background(), personalTask, "the provider failed")
		assert.Equal(t, 5000000, quotaOf(personal.userID))
		assert.Equal(t, 5000000, quotaOf(owner.userID))
		setQuota(owner.userID, 1000000)
		setQuota(personal.userID, 1000000)

		// --- the two entrances the company wallet cannot follow -----------------
		// The playground sends requests with no key, on the caller's own balance.
		playground := func(b browser) *httptest.ResponseRecorder {
			return b.raw(t, http.MethodPost, "/pg/chat/completions",
				map[string]any{"model": "gpt-4o-mini", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
		}
		reached := provider.calls.Load()
		for name, member := range map[string]browser{"the owner": owner, "a member": seller} {
			closed := playground(member)
			require.Equal(t, http.StatusForbidden, closed.Code, "%s: %s", name, closed.Body.String())
			assert.Contains(t, closed.Body.String(), "org.playground_closed", name)
		}
		assert.Equal(t, reached, provider.calls.Load(), "nothing was sent on a member's behalf")
		open := playground(personal)
		require.Equal(t, http.StatusOK, open.Code, "a personal account uses the playground as before: %s", open.Body.String())
		assert.Equal(t, reached+1, provider.calls.Load())

		// A Midjourney task cannot say who paid for it, so its refund would go
		// to the wrong account: organization keys are turned away at the door.
		imagine := func(keyValue string) *httptest.ResponseRecorder {
			request := httptest.NewRequest(http.MethodPost, "/mj/submit/imagine", bytes.NewReader([]byte(`{"prompt":"a cat"}`)))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+keyValue)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			return recorder
		}
		reached = provider.calls.Load()
		before = quotaOf(owner.userID)
		turnedAway := imagine(valueOf(sellersKey.Id))
		require.Equal(t, http.StatusForbidden, turnedAway.Code, turnedAway.Body.String())
		assert.Contains(t, turnedAway.Body.String(), "organization_key_not_supported")
		assert.Equal(t, reached, provider.calls.Load())
		assert.Equal(t, before, quotaOf(owner.userID))
		letThrough := imagine(valueOf(personalKey.Id))
		assert.NotContains(t, letThrough.Body.String(), "organization_key_not_supported")
		assert.Equal(t, reached+1, provider.calls.Load(), "a personal key's request goes on to the provider")

		// --- the organization tables are optional to the gateway ----------------
		// Should they ever be missing (internal/org/model/migrate.go reports a
		// failed migration and lets the gateway start), an organization key is
		// refused — it cannot be billed — and a personal key is served as ever.
		require.NoError(t, db.Migrator().DropTable(&orgmodel.Organization{}))
		reached = provider.calls.Load()
		broken := chat(t, engine, valueOf(sellersKey.Id))
		require.Equal(t, http.StatusInternalServerError, broken.Code, broken.Body.String())
		assert.Equal(t, reached, provider.calls.Load())
		require.Equal(t, http.StatusOK, chat(t, engine, valueOf(personalKey.Id)).Code)
	})
}
