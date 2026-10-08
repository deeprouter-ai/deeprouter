package router

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// alertPage is what GET /api/org/alerts answers with.
type alertPage struct {
	Total int `json:"total"`
	Items []struct {
		Id         int    `json:"id"`
		Rule       string `json:"rule"`
		Level      int    `json:"level"`
		KeyId      int    `json:"key_id"`
		HolderId   int    `json:"holder_id"`
		Department string `json:"department"`
		State      string `json:"state"`
		AckedBy    int    `json:"acked_by"`
		Detail     struct {
			Key   string   `json:"key"`
			Used  int      `json:"used"`
			Limit int      `json:"limit"`
			Spent int      `json:"spent"`
			Ips   []string `json:"ips"`
		} `json:"detail"`
	} `json:"items"`
}

// rules lists what a page of alerts is about, in the order it came, as "rule"
// or "rule@level".
func (p alertPage) rules() []string {
	rules := make([]string, 0, len(p.Items))
	for _, item := range p.Items {
		if item.Level != 0 {
			rules = append(rules, item.Rule+"@"+strconv.Itoa(item.Level))
			continue
		}
		rules = append(rules, item.Rule)
	}
	return rules
}

// TestOrgAlerts_ThroughTheRealGateway walks Enterprise Org P8 (meta-repo
// docs/enterprise-org-prd.md §4) end to end. Members' requests go through the
// gateway's real key authentication, relay, settlement and usage log to a
// provider that is the only thing faked; the two scans the background task
// runs are then called on what those requests left behind, and the alerts are
// read and dealt with over the real routes.
//
// Besides the two acceptance items it pins the half of the second one that is
// a negative: 不自动拦截任何请求. With every rule having spoken about a key, the
// key's next request is served like any other.
func TestOrgAlerts_ThroughTheRealGateway(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		provider := newFakeProvider(t)
		engine := newRelayEngine(t, db, provider)
		id := strconv.Itoa
		valueOf := func(keyID int) string {
			var key model.Token
			require.NoError(t, db.First(&key, keyID).Error)
			return "sk-" + key.Key
		}
		usageOf := func(userID int) []model.Log {
			var logs []model.Log
			require.NoError(t, db.Where("type = ? AND user_id = ?", model.LogTypeConsume, userID).Order("id").Find(&logs).Error)
			return logs
		}
		// served sends one request with a key, expects it to be answered, and
		// waits for its settlement, which runs after the answer is out.
		served := func(who string, userID int, keyID int) model.Log {
			t.Helper()
			before := len(usageOf(userID))
			calls := provider.calls.Load()
			answer := chat(t, engine, valueOf(keyID))
			require.Equal(t, http.StatusOK, answer.Code, "%s: %s", who, answer.Body.String())
			require.Equal(t, calls+1, provider.calls.Load(), "%s: the request reached the provider", who)
			deadline := time.Now().Add(5 * time.Second)
			for len(usageOf(userID)) == before {
				require.True(t, time.Now().Before(deadline), "timed out waiting for the usage of %s to be logged", who)
				time.Sleep(10 * time.Millisecond)
			}
			logs := usageOf(userID)
			return logs[len(logs)-1]
		}
		noMonthlyLimits := func(int) (int, error) { return 0, nil }

		// --- a company, three members, two keys --------------------------------
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
		join := func(username string, role string, department string) browser {
			var invite struct {
				Code string `json:"code"`
			}
			owner.ok(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[role], "department_id": departmentID[department]}, &invite)
			return signUp(t, engine, map[string]any{"username": username, "password": "password123", "org_invite": invite.Code}, "")
		}
		seller := join("seller", orgmodel.RoleStaff, "Sales")
		manager := join("manager", orgmodel.RoleManager, "Sales")
		maker := join("maker", orgmodel.RoleStaff, "Product")
		personal := signUp(t, engine, map[string]any{"username": "solo", "password": "password123"}, "")
		for _, account := range []browser{owner, personal} {
			require.NoError(t, db.Model(&model.User{}).Where("id = ?", account.userID).Update("quota", 10000000).Error)
		}
		var sellersKey, makersKey, personalKey struct {
			Id int `json:"id"`
		}
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Sales tools", "holder_id": seller.userID, "remain_quota": 1000000}, &sellersKey)
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Product tools", "holder_id": maker.userID, "unlimited_quota": true}, &makersKey)
		personal.ok(t, http.MethodPost, "/api/token/", map[string]any{"name": "mine", "expired_time": -1, "unlimited_quota": true}, &personalKey)

		// --- an organization key's usage always says where it came from ---------
		first := served("the seller", seller.userID, sellersKey.Id)
		cost := first.Quota
		require.Positive(t, cost)
		assert.Equal(t, "192.0.2.1", first.Ip, "the address the request came from, which nobody switched on")
		assert.Equal(t, "192.0.2.1", served("the maker", maker.userID, makersKey.Id).Ip)
		assert.Empty(t, served("the personal account", personal.userID, personalKey.Id).Ip,
			"a personal key records it only when its user asked for that")

		// --- the settings, over the real routes ---------------------------------
		for name, visitor := range map[string]browser{"the manager": manager, "a staff member": seller} {
			status, _, message, _ := visitor.call(t, http.MethodGet, "/api/org/alert-settings", nil)
			assert.Equal(t, http.StatusForbidden, status, name)
			assert.Equal(t, "org.forbidden", message, name)
			visitor.refused(t, http.MethodPut, "/api/org/alert-settings", orgmodel.DefaultAlertSettings())
		}
		status, _, message, _ := personal.call(t, http.MethodGet, "/api/org/alert-settings", nil)
		assert.Equal(t, http.StatusForbidden, status)
		assert.Equal(t, "org.not_member", message)

		var settings orgmodel.AlertSettings
		owner.ok(t, http.MethodGet, "/api/org/alert-settings", nil, &settings)
		assert.Equal(t, orgmodel.DefaultAlertSettings(), settings, "a new organization is on the defaults")

		bad := orgmodel.DefaultAlertSettings()
		bad.WarnAt = []int{120}
		status, success, message, _ := owner.call(t, http.MethodPut, "/api/org/alert-settings", bad)
		assert.Equal(t, http.StatusOK, status)
		assert.False(t, success)
		assert.Equal(t, "org.alert_settings_invalid", message)

		// The company sets no floor, and calls one day of the week its working
		// week — a day that is not today, so that whatever is spent while this
		// test runs is spent outside working hours.
		wanted := orgmodel.DefaultAlertSettings()
		wanted.MinSpend = 0
		wanted.WorkHours = &orgmodel.WorkHours{
			Timezone: "UTC", Days: []int{(int(time.Now().UTC().Weekday()) + 3) % 7}, Start: 0, End: 24 * 60,
		}
		owner.ok(t, http.MethodPut, "/api/org/alert-settings", wanted, &settings)
		assert.Equal(t, wanted, settings)

		// --- the three anomaly rules, on real usage -------------------------------
		// Both keys have been with their holders for a month.
		require.NoError(t, db.Model(&model.Token{}).Where("id IN ?", []int{sellersKey.Id, makersKey.Id}).
			Update("created_time", time.Now().Add(-30*24*time.Hour).Unix()).Error)
		raised, err := orgservice.ScanAnomalies(db, db, time.Now())
		require.NoError(t, err)
		assert.Equal(t, 6, raised, "a spike, spending outside working hours and an unfamiliar address, for each of the two keys")

		var everything alertPage
		owner.ok(t, http.MethodGet, "/api/org/alerts", nil, &everything)
		assert.Equal(t, 6, everything.Total)
		assert.Equal(t, []string{
			orgmodel.AlertRuleNewIP, orgmodel.AlertRuleOffHours, orgmodel.AlertRuleSpike,
			orgmodel.AlertRuleNewIP, orgmodel.AlertRuleOffHours, orgmodel.AlertRuleSpike,
		}, everything.rules(), "newest first")
		newAddress := everything.Items[0]
		assert.Equal(t, makersKey.Id, newAddress.KeyId)
		assert.Equal(t, maker.userID, newAddress.HolderId)
		assert.Equal(t, "Product", newAddress.Department)
		assert.Equal(t, "Product tools", newAddress.Detail.Key)
		assert.Equal(t, []string{"192.0.2.1"}, newAddress.Detail.Ips)
		assert.Equal(t, orgmodel.AlertStateOpen, newAddress.State)
		assert.Equal(t, cost, everything.Items[5].Detail.Spent, "the seller's spike says what the seller spent")

		// 🔴 不自动拦截任何请求: every rule has spoken about both keys, and both
		// are served exactly as before.
		served("the seller, alerts and all", seller.userID, sellersKey.Id)
		served("the maker, alerts and all", maker.userID, makersKey.Id)

		// --- a warning, on real spending -------------------------------------------
		// The seller's key has made two requests. The owner leaves it thirteen
		// more: fifteen requests' worth in all, so the twelfth takes it to 80%.
		owner.ok(t, http.MethodPut, "/api/org/keys/"+id(sellersKey.Id), map[string]any{"remain_quota": 13 * cost}, nil)
		scanAllowances := func() {
			t.Helper()
			_, err := orgservice.ScanAllowances(db, 0, time.Now(), noMonthlyLimits)
			require.NoError(t, err)
		}
		warnings := func() []string {
			var page alertPage
			owner.ok(t, http.MethodGet, "/api/org/alerts?page_size=100", nil, &page)
			var rules []string
			for i, rule := range page.rules() {
				if page.Items[i].Rule == orgmodel.AlertRuleQuota {
					rules = append(rules, rule)
				}
			}
			return rules
		}
		for request := 3; request <= 11; request++ {
			served("the seller's request "+id(request), seller.userID, sellersKey.Id)
		}
		scanAllowances()
		assert.Empty(t, warnings(), "eleven fifteenths used")
		served("the seller's twelfth request", seller.userID, sellersKey.Id)
		scanAllowances()
		require.Equal(t, []string{"quota@80"}, warnings(), "four fifths used")
		scanAllowances()
		assert.Equal(t, []string{"quota@80"}, warnings(), "and told once")
		owner.ok(t, http.MethodGet, "/api/org/alerts?page_size=1", nil, &everything)
		warning := everything.Items[0]
		assert.Equal(t, sellersKey.Id, warning.KeyId)
		assert.Equal(t, "Sales", warning.Department)
		assert.Equal(t, "Sales tools", warning.Detail.Key)
		assert.Equal(t, []int{12 * cost, 15 * cost}, []int{warning.Detail.Used, warning.Detail.Limit})
		// A warning does not block either.
		served("the seller, warning and all", seller.userID, sellersKey.Id)

		// --- who reads the list -------------------------------------------------------
		owner.ok(t, http.MethodGet, "/api/org/alerts?page_size=100", nil, &everything)
		assert.Equal(t, 7, everything.Total)
		var managers alertPage
		manager.ok(t, http.MethodGet, "/api/org/alerts?page_size=100", nil, &managers)
		assert.Equal(t, 4, managers.Total, "the manager of Sales sees what was raised in Sales")
		for _, item := range managers.Items {
			assert.Equal(t, sellersKey.Id, item.KeyId)
			assert.Equal(t, "Sales", item.Department)
		}
		// A member whose role reads no alerts is sent the warnings on their own
		// keys (PRD D47): the seller the one about "Sales tools" — not the three
		// anomalies on the same key — and the maker, whose key has only ever set
		// off anomalies, nothing.
		var sellers, makers alertPage
		seller.ok(t, http.MethodGet, "/api/org/alerts?page_size=100", nil, &sellers)
		require.Equal(t, []string{"quota@80"}, sellers.rules())
		assert.Equal(t, 1, sellers.Total)
		assert.Equal(t, sellersKey.Id, sellers.Items[0].KeyId)
		maker.ok(t, http.MethodGet, "/api/org/alerts?page_size=100", nil, &makers)
		assert.Zero(t, makers.Total)
		assert.Empty(t, makers.Items)
		seller.refused(t, http.MethodGet, "/api/org/alert-settings", nil)
		status, _, message, _ = personal.call(t, http.MethodGet, "/api/org/alerts", nil)
		assert.Equal(t, http.StatusForbidden, status)
		assert.Equal(t, "org.not_member", message)

		// --- dealing with one ---------------------------------------------------------
		spike := everything.Items[len(everything.Items)-1]
		require.Equal(t, orgmodel.AlertRuleSpike, spike.Rule)
		handle := "/api/org/alerts/" + id(spike.Id) + "/handle"
		manager.refused(t, http.MethodPost, handle, map[string]any{"state": orgmodel.AlertStateHandled})
		seller.refused(t, http.MethodPost, handle, map[string]any{"state": orgmodel.AlertStateHandled})
		for path, body := range map[string]map[string]any{
			handle:                          {"state": "ignored"},
			"/api/org/alerts/424242/handle": {"state": orgmodel.AlertStateHandled},
		} {
			status, success, message, _ := owner.call(t, http.MethodPost, path, body)
			assert.Equal(t, http.StatusOK, status, path)
			assert.False(t, success, path)
			assert.Contains(t, []string{"org.alert_state_invalid", "org.alert_not_found"}, message, path)
		}
		owner.ok(t, http.MethodPost, handle, map[string]any{"state": orgmodel.AlertStateFalseAlarm}, nil)
		var open alertPage
		owner.ok(t, http.MethodGet, "/api/org/alerts?state=open&page_size=100", nil, &open)
		assert.Equal(t, 6, open.Total)
		for _, item := range open.Items {
			assert.NotEqual(t, spike.Id, item.Id)
		}
		owner.ok(t, http.MethodGet, "/api/org/alerts?page_size=100", nil, &everything)
		dealtWith := everything.Items[len(everything.Items)-1]
		assert.Equal(t, orgmodel.AlertStateFalseAlarm, dealtWith.State)
		assert.Equal(t, owner.userID, dealtWith.AckedBy)

		// Both writes are in the audit log.
		var audit struct {
			Items []struct {
				Action string `json:"action"`
			} `json:"items"`
		}
		owner.ok(t, http.MethodGet, "/api/org/audit-logs?page_size=100", nil, &audit)
		recorded := map[string]int{}
		for _, item := range audit.Items {
			recorded[item.Action]++
		}
		assert.Equal(t, 1, recorded[orgmodel.AuditSettingsUpdate])
		assert.Equal(t, 1, recorded[orgmodel.AuditAlertHandle])

		// --- who is told ---------------------------------------------------------------
		unsent, err := orgservice.UnsentAlerts(db, time.Now())
		require.NoError(t, err)
		var org orgmodel.Organization
		require.NoError(t, db.First(&org).Error)
		require.Len(t, unsent[org.Id], 7, "dealing with an alert in the list does not stop it from being sent")
		digests, err := orgservice.DigestsFor(db, org.Id, unsent[org.Id])
		require.NoError(t, err)
		told := map[int]int{}
		for _, digest := range digests {
			told[digest.Recipient.Id] = len(digest.Alerts)
		}
		assert.Equal(t, map[int]int{owner.userID: 7, seller.userID: 1}, told,
			"the owner hears everything, the seller the warning about their own key, the manager and the maker nothing")
	})
}
