package router

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// usagePage is what GET /api/org/usage answers with.
type usagePage struct {
	GroupBy     string `json:"group_by"`
	Scope       string `json:"scope"`
	Departments []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"departments"`
	Total usageFigures `json:"total"`
	Rows  []struct {
		Id        int      `json:"id"`
		Name      string   `json:"name"`
		IsService bool     `json:"is_service"`
		Gone      bool     `json:"gone"`
		UsedBy    []string `json:"used_by"`
		usageFigures
	} `json:"rows"`
}

// usageFigures are the measures of a report row and of its total.
type usageFigures struct {
	Requests int `json:"requests"`
	Quota    int `json:"quota"`
}

// requestsBy maps the rows of a report to how many requests each stands for.
func (p usagePage) requestsBy() map[string]int {
	requests := map[string]int{}
	for _, row := range p.Rows {
		requests[row.Name] = row.Requests
	}
	return requests
}

// TestOrgUsage_ThroughTheRealGateway walks Enterprise Org P9 (meta-repo
// docs/enterprise-org-prd.md §6) end to end. Members' requests go through the
// gateway's real key authentication, relay, settlement and usage log to a
// provider that is the only thing faked; the reports are then read over the
// real routes by an owner, a read-only member, a manager and two staff
// members, each of whom is shown their part — and is answered with a real 403
// when they ask for more.
//
// It closes three acceptance items other cards left open until there was a
// report to look at: a service account's usage 出现在报表中 (P3), readonly 能看
// 全公司用量 and 部门作用域硬检查 … 报表 (P4).
func TestOrgUsage_ThroughTheRealGateway(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		provider := newFakeProvider(t)
		engine := newRelayEngine(t, db, provider)
		id := strconv.Itoa
		usageOf := func(userID int) []model.Log {
			var logs []model.Log
			require.NoError(t, db.Where("type = ? AND user_id = ?", model.LogTypeConsume, userID).Order("id").Find(&logs).Error)
			return logs
		}
		// served sends one request with a key, expects it to be answered, and
		// waits for its settlement, which runs after the answer is out.
		served := func(who string, userID int, keyID int) model.Log {
			t.Helper()
			var key model.Token
			require.NoError(t, db.First(&key, keyID).Error)
			before := len(usageOf(userID))
			answer := chat(t, engine, "sk-"+key.Key)
			require.Equal(t, http.StatusOK, answer.Code, "%s: %s", who, answer.Body.String())
			deadline := time.Now().Add(5 * time.Second)
			for len(usageOf(userID)) == before {
				require.True(t, time.Now().Before(deadline), "timed out waiting for the usage of %s to be logged", who)
				time.Sleep(10 * time.Millisecond)
			}
			logs := usageOf(userID)
			return logs[len(logs)-1]
		}

		// --- a company: four people in three departments, and a pipeline ---------
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
		auditor := join("auditor", orgmodel.RoleReadonly, "General")
		personal := signUp(t, engine, map[string]any{"username": "solo", "password": "password123"}, "")
		var pipeline struct {
			Id int `json:"id"`
		}
		owner.ok(t, http.MethodPost, "/api/org/service-accounts", map[string]any{"name": "CI Pipeline", "department_id": 0}, &pipeline)
		require.NoError(t, db.Model(&model.User{}).Where("id = ?", owner.userID).Update("quota", 10000000).Error)
		var sellersKey, makersKey, pipelineKey struct {
			Id int `json:"id"`
		}
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Sales tools", "holder_id": seller.userID, "unlimited_quota": true}, &sellersKey)
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Product tools", "holder_id": maker.userID, "unlimited_quota": true}, &makersKey)
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Pipeline", "holder_id": pipeline.Id, "unlimited_quota": true}, &pipelineKey)

		// --- a week's work: two requests in Sales, one in Product, one by the
		// pipeline; then the seller moves to Product and makes one more -----------
		cost := served("the seller", seller.userID, sellersKey.Id).Quota
		require.Positive(t, cost)
		served("the seller again", seller.userID, sellersKey.Id)
		served("the maker", maker.userID, makersKey.Id)
		served("the pipeline", pipeline.Id, pipelineKey.Id)
		owner.ok(t, http.MethodPut, "/api/org/members/"+id(seller.userID), map[string]any{"department_id": departmentID["Product"]}, nil)
		served("the seller, now in Product", seller.userID, sellersKey.Id)

		report := func(reader browser, query string) usagePage {
			t.Helper()
			var page usagePage
			reader.ok(t, http.MethodGet, "/api/org/usage?"+query, nil, &page)
			return page
		}
		everything := usageFigures{Requests: 5, Quota: 5 * cost}

		// --- the owner: the whole company, cut four ways ---------------------------
		byDepartment := report(owner, "group_by=department")
		assert.Equal(t, orgmodel.ScopeOrg, byDepartment.Scope)
		assert.Equal(t, everything, byDepartment.Total)
		assert.Equal(t, map[string]int{"Sales": 2, "Product": 2, "General": 1}, byDepartment.requestsBy(),
			"the seller's first two requests stay in Sales after the move")
		assert.Len(t, byDepartment.Departments, 6)

		byMember := report(owner, "group_by=member")
		assert.Equal(t, everything, byMember.Total)
		assert.Equal(t, map[string]int{"seller": 3, "maker": 1, "CI Pipeline": 1}, byMember.requestsBy())
		require.Len(t, byMember.Rows, 3)
		assert.Equal(t, seller.userID, byMember.Rows[0].Id, "the biggest spender first")
		assert.Equal(t, usageFigures{Requests: 3, Quota: 3 * cost}, byMember.Rows[0].usageFigures, "what the member's requests cost")
		// Acceptance of P3: 用量照常统计并出现在报表中.
		for _, row := range byMember.Rows {
			assert.Equal(t, row.Id == pipeline.Id, row.IsService, row.Name)
		}

		byKey := report(owner, "group_by=key")
		assert.Equal(t, everything, byKey.Total)
		assert.Equal(t, map[string]int{"Sales tools": 3, "Product tools": 1, "Pipeline": 1}, byKey.requestsBy())
		assert.Equal(t, []string{"seller"}, byKey.Rows[0].UsedBy)

		byModel := report(owner, "group_by=model")
		assert.Equal(t, everything, byModel.Total)
		assert.Equal(t, map[string]int{"gpt-4o-mini": 5}, byModel.requestsBy())

		// The period: all of this happened in the last minute.
		now := time.Now().Unix()
		assert.Equal(t, everything, report(owner, "group_by=model&start_timestamp="+id(int(now-60))+"&end_timestamp="+id(int(now+60))).Total)
		assert.Empty(t, report(owner, "group_by=model&end_timestamp="+id(int(now-60))).Rows)
		assert.Empty(t, report(owner, "group_by=model&start_timestamp="+id(int(now+60))).Rows)
		// One department of the owner's choosing.
		assert.Equal(t, map[string]int{"seller": 1, "maker": 1}, report(owner, "group_by=member&department_id="+id(departmentID["Product"])).requestsBy())

		// --- Acceptance of P4: readonly 能看全公司用量 ---------------------------------
		readonly := report(auditor, "group_by=member")
		assert.Equal(t, orgmodel.ScopeOrg, readonly.Scope)
		assert.Equal(t, everything, readonly.Total)

		// --- Acceptance: manager 默认只见本部门数据, 越权查询返回 403 ---------------------
		managers := report(manager, "group_by=member")
		assert.Equal(t, orgmodel.ScopeDept, managers.Scope)
		assert.Equal(t, usageFigures{Requests: 2, Quota: 2 * cost}, managers.Total)
		assert.Equal(t, map[string]int{"seller": 2}, managers.requestsBy())
		require.Len(t, managers.Departments, 1)
		assert.Equal(t, "Sales", managers.Departments[0].Name)
		assert.Equal(t, 2, report(manager, "group_by=key&department_id="+id(departmentID["Sales"])).Total.Requests)
		for _, groupBy := range []string{"department", "member", "key", "model"} {
			manager.refused(t, http.MethodGet, "/api/org/usage?group_by="+groupBy+"&department_id="+id(departmentID["Product"]), nil)
		}

		// --- Acceptance: staff 只能查看自己 ---------------------------------------------
		own := report(seller, "group_by=department")
		assert.Equal(t, orgmodel.ScopeSelf, own.Scope)
		assert.Equal(t, map[string]int{"Sales": 2, "Product": 1}, own.requestsBy(), "what the seller spent, wherever they sat")
		assert.Empty(t, own.Departments)
		assert.Equal(t, map[string]int{"seller": 3}, report(seller, "group_by=member").requestsBy())
		assert.Equal(t, map[string]int{"maker": 1}, report(maker, "group_by=member").requestsBy())
		for _, department := range []string{"Sales", "Product", "General"} {
			seller.refused(t, http.MethodGet, "/api/org/usage?group_by=member&department_id="+id(departmentID[department]), nil)
		}

		// A personal account has no organization to ask about.
		status, _, message, _ := personal.call(t, http.MethodGet, "/api/org/usage?group_by=member", nil)
		assert.Equal(t, http.StatusForbidden, status)
		assert.Equal(t, "org.not_member", message)

		// A question that makes no sense is turned down as one, not as a 403.
		for _, query := range []string{"", "group_by=team", "group_by=member&start_timestamp=yesterday", "group_by=member&end_timestamp=-5",
			"group_by=member&department_id=sales", "group_by=member&department_id=-3", "group_by=member&start_timestamp=200&end_timestamp=100"} {
			status, success, message, _ := owner.call(t, http.MethodGet, "/api/org/usage?"+query, nil)
			assert.Equal(t, http.StatusOK, status, query)
			assert.False(t, success, query)
			assert.Equal(t, "common.invalid_params", message, query)
		}

		// --- Enterprise Org P10: the same usage, drawn over time -------------------------
		// Acceptance: owner/admin 可查看用量随时间变化的折线图，按部门或模型分线，粒度可选
		// 日 / 周 / 月 / 年 … 以金额计; 趋势图的可见范围与报表一致 … 越权查询返回 403.
		type trendPage struct {
			GroupBy string   `json:"group_by"`
			Bucket  string   `json:"bucket"`
			Scope   string   `json:"scope"`
			Buckets []string `json:"buckets"`
			Total   int      `json:"total"`
			Series  []struct {
				Id     int    `json:"id"`
				Name   string `json:"name"`
				Other  bool   `json:"other"`
				Quota  int    `json:"quota"`
				Points []int  `json:"points"`
			} `json:"series"`
		}
		const inShanghai = "&timezone=Asia%2FShanghai"
		shanghai, err := time.LoadLocation("Asia/Shanghai")
		require.NoError(t, err)
		trend := func(reader browser, query string) trendPage {
			t.Helper()
			var page trendPage
			reader.ok(t, http.MethodGet, "/api/org/usage/trend?"+query+inShanghai, nil, &page)
			// Every line adds up to what it says, and the lines to the total.
			drawn := 0
			for _, series := range page.Series {
				require.Len(t, series.Points, len(page.Buckets), series.Name)
				sum := 0
				for _, point := range series.Points {
					sum += point
				}
				assert.Equal(t, series.Quota, sum, series.Name)
				drawn += series.Quota
			}
			assert.Equal(t, page.Total, drawn)
			return page
		}
		spentBy := func(page trendPage) map[string]int {
			spent := map[string]int{}
			for _, series := range page.Series {
				spent[series.Name] = series.Quota
			}
			return spent
		}
		// All of it was spent today, on the viewer's calendar.
		today := time.Now().In(shanghai).Format(time.DateOnly)
		overTime := trend(owner, "group_by=department&bucket=day")
		assert.Equal(t, "department", overTime.GroupBy)
		assert.Equal(t, "day", overTime.Bucket)
		assert.Equal(t, orgmodel.ScopeOrg, overTime.Scope)
		assert.Equal(t, everything.Quota, overTime.Total, "what the report says for the same question")
		assert.Equal(t, map[string]int{"Sales": 2 * cost, "Product": 2 * cost, "General": cost}, spentBy(overTime))
		require.NotEmpty(t, overTime.Buckets)
		assert.Contains(t, []string{today, time.Now().In(shanghai).Format(time.DateOnly)}, overTime.Buckets[len(overTime.Buckets)-1])
		assert.Equal(t, map[string]int{"gpt-4o-mini": 5 * cost}, spentBy(trend(owner, "group_by=model&bucket=day")))
		for _, bucket := range []string{"week", "month", "year"} {
			wider := trend(owner, "group_by=model&bucket="+bucket)
			assert.Equal(t, bucket, wider.Bucket)
			assert.Equal(t, everything.Quota, wider.Total, bucket)
			assert.Len(t, wider.Buckets, 1, bucket)
		}
		// The period and the department filter are the report's.
		assert.Empty(t, trend(owner, "group_by=model&bucket=day&end_timestamp="+id(int(now-60))).Series)
		assert.Equal(t, 2*cost, trend(owner, "group_by=model&bucket=day&department_id="+id(departmentID["Product"])).Total)
		// A read-only member sees the whole company; the manager of Sales what
		// was spent in Sales, and is refused any other department.
		assert.Equal(t, everything.Quota, trend(auditor, "group_by=department&bucket=day").Total)
		managersTrend := trend(manager, "group_by=department&bucket=day")
		assert.Equal(t, orgmodel.ScopeDept, managersTrend.Scope)
		assert.Equal(t, map[string]int{"Sales": 2 * cost}, spentBy(managersTrend))
		assert.Equal(t, 2*cost, trend(manager, "group_by=model&bucket=day&department_id="+id(departmentID["Sales"])).Total)
		for _, groupBy := range []string{"department", "model"} {
			manager.refused(t, http.MethodGet, "/api/org/usage/trend?bucket=day&group_by="+groupBy+"&department_id="+id(departmentID["Product"])+inShanghai, nil)
		}
		// Staff have no chart on the page; asked directly, the route answers
		// them as the report does — with their own usage, and no department.
		sellersTrend := trend(seller, "group_by=department&bucket=day")
		assert.Equal(t, orgmodel.ScopeSelf, sellersTrend.Scope)
		assert.Equal(t, map[string]int{"Sales": 2 * cost, "Product": cost}, spentBy(sellersTrend))
		for _, department := range []string{"Sales", "Product", "General"} {
			seller.refused(t, http.MethodGet, "/api/org/usage/trend?bucket=day&group_by=model&department_id="+id(departmentID[department])+inShanghai, nil)
		}
		status, _, message, _ = personal.call(t, http.MethodGet, "/api/org/usage/trend?group_by=model&bucket=day"+inShanghai, nil)
		assert.Equal(t, http.StatusForbidden, status)
		assert.Equal(t, "org.not_member", message)

		// --- PRD D49: one member's or one key's row of the report, drawn over time ------
		assert.Equal(t, map[string]int{"seller": 3 * cost}, spentBy(trend(owner, "group_by=member&bucket=day&user_id="+id(seller.userID))),
			"the seller's row, wherever they sat")
		assert.Equal(t, map[string]int{"Sales tools": 3 * cost}, spentBy(trend(owner, "group_by=key&bucket=day&token_id="+id(sellersKey.Id))))
		// The manager of Sales is drawn the part spent in Sales, and nothing of
		// a member who was never there.
		assert.Equal(t, map[string]int{"seller": 2 * cost}, spentBy(trend(manager, "group_by=member&bucket=day&user_id="+id(seller.userID))))
		assert.Empty(t, trend(manager, "group_by=member&bucket=day&user_id="+id(maker.userID)).Series)
		// The report takes the same narrowing.
		assert.Equal(t, map[string]int{"gpt-4o-mini": 3}, report(owner, "group_by=model&user_id="+id(seller.userID)).requestsBy())
		assert.Equal(t, map[string]int{"seller": 2}, report(manager, "group_by=member&token_id="+id(sellersKey.Id)).requestsBy())

		for _, query := range []string{"", "group_by=model&bucket=day", "group_by=model&timezone=UTC", "group_by=team&bucket=day&timezone=UTC",
			"group_by=model&bucket=hour&timezone=UTC", "group_by=model&bucket=day&timezone=Nowhere%2FAt_All",
			"group_by=model&bucket=day&timezone=UTC&department_id=sales", "group_by=member&bucket=day&timezone=UTC&user_id=me",
			"group_by=key&bucket=day&timezone=UTC&token_id=-1", "group_by=model&bucket=day&timezone=UTC&start_timestamp=200&end_timestamp=100"} {
			status, success, message, _ := owner.call(t, http.MethodGet, "/api/org/usage/trend?"+query, nil)
			assert.Equal(t, http.StatusOK, status, query)
			assert.False(t, success, query)
			assert.Equal(t, "common.invalid_params", message, query)
		}

		// --- the audit log, narrowed over the real route --------------------------------
		type auditPage struct {
			Total int `json:"total"`
			Items []struct {
				Actor      string `json:"actor"`
				Action     string `json:"action"`
				TargetType string `json:"target_type"`
			} `json:"items"`
		}
		audit := func(query string) auditPage {
			t.Helper()
			var page auditPage
			auditor.ok(t, http.MethodGet, "/api/org/audit-logs?page_size=100&"+query, nil, &page)
			return page
		}
		keys := audit("target_type=key")
		assert.Equal(t, 3, keys.Total)
		for _, item := range keys.Items {
			assert.Equal(t, orgmodel.AuditKeyCreate, item.Action)
			assert.Equal(t, "founder", item.Actor)
		}
		joined := audit("actor=SELL")
		require.Equal(t, 1, joined.Total, "the seller's only record is having joined")
		assert.Equal(t, orgmodel.AuditMemberJoin, joined.Items[0].Action)
		assert.Equal(t, 2, audit("actor=found&target_type=member&start_timestamp="+id(int(now-60))).Total,
			"the founder added a service account and moved the seller")
		assert.Zero(t, audit("start_timestamp="+id(int(now+60))).Total)
		for _, query := range []string{"end_timestamp=soon", "start_timestamp=-1"} {
			status, success, message, _ := auditor.call(t, http.MethodGet, "/api/org/audit-logs?"+query, nil)
			assert.Equal(t, http.StatusOK, status, query)
			assert.False(t, success, query)
			assert.Equal(t, "common.invalid_params", message, query)
		}
	})
}
