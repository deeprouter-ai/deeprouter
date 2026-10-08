package service

// Tests of the background task behind Enterprise Org's warnings and alerts
// (meta-repo docs/enterprise-org-prd.md §4, card P8): what one pass does with
// the keys it finds, who is told and in which words, and that alerts raised
// close together leave as one notification. What the scans decide is tested
// where they live, in internal/org/service; these tests are about what needs
// the platform — the loop's bookkeeping, the gateway's request counter, the
// wording and the sending.
//
// They use the package's in-memory database and the cast of org_wallet_test.go,
// and are named TestOrg… so both CI workflows select them.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	tenantquota "github.com/QuantumNous/new-api/internal/quota"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedAlertWatch turns the wallet cast into a real organization (see
// seedWalletWatchers), sets the platform's notification limit to its defaults
// and returns the admin's id and a watch that has not looked yet.
func seedAlertWatch(t *testing.T) (adminID int, watch *orgAlertWatch) {
	t.Helper()
	adminID = seedWalletWatchers(t)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM org_alerts")
		model.DB.Exec("DELETE FROM org_audit_logs")
	})
	limit, window := constant.NotifyLimitCount, constant.NotificationLimitDurationMinute
	constant.NotifyLimitCount, constant.NotificationLimitDurationMinute = 2, 10
	t.Cleanup(func() { constant.NotifyLimitCount, constant.NotificationLimitDurationMinute = limit, window })
	return adminID, &orgAlertWatch{lastDigest: map[int]time.Time{}}
}

// listLinkHTML and listLinkPlain are the line a notification ends with — the
// link to the organization's alert list — as mail and as plain text in
// Chinese, the two ways the members of these tests read.
func listLinkHTML() string {
	return "<a href='" + orgAlertListLink() + "'>See all alerts</a>"
}

func listLinkPlain() string { return "查看全部告警：" + orgAlertListLink() }

// captureOrgAlertNotices replaces the sender for the length of a test and
// returns what was handed to it.
func captureOrgAlertNotices(t *testing.T) func() []sentNotice {
	t.Helper()
	var mu sync.Mutex
	var sent []sentNotice
	real := sendOrgAlertNotice
	sendOrgAlertNotice = func(userID int, email string, _ dto.UserSetting, notice dto.Notify) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, sentNotice{userID: userID, email: email, notice: notice})
		return nil
	}
	t.Cleanup(func() { sendOrgAlertNotice = real })
	return func() []sentNotice {
		mu.Lock()
		defer mu.Unlock()
		return append([]sentNotice{}, sent...)
	}
}

// noticesByUser sorts what was sent by who it went to.
func noticesByUser(notices []sentNotice) map[int][]sentNotice {
	byUser := map[int][]sentNotice{}
	for _, notice := range notices {
		byUser[notice.userID] = append(byUser[notice.userID], notice)
	}
	return byUser
}

// useKey writes what a key has used and has left and when it was last used,
// the way a settled request does.
func useKey(t *testing.T, keyID int, used int, remain int, at time.Time) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", keyID).
		Updates(map[string]any{"used_quota": used, "remain_quota": remain, "accessed_time": at.Unix()}).Error)
}

// seedSecondOrgKey gives the member another organization key, 5000 in all.
func seedSecondOrgKey(t *testing.T, keyID int, name string) {
	t.Helper()
	seedToken(t, keyID, walletMemberID, fmt.Sprintf("org-key-%d", keyID), 5000)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", keyID).
		Updates(map[string]any{"org_id": walletOrgID, "name": name}).Error)
}

// unsentAlertCount counts the alerts still waiting to be sent.
func unsentAlertCount(t *testing.T) int {
	t.Helper()
	var n int64
	require.NoError(t, model.DB.Model(&orgmodel.OrgAlert{}).Where("notified_time = ?", 0).Count(&n).Error)
	return int(n)
}

// --- a warning, from the key's row to the notification -------------------------

// Acceptance: key 的额度…使用率达到阈值…时，admin 与 key 持有人收到通知. One pass
// finds the key, and the owner, the admin and the key's holder are each told
// once — in their own language, for their own channel — and nobody else is.
func TestOrgAlerts_AKeyRunningLowIsReportedOnceToTheAdminsAndItsHolder(t *testing.T) {
	seedWalletCast(t, 100000, 0, 100000)
	adminID, watch := seedAlertWatch(t)
	sent := captureOrgAlertNotices(t)
	now := time.Now()
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", walletMemberKeyID).Update("name", "Design <tools>").Error)
	useKey(t, walletMemberKeyID, 4000, 1000, now)
	// A personal key at the very same point is nobody's to be warned about.
	useKey(t, walletBystanderKey, 4000, 1000, now)

	watch.pass(now)

	byUser := noticesByUser(sent())
	require.Len(t, sent(), 3, "the owner, the admin and the key's holder")
	require.Contains(t, byUser, walletOwnerID)
	require.Contains(t, byUser, adminID)
	require.Contains(t, byUser, walletMemberID)
	assert.NotContains(t, byUser, walletBystanderID)

	used, limit := logger.FormatQuota(4000), logger.FormatQuota(5000)
	owner := byUser[walletOwnerID][0]
	assert.Equal(t, "owner@acme.test", owner.email)
	assert.Equal(t, notifyTypeOrgAlert, owner.notice.Type, "a type of its own, apart from the low-balance reminder")
	assert.NotEqual(t, dto.NotifyTypeQuotaExceed, owner.notice.Type)
	assert.Equal(t, "1 new alert(s) about your organization's keys", owner.notice.Title)
	warning := "Key “Design &lt;tools&gt;” (member) has used 80% of its quota: " + used + " of " + limit + "."
	assert.Equal(t, warning+"<br/>"+listLinkHTML(), owner.notice.Content,
		"mail is read as HTML, so a name is escaped; and the owner can open the list")
	assert.Empty(t, owner.notice.Values, "the text is complete: nothing is left to fill in")

	admin := byUser[adminID][0]
	assert.Equal(t, "企业密钥有 1 条新提醒", admin.notice.Title, "in the language the admin saved")
	assert.Equal(t, "密钥「Design <tools>」（member）的额度已用 80%：共 "+limit+"，已用 "+used+"。\n"+listLinkPlain(), admin.notice.Content,
		"Bark shows plain text, so the name goes out as it is")

	holder := byUser[walletMemberID][0]
	assert.Equal(t, "member@acme.test", holder.email)
	assert.Equal(t, owner.notice.Title, holder.notice.Title)
	assert.Equal(t, warning+"<br/>"+listLinkHTML(), holder.notice.Content,
		"the holder is told the same thing, with the same link: the list shows them the warnings on their own keys")

	// The alert is in the organization's list, marked as sent.
	var alerts []orgmodel.OrgAlert
	require.NoError(t, model.DB.Find(&alerts).Error)
	require.Len(t, alerts, 1)
	assert.Equal(t, []any{walletOrgID, orgmodel.AlertRuleQuota, 80, walletMemberKeyID, walletMemberID, walletDepartmentID},
		[]any{alerts[0].OrgId, alerts[0].Rule, alerts[0].Level, alerts[0].TokenId, alerts[0].UserId, alerts[0].DepartmentId})
	assert.Equal(t, now.Unix(), alerts[0].NotifiedTime)

	// The passes that follow have nothing to add.
	for minute := 1; minute <= 12; minute++ {
		watch.pass(now.Add(time.Duration(minute) * time.Minute))
	}
	assert.Len(t, sent(), 3)
	// The key is exactly what it was: a warning never switches anything off.
	var key model.Token
	require.NoError(t, model.DB.First(&key, walletMemberKeyID).Error)
	assert.Equal(t, []int{common.TokenStatusEnabled, 4000, 1000}, []int{key.Status, key.UsedQuota, key.RemainQuota})
}

// PRD D45: a key the gateway refuses because what it has left does not cover
// the request is warned about then and there — the request path raises it,
// since no scan can see a request that never happened — and the next pass
// sends it like any warning, saying what was left and what was needed. A
// personal key refused the same way, and an organization key refused because
// the company wallet is empty, raise nothing.
func TestOrgAlerts_AKeyRefusedForLackOfQuotaIsWarnedAboutThenAndThere(t *testing.T) {
	seedWalletCast(t, 100000, 0, 100000)
	adminID, watch := seedAlertWatch(t)
	sent := captureOrgAlertNotices(t)
	now := time.Now()
	useKey(t, walletMemberKeyID, 4600, 400, now)
	useKey(t, walletBystanderKey, 4600, 400, now)

	apiErr := PreConsumeBilling(walletCtx(), 1000, memberRequest())
	require.NotNil(t, apiErr, "the key has 400 left and the request needs 1000")
	assert.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, apiErr.GetErrorCode())
	eventually(t, "the refusal to be on the alert list", func() bool { return unsentAlertCount(t) == 1 })
	var alerts []orgmodel.OrgAlert
	require.NoError(t, model.DB.Find(&alerts).Error)
	assert.Equal(t, []any{walletOrgID, orgmodel.AlertRuleQuota, 100, walletMemberKeyID, walletMemberID, walletDepartmentID},
		[]any{alerts[0].OrgId, alerts[0].Rule, alerts[0].Level, alerts[0].TokenId, alerts[0].UserId, alerts[0].DepartmentId})
	var detail orgmodel.AlertDetail
	require.NoError(t, common.UnmarshalJsonStr(alerts[0].Detail, &detail))
	assert.Equal(t, orgmodel.AlertDetail{Key: "test_token", Used: 4600, Limit: 5000, Needed: 1000}, detail)

	// Refused again, and the personal key refused too: nothing more to say.
	apiErr = PreConsumeBilling(walletCtx(), 1000, memberRequest())
	require.NotNil(t, apiErr)
	apiErr = PreConsumeBilling(walletCtx(), 1000, bystanderRequest())
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, apiErr.GetErrorCode())
	// An empty company wallet refuses the key for another reason.
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", walletOwnerID).Update("quota", 0).Error)
	apiErr = PreConsumeBilling(walletCtx(), 100, memberRequest())
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, unsentAlertCount(t), "one refusal, one warning")

	watch.pass(now)
	byUser := noticesByUser(sent())
	require.Len(t, sent(), 3, "the owner, the admin and the key's holder")
	left, needed := logger.FormatQuota(400), logger.FormatQuota(1000)
	warning := "Key “test_token” (member) has " + left + " of its quota left — less than a request has to have in hand (" + needed + " this time) — so its requests are refused. An administrator of your organization can give it more."
	assert.Equal(t, warning+"<br/>"+listLinkHTML(), byUser[walletOwnerID][0].notice.Content)
	assert.Equal(t, "密钥「test_token」（member）的额度只剩 "+left+"，不够完成一次请求（这次需要预留 "+needed+"），请求已被拒绝。企业管理员可以给它增加额度。\n"+listLinkPlain(),
		byUser[adminID][0].notice.Content)
	assert.Equal(t, warning+"<br/>"+listLinkHTML(), byUser[walletMemberID][0].notice.Content)
	// The scan that follows has nothing to add: the key heard its last warning.
	useKey(t, walletMemberKeyID, 5000, 0, now.Add(time.Minute))
	watch.pass(now.Add(time.Minute))
	assert.Len(t, sent(), 3)
}

// The platform lets a user have two notifications of a type in ten minutes
// and drops the rest. So alerts raised soon after a notification wait, and
// leave together once the organization may be written to again.
func TestOrgAlerts_AlertsRaisedCloseTogetherLeaveAsOneNotification(t *testing.T) {
	seedWalletCast(t, 100000, 0, 100000)
	adminID, watch := seedAlertWatch(t)
	sent := captureOrgAlertNotices(t)
	now := time.Now()
	seedSecondOrgKey(t, 203, "Second key")
	require.Equal(t, 6*time.Minute, orgAlertDigestGap())

	useKey(t, walletMemberKeyID, 4000, 1000, now)
	watch.pass(now)
	require.Len(t, sent(), 3, "the first alert goes out at once")

	// A minute later the key runs out, and two minutes later another one of
	// the member's keys is running low.
	useKey(t, walletMemberKeyID, 5000, 0, now.Add(time.Minute))
	watch.pass(now.Add(time.Minute))
	useKey(t, 203, 4500, 500, now.Add(2*time.Minute))
	watch.pass(now.Add(2 * time.Minute))
	for minute := 3; minute <= 5; minute++ {
		watch.pass(now.Add(time.Duration(minute) * time.Minute))
	}
	assert.Len(t, sent(), 3, "nothing more is sent inside the gap")
	assert.Equal(t, 2, unsentAlertCount(t), "both alerts are in the list, waiting")

	watch.pass(now.Add(6 * time.Minute))
	notices := sent()
	require.Len(t, notices, 6, "one more notification each")
	assert.Zero(t, unsentAlertCount(t))
	limit := logger.FormatQuota(5000)
	for _, userID := range []int{walletOwnerID, adminID, walletMemberID} {
		second := noticesByUser(notices)[userID][1]
		lines := strings.Split(second.notice.Content, "<br/>")
		if userID == adminID {
			lines = strings.Split(second.notice.Content, "\n")
		}
		require.Len(t, lines, 3, "user %d: two alerts in one notification, and the link to the list", userID)
		if userID != adminID {
			assert.Equal(t, "2 new alert(s) about your organization's keys", second.notice.Title)
			assert.Equal(t, "Key “test_token” (member) has used up its quota of "+limit+" and no longer works. An administrator of your organization can give it more.", lines[0])
			assert.Equal(t, "Key “Second key” (member) has used 90% of its quota: "+logger.FormatQuota(4500)+" of "+limit+".", lines[1])
		}
	}
}

// A key is looked at when it is used. The first pass after a start looks at
// every key; after that, only at the ones used since the pass before.
func TestOrgAlerts_AfterTheFirstPassOnlyKeysUsedSinceAreLookedAt(t *testing.T) {
	seedWalletCast(t, 100000, 0, 100000)
	_, watch := seedAlertWatch(t)
	sent := captureOrgAlertNotices(t)
	now := time.Now()
	seedSecondOrgKey(t, 203, "Second key")

	// Used long ago, and running low: the first pass still finds it.
	useKey(t, walletMemberKeyID, 4000, 1000, now.Add(-90*24*time.Hour))
	watch.pass(now)
	require.Len(t, sent(), 3)

	// A row changed behind the gateway's back, with no request to show for it.
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 203).
		Updates(map[string]any{"used_quota": 4000, "remain_quota": 1000}).Error)
	watch.pass(now.Add(10 * time.Minute))
	assert.Len(t, sent(), 3, "nothing used that key, so nothing looked at it")

	// Its next request is what gets it looked at.
	useKey(t, 203, 4010, 990, now.Add(19*time.Minute+50*time.Second))
	watch.pass(now.Add(20 * time.Minute))
	assert.Len(t, sent(), 6)
}

// Acceptance: …或月限额使用率达到阈值. The monthly limit counts requests, and the
// count is the gateway's own — the one its monthly limit refuses by.
func TestOrgAlerts_TheMonthlyLimitIsReadFromTheGatewaysOwnCounter(t *testing.T) {
	seedWalletCast(t, 100000, 0, 100000)
	_, watch := seedAlertWatch(t)
	sent := captureOrgAlertNotices(t)
	now := time.Now()
	// The counter lives for as long as the test binary does: a key id of this
	// run's own keeps a repeated run from finding the last one's count.
	keyID := 300000 + int(now.UnixNano()%1000000)
	seedSecondOrgKey(t, keyID, "Batch job")
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", keyID).
		Updates(map[string]any{"monthly_limit": 10, "unlimited_quota": true, "accessed_time": now.Unix()}).Error)
	request := func() bool {
		allowed, err := tenantquota.CheckMonthly(context.Background(), nil, keyID, 10)
		require.NoError(t, err)
		return allowed
	}

	for n := 1; n <= 7; n++ {
		require.True(t, request())
	}
	watch.pass(now)
	require.Empty(t, sent(), "seven of ten")

	require.True(t, request())
	useKey(t, keyID, 0, 0, now.Add(time.Minute))
	watch.pass(now.Add(time.Minute))
	notices := sent()
	require.Len(t, notices, 3)
	assert.Equal(t, "Key “Batch job” (member) has made 8 of the 10 requests it may make this month (80%).<br/>"+listLinkHTML(),
		noticesByUser(notices)[walletOwnerID][0].notice.Content)

	require.True(t, request())
	require.True(t, request())
	require.False(t, request(), "the eleventh request is the gateway's to refuse")
	useKey(t, keyID, 0, 0, now.Add(10*time.Minute))
	watch.pass(now.Add(10 * time.Minute))
	notices = sent()
	require.Len(t, notices, 6)
	assert.Equal(t, "Key “Batch job” (member) has made all 10 requests it may make this month and stops working until next month. An administrator of your organization can raise the limit.<br/>"+listLinkHTML(),
		noticesByUser(notices)[walletOwnerID][1].notice.Content)
}

// --- an anomaly, from the usage log to the notification -------------------------

// logOrgUsage writes one usage log line of an organization key held by the
// member.
func logOrgUsage(t *testing.T, keyID int, at time.Time, quota int, ip string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.Log{
		UserId: walletMemberID, TokenId: keyID, OrgId: walletOrgID, DepartmentId: walletDepartmentID,
		Type: model.LogTypeConsume, CreatedAt: at.Unix(), Quota: quota, Ip: ip,
	}).Error)
}

// Acceptance: 异常检测…命中后写入组织告警列表并通知 admin. An anomaly goes to the
// owner and the admins — not to the key's holder, who may not be the one using
// it — and says that nothing was blocked. The usage log is read every ten
// minutes, not on every pass.
func TestOrgAlerts_AnAnomalyGoesToTheAdminsAndIsLookedForEveryTenMinutes(t *testing.T) {
	seedWalletCast(t, 100000, 0, 100000)
	adminID, watch := seedAlertWatch(t)
	sent := captureOrgAlertNotices(t)
	now := time.Now()
	seedSecondOrgKey(t, 203, "Second key")
	spent := 3 * int(common.QuotaPerUnit)
	logOrgUsage(t, walletMemberKeyID, now.Add(-time.Hour), spent, "203.0.113.5")

	watch.pass(now)

	notices := sent()
	byUser := noticesByUser(notices)
	require.Len(t, notices, 2, "the owner and the admin")
	require.Contains(t, byUser, walletOwnerID)
	require.Contains(t, byUser, adminID)
	owner := byUser[walletOwnerID][0]
	assert.Equal(t, "2 new alert(s) about your organization's keys", owner.notice.Title)
	assert.Equal(t, []string{
		"Key “test_token” (member) spent " + logger.FormatQuota(spent) + " in the last 24 hours. Over the week before it spent " + logger.FormatQuota(0) + " a day.",
		"Key “test_token” (member) was used from an address it had not been used from in the last 30 days: 203.0.113.5.",
		"No request was blocked. If a key is in the wrong hands, freeze or rotate it on the organization keys page.",
		listLinkHTML(),
	}, strings.Split(owner.notice.Content, "<br/>"))
	admin := byUser[adminID][0]
	assert.Equal(t, []string{
		"密钥「test_token」（member）最近 24 小时花了 " + logger.FormatQuota(spent) + "，此前一周平均每天 " + logger.FormatQuota(0) + "。",
		"密钥「test_token」（member）出现了过去 30 天没用过的 IP：203.0.113.5。",
		"没有任何请求被拦截。如果密钥落到了不该用它的人手里，请到「企业密钥」页冻结它或更换密钥值。",
		listLinkPlain(),
	}, strings.Split(admin.notice.Content, "\n"))

	// Another key starts spending. The passes of the next minutes do not read
	// the usage log; the one ten minutes on does.
	logOrgUsage(t, 203, now.Add(time.Minute), spent, "")
	for minute := 2; minute <= 9; minute++ {
		watch.pass(now.Add(time.Duration(minute) * time.Minute))
	}
	var alerts int64
	require.NoError(t, model.DB.Model(&orgmodel.OrgAlert{}).Count(&alerts).Error)
	assert.EqualValues(t, 2, alerts)
	watch.pass(now.Add(10 * time.Minute))
	require.NoError(t, model.DB.Model(&orgmodel.OrgAlert{}).Count(&alerts).Error)
	assert.EqualValues(t, 3, alerts)
	assert.Len(t, sent(), 4)
	// 🔴 Nothing was blocked: both keys are still switched on.
	var keys []model.Token
	require.NoError(t, model.DB.Where("id IN ?", []int{walletMemberKeyID, 203}).Find(&keys).Error)
	for _, key := range keys {
		assert.Equal(t, common.TokenStatusEnabled, key.Status, key.Name)
	}
}

// --- when telling somebody fails ---------------------------------------------------

// A member who cannot be reached does not cost the others their notification,
// and the alerts are not sent a second time because of it.
func TestOrgAlerts_OneUnreachableMemberDoesNotCostTheOthers(t *testing.T) {
	seedWalletCast(t, 100000, 0, 100000)
	adminID, watch := seedAlertWatch(t)
	now := time.Now()
	useKey(t, walletMemberKeyID, 4000, 1000, now)
	var mu sync.Mutex
	var reached []int
	real := sendOrgAlertNotice
	sendOrgAlertNotice = func(userID int, _ string, _ dto.UserSetting, _ dto.Notify) error {
		if userID == walletOwnerID {
			return errors.New("the mail server is away")
		}
		mu.Lock()
		defer mu.Unlock()
		reached = append(reached, userID)
		return nil
	}
	t.Cleanup(func() { sendOrgAlertNotice = real })

	watch.pass(now)
	watch.pass(now.Add(10 * time.Minute))

	assert.ElementsMatch(t, []int{adminID, walletMemberID}, reached, "each of the others, once")
	assert.Zero(t, unsentAlertCount(t))
}

// A pass that blows up is reported and over; the task goes on to the next one.
func TestOrgAlerts_APassThatPanicsDoesNotTakeTheTaskDown(t *testing.T) {
	seedWalletCast(t, 100000, 0, 100000)
	_, watch := seedAlertWatch(t)
	now := time.Now()
	useKey(t, walletMemberKeyID, 4000, 1000, now)
	real := sendOrgAlertNotice
	sendOrgAlertNotice = func(int, string, dto.UserSetting, dto.Notify) error { panic("a sender gone wrong") }
	require.NotPanics(t, func() { watch.pass(now) })
	sendOrgAlertNotice = real
	sent := captureOrgAlertNotices(t)
	watch.pass(now.Add(time.Minute))
	assert.Len(t, sent(), 3, "what could not be sent then is sent now")
}

// --- the wording -----------------------------------------------------------------------

// The gap between two notifications to one organization follows the
// platform's own limit, whatever it is set to.
func TestOrgAlertDigestGap_StaysJustWideOfThePlatformsLimit(t *testing.T) {
	limit, window := constant.NotifyLimitCount, constant.NotificationLimitDurationMinute
	t.Cleanup(func() { constant.NotifyLimitCount, constant.NotificationLimitDurationMinute = limit, window })
	for _, c := range []struct {
		count  int
		window int
		want   time.Duration
	}{
		{2, 10, 6 * time.Minute},
		{1, 10, 11 * time.Minute},
		{5, 60, 13 * time.Minute},
		{0, 0, time.Minute},
	} {
		constant.NotifyLimitCount, constant.NotificationLimitDurationMinute = c.count, c.window
		assert.Equal(t, c.want, orgAlertDigestGap(), "%d per %d minutes", c.count, c.window)
	}
}

// Every rule has a line, in each of the three languages the console speaks.
func TestOrgAlertNotice_WordsEveryRuleInEveryLanguage(t *testing.T) {
	require.NoError(t, i18n.Init())
	dollar := int(common.QuotaPerUnit)
	one, three, eight, ten := logger.FormatQuota(dollar), logger.FormatQuota(3*dollar), logger.FormatQuota(8*dollar), logger.FormatQuota(10*dollar)
	alerts := []orgservice.AlertNotice{
		{Rule: orgmodel.AlertRuleQuota, Level: 80, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Used: 8 * dollar, Limit: 10 * dollar}},
		{Rule: orgmodel.AlertRuleQuota, Level: 100, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Used: 10 * dollar, Limit: 10 * dollar}},
		{Rule: orgmodel.AlertRuleQuota, Level: 100, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Used: 9 * dollar, Limit: 10 * dollar, Needed: 3 * dollar}},
		{Rule: orgmodel.AlertRuleMonthly, Level: 80, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Used: 850, Limit: 1000}},
		{Rule: orgmodel.AlertRuleMonthly, Level: 100, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Used: 1000, Limit: 1000}},
		{Rule: orgmodel.AlertRuleSpike, Holder: "CI", Detail: orgmodel.AlertDetail{Key: "Pipeline", Spent: 8 * dollar, DailyAverage: dollar}},
		{Rule: orgmodel.AlertRuleOffHours, Holder: "CI", Detail: orgmodel.AlertDetail{Key: "Pipeline", Spent: 3 * dollar, DailyAverage: dollar}},
		{Rule: orgmodel.AlertRuleNewIP, Holder: "CI", Detail: orgmodel.AlertDetail{Key: "Pipeline", Ips: []string{"192.0.2.77", "192.0.2.78"}, KnownIps: 2}},
	}
	for language, want := range map[string][]string{
		"en": {
			"Key “Design” (Ada) has used 80% of its quota: " + eight + " of " + ten + ".",
			"Key “Design” (Ada) has used up its quota of " + ten + " and no longer works. An administrator of your organization can give it more.",
			"Key “Design” (Ada) has " + one + " of its quota left — less than a request has to have in hand (" + three + " this time) — so its requests are refused. An administrator of your organization can give it more.",
			"Key “Design” (Ada) has made 850 of the 1000 requests it may make this month (85%).",
			"Key “Design” (Ada) has made all 1000 requests it may make this month and stops working until next month. An administrator of your organization can raise the limit.",
			"Key “Pipeline” (CI) spent " + eight + " in the last 24 hours. Over the week before it spent " + one + " a day.",
			"Key “Pipeline” (CI) spent " + three + " outside working hours in the last 24 hours. Over the week before it spent " + one + " a day.",
			"Key “Pipeline” (CI) was used from an address it had not been used from in the last 30 days: 192.0.2.77, 192.0.2.78.",
			"No request was blocked. If a key is in the wrong hands, freeze or rotate it on the organization keys page.",
		},
		"zh": {
			"密钥「Design」（Ada）的额度已用 80%：共 " + ten + "，已用 " + eight + "。",
			"密钥「Design」（Ada）的额度 " + ten + " 已经用完，不能再调用。企业管理员可以给它增加额度。",
			"密钥「Design」（Ada）的额度只剩 " + one + "，不够完成一次请求（这次需要预留 " + three + "），请求已被拒绝。企业管理员可以给它增加额度。",
			"密钥「Design」（Ada）本月已发出 850 次请求，月限额是 1000 次（85%）。",
			"密钥「Design」（Ada）本月的 1000 次请求已经用完，下个月之前不能再调用。企业管理员可以调高月限额。",
			"密钥「Pipeline」（CI）最近 24 小时花了 " + eight + "，此前一周平均每天 " + one + "。",
			"密钥「Pipeline」（CI）最近 24 小时在非工作时间花了 " + three + "，此前一周平均每天 " + one + "。",
			"密钥「Pipeline」（CI）出现了过去 30 天没用过的 IP：192.0.2.77, 192.0.2.78。",
			"没有任何请求被拦截。如果密钥落到了不该用它的人手里，请到「企业密钥」页冻结它或更换密钥值。",
		},
		"zh-TW": {
			"金鑰「Design」（Ada）的額度已用 80%：共 " + ten + "，已用 " + eight + "。",
			"金鑰「Design」（Ada）的額度 " + ten + " 已經用完，不能再呼叫。企業管理員可以為它增加額度。",
			"金鑰「Design」（Ada）的額度只剩 " + one + "，不夠完成一次請求（這次需要預留 " + three + "），請求已被拒絕。企業管理員可以為它增加額度。",
			"金鑰「Design」（Ada）本月已發出 850 次請求，月限額是 1000 次（85%）。",
			"金鑰「Design」（Ada）本月的 1000 次請求已經用完，下個月之前不能再呼叫。企業管理員可以調高月限額。",
			"金鑰「Pipeline」（CI）最近 24 小時花了 " + eight + "，此前一週平均每天 " + one + "。",
			"金鑰「Pipeline」（CI）最近 24 小時在非工作時間花了 " + three + "，此前一週平均每天 " + one + "。",
			"金鑰「Pipeline」（CI）出現了過去 30 天沒用過的 IP：192.0.2.77, 192.0.2.78。",
			"沒有任何請求被攔截。如果金鑰落到了不該用它的人手裡，請到「企業金鑰」頁凍結它或更換金鑰值。",
		},
	} {
		notice := orgAlertNotice(dto.UserSetting{Language: language, NotifyType: dto.NotifyTypeBark}, alerts, "")
		assert.Equal(t, want, strings.Split(notice.Content, "\n"), language)
		assert.Equal(t, notifyTypeOrgAlert, notice.Type, language)
		assert.Contains(t, notice.Title, "8", language)
		assert.NotContains(t, notice.Title+notice.Content, "org.alert_", "%s: every message is translated", language)
		assert.NotContains(t, notice.Title+notice.Content, "<no value>", "%s: every message is given what it names", language)
	}
}

// What a notification is made of besides the lines: the closing line only
// when an anomaly is reported, a cap on how many alerts are spelled out, and
// names that cannot be read as markup.
func TestOrgAlertNotice_ClosingLineCapAndEscaping(t *testing.T) {
	require.NoError(t, i18n.Init())
	warning := orgservice.AlertNotice{Rule: orgmodel.AlertRuleQuota, Level: 80, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Used: 80, Limit: 100}}
	mail := dto.UserSetting{Language: "en"}

	onlyWarnings := orgAlertNotice(mail, []orgservice.AlertNotice{warning, warning}, "")
	assert.Len(t, strings.Split(onlyWarnings.Content, "<br/>"), 2, "a notification about warnings has no closing line")
	assert.NotContains(t, onlyWarnings.Content, "blocked")

	many := make([]orgservice.AlertNotice, 0, 26)
	for n := 0; n < 25; n++ {
		many = append(many, warning)
	}
	many = append(many, orgservice.AlertNotice{Rule: orgmodel.AlertRuleSpike, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Spent: 5}})
	long := strings.Split(orgAlertNotice(mail, many, "").Content, "<br/>")
	require.Len(t, long, orgAlertLinesShown+2, "twenty lines, how many were left out, and the closing line")
	assert.Equal(t, "…and 6 more.", long[orgAlertLinesShown])
	assert.Contains(t, long[orgAlertLinesShown+1], "No request was blocked",
		"the closing line is there for an anomaly that was left out as well")
	assert.Equal(t, "26 new alert(s) about your organization's keys", orgAlertNotice(mail, many, "").Title)

	hostile := orgservice.AlertNotice{
		Rule: orgmodel.AlertRuleNewIP, Holder: `Eve & "Co"`,
		Detail: orgmodel.AlertDetail{Key: "<script>alert(1)</script>", Ips: []string{"<img src=x>"}},
	}
	html := orgAlertNotice(mail, []orgservice.AlertNotice{hostile}, "").Content
	assert.NotContains(t, html, "<script>")
	assert.NotContains(t, html, "<img")
	assert.Contains(t, html, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.Contains(t, html, "Eve &amp; &#34;Co&#34;")
	for _, channel := range []string{dto.NotifyTypeBark, dto.NotifyTypeGotify} {
		plain := orgAlertNotice(dto.UserSetting{Language: "en", NotifyType: channel}, []orgservice.AlertNotice{hostile}, "").Content
		assert.Contains(t, plain, `Key “<script>alert(1)</script>” (Eve & "Co")`, channel)
	}
	for _, channel := range []string{"", dto.NotifyTypeEmail, dto.NotifyTypeWebhook} {
		markup := orgAlertNotice(dto.UserSetting{Language: "en", NotifyType: channel}, []orgservice.AlertNotice{hostile, hostile}, "").Content
		assert.NotContains(t, markup, "<script>", channel)
		assert.Contains(t, markup, "<br/>", channel)
	}

	// A rule this build has no words for still names the key.
	unknown := orgAlertNotice(mail, []orgservice.AlertNotice{{Rule: "from_the_future", Detail: orgmodel.AlertDetail{Key: "A<b>"}}}, "")
	assert.Equal(t, "from_the_future: A&lt;b&gt;", unknown.Content)
}

// Enterprise Org P9: now that there is a page to read alerts on, a
// notification ends with a link to it — the last line, after the one that
// says nothing was blocked — in the form the recipient's channel shows and in
// every language. Everyone who is told gets it: since D47 a key's holder can
// open the list too, and finds the warnings on their own keys there.
func TestOrgAlertNotice_EndsWithALinkToTheAlertList(t *testing.T) {
	require.NoError(t, i18n.Init())
	address := system_setting.ServerAddress
	system_setting.ServerAddress = "https://gateway.example"
	t.Cleanup(func() { system_setting.ServerAddress = address })
	link := "https://gateway.example/org/reports?section=alerts"
	require.Equal(t, link, orgAlertListLink(), "the alerts section of the reports page")

	warning := orgservice.AlertNotice{Rule: orgmodel.AlertRuleQuota, Level: 80, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Used: 80, Limit: 100}}
	spike := orgservice.AlertNotice{Rule: orgmodel.AlertRuleSpike, Holder: "Ada", Detail: orgmodel.AlertDetail{Key: "Design", Spent: 5}}
	for language, words := range map[string]string{"en": "See all alerts", "zh": "查看全部告警", "zh-TW": "查看全部提醒"} {
		mail := strings.Split(orgAlertNotice(dto.UserSetting{Language: language}, []orgservice.AlertNotice{warning, spike}, link).Content, "<br/>")
		require.Len(t, mail, 4, "%s: two alerts, the closing line and the link", language)
		assert.Equal(t, "<a href='"+link+"'>"+words+"</a>", mail[3], language)
		for _, channel := range []string{dto.NotifyTypeBark, dto.NotifyTypeGotify} {
			plain := strings.Split(orgAlertNotice(dto.UserSetting{Language: language, NotifyType: channel}, []orgservice.AlertNotice{warning}, link).Content, "\n")
			require.Len(t, plain, 2, "%s over %s: one alert and the link", language, channel)
			assert.True(t, strings.HasPrefix(plain[1], words), "%s over %s: %s", language, channel, plain[1])
			assert.True(t, strings.HasSuffix(plain[1], link), "%s over %s: %s", language, channel, plain[1])
			assert.NotContains(t, plain[1], "<a", "%s over %s", language, channel)
		}
	}
	// Given no link, the notification is the lines alone.
	without := orgAlertNotice(dto.UserSetting{Language: "en"}, []orgservice.AlertNotice{warning, spike}, "").Content
	assert.Len(t, strings.Split(without, "<br/>"), 3)
	assert.NotContains(t, without, "/org/reports")
}
