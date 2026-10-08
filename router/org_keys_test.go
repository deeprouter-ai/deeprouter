package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// raw sends a request as the browser and returns the whole response, for the
// answers that are not the usual JSON envelope or whose headers matter.
func (b browser) raw(t *testing.T, method string, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	reader := bytes.NewReader(nil)
	if body != nil {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if b.cookie != "" {
		request.Header.Set("Cookie", b.cookie)
	}
	if b.userID != 0 {
		request.Header.Set("New-Api-User", strconv.Itoa(b.userID))
	}
	recorder := httptest.NewRecorder()
	b.engine.ServeHTTP(recorder, request)
	return recorder
}

// TestOrgKeys_ThroughTheRealRouter walks Enterprise Org P5 (meta-repo
// docs/enterprise-org-prd.md §3) end to end over the real routes, the real
// auth middleware and browser-style session cookies: keys made for a person
// and for a service account, a holder who can look at their key and do nothing
// to it but install it, a manager and a readonly member who see and do not
// touch, rotation, freezing, deletion, an audit log that saw all of it and
// holds no key — and a key limited to a hand-picked list of models.
func TestOrgKeys_ThroughTheRealRouter(t *testing.T) {
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		engine := newOrgEngine(t, db)
		SetConnectRouter(engine)
		id := strconv.Itoa
		// valueOf reads what the database holds for a key: the one thing no
		// endpoint below may ever say, except where the test expects it.
		valueOf := func(keyID int) string {
			var stored model.Token
			require.NoError(t, db.Unscoped().First(&stored, keyID).Error)
			return stored.Key
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
		join := func(username string, roleName string, departmentName string) browser {
			var invite struct {
				Code string `json:"code"`
			}
			owner.ok(t, http.MethodPost, "/api/org/invites", map[string]any{"role_id": roleID[roleName], "department_id": departmentID[departmentName]}, &invite)
			return signUp(t, engine, map[string]any{"username": username, "password": "password123", "org_invite": invite.Code}, "")
		}
		manager := join("manager", orgmodel.RoleManager, "Sales")
		readonly := join("readonly", orgmodel.RoleReadonly, "General")
		seller := join("seller", orgmodel.RoleStaff, "Sales")
		builder := join("builder", orgmodel.RoleStaff, "Product")
		var bot struct {
			Id int `json:"id"`
		}
		owner.ok(t, http.MethodPost, "/api/org/service-accounts", map[string]any{"name": "CI", "department_id": departmentID["Product"]}, &bot)

		type key struct {
			Id              int      `json:"id"`
			Name            string   `json:"name"`
			Key             string   `json:"key"`
			Value           string   `json:"value"`
			Status          int      `json:"status"`
			HolderId        int      `json:"holder_id"`
			Holder          string   `json:"holder"`
			HolderIsService bool     `json:"holder_is_service"`
			DepartmentId    int      `json:"department_id"`
			PolicyTemplate  string   `json:"policy_template"`
			ModelLimits     []string `json:"model_limits"`
			RemainQuota     int      `json:"remain_quota"`
			RpmLimit        int      `json:"rpm_limit"`
		}

		// --- Creating: a person's key says nothing, a service account's once ----
		var sellersKey, botsKey key
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Seller's", "holder_id": seller.userID, "remain_quota": 5000, "rpm_limit": 60}, &sellersKey)
		require.Empty(t, sellersKey.Value)
		require.Equal(t, model.MaskTokenKey(valueOf(sellersKey.Id)), sellersKey.Key)
		require.Equal(t, []any{seller.userID, "seller", false, departmentID["Sales"], 5000, 60},
			[]any{sellersKey.HolderId, sellersKey.Holder, sellersKey.HolderIsService, sellersKey.DepartmentId, sellersKey.RemainQuota, sellersKey.RpmLimit})

		created := owner.raw(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Pipeline", "holder_id": bot.Id, "unlimited_quota": true})
		require.Equal(t, http.StatusOK, created.Code)
		require.Contains(t, created.Header().Get("Cache-Control"), "no-store", "an answer that carries a key is not to be cached")
		var envelope struct {
			Success bool `json:"success"`
			Data    key  `json:"data"`
		}
		require.NoError(t, common.Unmarshal(created.Body.Bytes(), &envelope))
		require.True(t, envelope.Success)
		botsKey = envelope.Data
		require.Len(t, botsKey.Value, 48)
		require.Equal(t, valueOf(botsKey.Id), botsKey.Value)
		require.True(t, botsKey.HolderIsService)

		// --- The holder: may look, may not touch, receives it by one-click setup -
		var mine struct {
			Items []struct {
				Id    int    `json:"id"`
				Key   string `json:"key"`
				OrgId int    `json:"org_id"`
			} `json:"items"`
		}
		seller.ok(t, http.MethodGet, "/api/token/?p=1&size=10", nil, &mine)
		require.Len(t, mine.Items, 1)
		require.Equal(t, sellersKey.Id, mine.Items[0].Id)
		require.Equal(t, model.MaskTokenKey(valueOf(sellersKey.Id)), mine.Items[0].Key)
		require.NotZero(t, mine.Items[0].OrgId)

		keyPath := "/api/token/" + id(sellersKey.Id)
		only := map[string]any{"ids": []int{sellersKey.Id}}
		for _, attempt := range []struct {
			method string
			path   string
			body   any
		}{
			{http.MethodPost, keyPath + "/key", nil},
			{http.MethodPost, "/api/token/batch/keys", only},
			{http.MethodPut, "/api/token/", map[string]any{"id": sellersKey.Id, "name": "mine now", "unlimited_quota": true, "expired_time": -1}},
			{http.MethodPut, "/api/token/?status_only=true", map[string]any{"id": sellersKey.Id, "status": common.TokenStatusDisabled}},
			{http.MethodDelete, keyPath, nil},
			{http.MethodPost, "/api/token/batch", only},
		} {
			answer := seller.raw(t, attempt.method, attempt.path, attempt.body)
			require.Equal(t, http.StatusForbidden, answer.Code, "%s %s", attempt.method, attempt.path)
			require.Contains(t, answer.Body.String(), "org.key_managed_by_org", "%s %s", attempt.method, attempt.path)
			require.NotContains(t, answer.Body.String(), valueOf(sellersKey.Id), "%s %s", attempt.method, attempt.path)
		}
		orgKeyPath := "/api/org/keys/" + id(sellersKey.Id)
		seller.refused(t, http.MethodGet, "/api/org/keys", nil)
		seller.refused(t, http.MethodPost, orgKeyPath+"/rotate", nil)
		seller.refused(t, http.MethodPut, orgKeyPath, map[string]any{"unlimited_quota": true})

		// Acceptance: 成员的 key 只能由持有人本人经一键配置下发到自己的工具.
		install := func(holder browser, keyID int) string {
			var issued struct {
				ScriptPath string `json:"script_path"`
			}
			holder.ok(t, http.MethodPost, "/api/connect/token", map[string]any{"key_id": keyID, "tools": []string{"codex"}}, &issued)
			script := browser{engine: engine}.raw(t, http.MethodGet, issued.ScriptPath, nil)
			require.Equal(t, http.StatusOK, script.Code)
			return script.Body.String()
		}
		require.Contains(t, install(seller, sellersKey.Id), valueOf(sellersKey.Id))
		// Nobody else can ask for it — not the manager who sees the key on the
		// organization's page, and not the owner.
		for _, other := range []browser{manager, owner, builder} {
			answer := other.raw(t, http.MethodPost, "/api/connect/token", map[string]any{"key_id": sellersKey.Id, "tools": []string{"codex"}})
			require.Equal(t, http.StatusNotFound, answer.Code)
			require.NotContains(t, answer.Body.String(), valueOf(sellersKey.Id))
		}

		// --- A manager sees the keys held in their department, and changes none --
		var listed []key
		manager.ok(t, http.MethodGet, "/api/org/keys", nil, &listed)
		require.Len(t, listed, 1, "the pipeline's key is held in Product")
		require.Equal(t, sellersKey.Id, listed[0].Id)
		manager.ok(t, http.MethodGet, "/api/org/key-templates", nil, nil)
		manager.refused(t, http.MethodGet, "/api/org/key-holders", nil)
		manager.refused(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "K", "holder_id": seller.userID})

		// --- Readonly sees every key, and changes none either ---------------------
		readonly.ok(t, http.MethodGet, "/api/org/keys", nil, &listed)
		require.Len(t, listed, 2)
		require.Equal(t, []int{botsKey.Id, sellersKey.Id}, []int{listed[0].Id, listed[1].Id}, "newest first")
		for _, reader := range []browser{manager, readonly} {
			reader.refused(t, http.MethodPut, orgKeyPath, map[string]any{"name": "Mine"})
			reader.refused(t, http.MethodPost, orgKeyPath+"/rotate", nil)
			reader.refused(t, http.MethodPost, orgKeyPath+"/freeze", nil)
			reader.refused(t, http.MethodPost, orgKeyPath+"/unfreeze", nil)
			reader.refused(t, http.MethodDelete, orgKeyPath, nil)
		}
		builder.refused(t, http.MethodGet, "/api/org/keys", nil)

		// --- Rotation: a new value, the same key; its holder installs it again ---
		oldValue := valueOf(sellersKey.Id)
		var rotated key
		owner.ok(t, http.MethodPost, orgKeyPath+"/rotate", nil, &rotated)
		require.Empty(t, rotated.Value, "a person's new value is not shown to whoever rotated it")
		newValue := valueOf(sellersKey.Id)
		require.NotEqual(t, oldValue, newValue)
		require.Equal(t, []any{sellersKey.Id, seller.userID, 5000, 60}, []any{rotated.Id, rotated.HolderId, rotated.RemainQuota, rotated.RpmLimit})
		script := install(seller, sellersKey.Id)
		require.Contains(t, script, newValue)
		require.NotContains(t, script, oldValue)

		replaced := owner.raw(t, http.MethodPost, "/api/org/keys/"+id(botsKey.Id)+"/rotate", nil)
		require.Equal(t, http.StatusOK, replaced.Code)
		require.Contains(t, replaced.Header().Get("Cache-Control"), "no-store", "nor is the answer that carries its new value")
		require.NoError(t, common.Unmarshal(replaced.Body.Bytes(), &envelope))
		require.True(t, envelope.Success)
		rotated = envelope.Data
		require.Len(t, rotated.Value, 48)
		require.NotEqual(t, botsKey.Value, rotated.Value)
		require.Equal(t, valueOf(botsKey.Id), rotated.Value)

		// --- Freezing holds against the holder; changing; deleting -----------------
		owner.ok(t, http.MethodPost, orgKeyPath+"/freeze", nil, nil)
		thaw := seller.raw(t, http.MethodPut, "/api/token/?status_only=true", map[string]any{"id": sellersKey.Id, "status": common.TokenStatusEnabled})
		require.Equal(t, http.StatusForbidden, thaw.Code)
		readonly.ok(t, http.MethodGet, "/api/org/keys", nil, &listed)
		require.Equal(t, common.TokenStatusDisabled, listed[1].Status)
		owner.ok(t, http.MethodPost, orgKeyPath+"/unfreeze", nil, nil)

		var changed key
		owner.ok(t, http.MethodPut, orgKeyPath, map[string]any{"remain_quota": 9000}, &changed)
		require.Equal(t, []any{9000, 60, common.TokenStatusEnabled}, []any{changed.RemainQuota, changed.RpmLimit, changed.Status})

		owner.ok(t, http.MethodDelete, orgKeyPath, nil, nil)
		readonly.ok(t, http.MethodGet, "/api/org/keys", nil, &listed)
		require.Len(t, listed, 1)
		seller.ok(t, http.MethodGet, "/api/token/?p=1&size=10", nil, &mine)
		require.Empty(t, mine.Items)

		// --- The audit log saw all of it, and holds no key --------------------------
		var audit struct {
			Items []struct {
				ActorUserId int             `json:"actor_user_id"`
				Action      string          `json:"action"`
				TargetType  string          `json:"target_type"`
				TargetId    int             `json:"target_id"`
				Detail      json.RawMessage `json:"detail"`
				Ip          string          `json:"ip"`
			} `json:"items"`
		}
		readonly.ok(t, http.MethodGet, "/api/org/audit-logs?page_size=100", nil, &audit)
		counts := map[string]int{}
		for _, item := range audit.Items {
			counts[item.Action]++
			require.Equal(t, "192.0.2.1", item.Ip, item.Action)
			for _, value := range []string{oldValue, newValue, botsKey.Value, valueOf(botsKey.Id)} {
				require.NotContains(t, string(item.Detail), value, "%s record carries a key", item.Action)
			}
			if item.Action == orgmodel.AuditKeyDeliver {
				require.Equal(t, seller.userID, item.ActorUserId, "a delivery is the holder's own act")
				require.Equal(t, []any{orgmodel.AuditTargetKey, sellersKey.Id}, []any{item.TargetType, item.TargetId})
			}
		}
		require.Equal(t, map[string]int{
			orgmodel.AuditMemberInvite:         4,
			orgmodel.AuditMemberJoin:           4,
			orgmodel.AuditServiceAccountCreate: 1,
			orgmodel.AuditKeyCreate:            2,
			orgmodel.AuditKeyDeliver:           2, // before the rotation, and after it
			orgmodel.AuditKeyRotate:            2,
			orgmodel.AuditKeyFreeze:            1,
			orgmodel.AuditKeyUnfreeze:          1,
			orgmodel.AuditKeyUpdate:            1,
			orgmodel.AuditKeyDelete:            1,
		}, counts, "every write that succeeded, and none of the refused ones")

		// --- A hand-picked list of models (PRD §5, D33) -------------------------------
		// Which models a holder can be served is read with settings only the
		// gateway's start-up makes, so the list itself is checked where that has
		// run (controller/org_keys_test.go). Here: the route, who may ask, and
		// that a list in a request body reaches the rules.
		modelsFor := "/api/org/key-models?holder_id=" + id(builder.userID)
		for _, reader := range []browser{manager, readonly, seller, builder} {
			reader.refused(t, http.MethodGet, modelsFor, nil)
		}
		status, success, message, _ := owner.call(t, http.MethodGet, "/api/org/key-models?holder_id=424242", nil)
		require.Equal(t, []any{http.StatusOK, false, "org.member_not_found"}, []any{status, success, message})

		refusal := owner.raw(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "K", "policy_template": "coding", "model_limits": []string{"model-a"}})
		require.Equal(t, http.StatusOK, refusal.Code)
		require.Contains(t, refusal.Body.String(), "org.key_models_with_template")
		var open key
		owner.ok(t, http.MethodPost, "/api/org/keys", map[string]any{"name": "Open", "holder_id": builder.userID, "unlimited_quota": true, "model_limits": []string{}}, &open)
		require.Empty(t, open.ModelLimits, "an empty list is every model")
		require.Empty(t, open.PolicyTemplate)

		// --- No sign-in, no keys ------------------------------------------------------
		visitor := browser{engine: engine}
		for _, probe := range []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/org/keys"},
			{http.MethodPost, "/api/org/keys"},
			{http.MethodPut, orgKeyPath},
			{http.MethodDelete, orgKeyPath},
			{http.MethodPost, orgKeyPath + "/rotate"},
			{http.MethodPost, orgKeyPath + "/freeze"},
			{http.MethodPost, orgKeyPath + "/unfreeze"},
			{http.MethodGet, "/api/org/key-holders"},
			{http.MethodGet, "/api/org/key-templates"},
			{http.MethodGet, modelsFor},
		} {
			status, success, _, _ := visitor.call(t, probe.method, probe.path, nil)
			require.Equal(t, http.StatusUnauthorized, status, "%s %s", probe.method, probe.path)
			require.False(t, success, "%s %s", probe.method, probe.path)
		}
	})
}
