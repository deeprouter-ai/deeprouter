package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P5 (meta-repo docs/enterprise-org-prd.md §3): the keys of an
// organization. Who may do which of these things is pinned in gate_test.go;
// this file is about what each of them does.

// keysFixture is one organization with the people the key tests need.
type keysFixture struct {
	org     testOrg
	owner   *Actor
	general orgmodel.Department
	sales   orgmodel.Department
	product orgmodel.Department
	alice   platformmodel.User // staff, in Sales
	bob     platformmodel.User // staff, in Product
	bot     MemberView         // a service account, in Sales
}

// newKeysFixture founds an organization and puts a person in Sales, a person
// in Product and a service account in Sales.
func newKeysFixture(t *testing.T, db *gorm.DB, name string) keysFixture {
	t.Helper()
	org := seedOrg(t, db, name)
	f := keysFixture{
		org:     org,
		owner:   actorFor(t, db, org.owner.Id),
		general: defaultDepartment(t, db, org.id),
		sales:   departmentNamed(t, db, org.id, "Sales"),
		product: departmentNamed(t, db, org.id, "Product"),
	}
	staff := presetRoleID(t, db, orgmodel.RoleStaff)
	f.alice = seedMemberIn(t, db, org, "alice-of-"+name, staff, f.sales.Id)
	f.bob = seedMemberIn(t, db, org, "bob-of-"+name, staff, f.product.Id)
	bot, err := CreateServiceAccount(db, f.owner, "CI", f.sales.Id)
	require.NoError(t, err)
	f.bot = *bot
	return f
}

// memberWith adds a member who holds a custom role made of the given
// primitives, and returns them ready to act.
func (f keysFixture) memberWith(t *testing.T, db *gorm.DB, username string, scope string, departmentID int, permissions ...string) *Actor {
	t.Helper()
	role := seedRole(t, db, f.org.id, "Role of "+username, scope, permissions...)
	member := seedMemberIn(t, db, f.org, username, role.Id, departmentID)
	return actorFor(t, db, member.Id)
}

// everyKeyWrite lists the primitives that change keys.
var everyKeyWrite = []string{"key.create", "key.update", "key.assign", "key.rotate", "key.freeze", "key.delete"}

// keyRows counts the live keys of an organization.
func keyRows(t *testing.T, db *gorm.DB, orgID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&platformmodel.Token{}).Where("org_id = ?", orgID).Count(&n).Error)
	return n
}

// keysWithValue counts the keys, live or deleted, that answer to a value.
func keysWithValue(t *testing.T, db *gorm.DB, value string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Unscoped().Model(&platformmodel.Token{}).Where(&platformmodel.Token{Key: value}).Count(&n).Error)
	return n
}

// requireNoKeyValueInAudit fails if any audit record of the organization
// contains one of the given key values.
func requireNoKeyValueInAudit(t *testing.T, db *gorm.DB, orgID int, values ...string) {
	t.Helper()
	for _, record := range auditRecords(t, db, orgID) {
		for _, value := range values {
			require.NotEmpty(t, value)
			require.NotContains(t, record.Detail, value, "%s record #%d carries a key value", record.Action, record.Id)
		}
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func int64Ptr(n int64) *int64 { return &n }

// watchKeyCache puts a recorder in place of the door to the gateway's token
// cache for one test — a cache that is there when cached is true, as it is in
// production — and makes the second sweep come quickly. It returns how often a
// key value has been dropped so far.
func watchKeyCache(t *testing.T, cached bool) func(value string) int {
	t.Helper()
	var mu sync.Mutex
	dropped := map[string]int{}
	previousDrop, previousDelay := dropCachedKey, keyCacheSweepDelay
	dropCachedKey = func(value string) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		dropped[value]++
		return cached, nil
	}
	keyCacheSweepDelay = 20 * time.Millisecond
	t.Cleanup(func() { dropCachedKey, keyCacheSweepDelay = previousDrop, previousDelay })
	return func(value string) int {
		mu.Lock()
		defer mu.Unlock()
		return dropped[value]
	}
}

// askedCatalogue is a catalogue that remembers which holders it was asked to
// name the servable models of, and that fails at it when told to.
type askedCatalogue struct {
	fixedCatalogue
	asked *[]int
	err   error
}

// ServableModels notes who was asked about before answering.
func (c askedCatalogue) ServableModels(holderID int) ([]string, error) {
	*c.asked = append(*c.asked, holderID)
	if c.err != nil {
		return nil, c.err
	}
	return c.fixedCatalogue.ServableModels(holderID)
}

// modelList is the hand-picked list of a KeyPatch; none at all is "every model".
func modelList(names ...string) *[]string {
	if names == nil {
		names = []string{}
	}
	return &names
}

// keyChange reads the two sides of a key.update record.
func keyChange(t *testing.T, record orgmodel.OrgAuditLog) (before keyRecord, after keyRecord) {
	t.Helper()
	require.Equal(t, orgmodel.AuditKeyUpdate, record.Action)
	var detail struct {
		Before keyRecord `json:"before"`
		After  keyRecord `json:"after"`
	}
	require.NoError(t, common.UnmarshalJsonStr(record.Detail, &detail))
	return detail.Before, detail.After
}

// PRD §5: 创意生成包 = 图片 + 视频 + 对话；coding 包 = 编程。模板名在服务端校验，
// 不认识的直接拒绝；解析结果为空时拒绝建 key，绝不能变成不限制。
func TestResolveTemplate(t *testing.T) {
	models := fixedModels(testCatalogue).PurposeModels

	limits, err := resolveTemplate("", 7, models)
	require.NoError(t, err)
	require.Empty(t, limits, "no template means every model")

	// The union of the template's purposes, each entry once, in one order.
	limits, err = resolveTemplate("creative", 7, models)
	require.NoError(t, err)
	require.Equal(t, []string{"MiniMax-H3", "claude-*", "dall-e-3", "deeprouter-auto", "gpt-4o*"}, limits)
	limits, err = resolveTemplate("coding", 7, models)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-sonnet-*", "deeprouter-auto", "o1*"}, limits)

	// A name the platform does not offer is refused, never read as "no template".
	for _, unknown := range []string{"everything", "Creative", " creative", "chat"} {
		_, err = resolveTemplate(unknown, 7, models)
		require.ErrorIs(t, err, ErrKeyTemplateUnknown, unknown)
	}

	// Nothing to allow is refused too.
	_, err = resolveTemplate("creative", 7, fixedModels(nil).PurposeModels)
	require.ErrorIs(t, err, ErrKeyTemplateEmpty)
	_, err = resolveTemplate("coding", 7, fixedModels(map[string][]string{"coding": {"", "  "}}).PurposeModels)
	require.ErrorIs(t, err, ErrKeyTemplateEmpty)
	// One purpose with nothing leaves the others: the key is narrower, not wider.
	limits, err = resolveTemplate("creative", 7, fixedModels(map[string][]string{"chat": {"claude-*"}}).PurposeModels)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-*"}, limits)

	// Each purpose is asked for the holder of the key, and a failure to
	// resolve one stops the whole thing.
	var asked []string
	_, err = resolveTemplate("creative", 42, func(holderID int, purpose string) ([]string, error) {
		asked = append(asked, fmt.Sprintf("%d:%s", holderID, purpose))
		return []string{"m"}, nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"42:image", "42:video", "42:chat"}, asked)
	down := errors.New("the catalogue is down")
	_, err = resolveTemplate("coding", 7, func(int, string) ([]string, error) { return nil, down })
	require.ErrorIs(t, err, down)
}

// PRD §5 (D33): 不套模板 = 不限制模型，或手动指定。手动指定的每个模型都必须是
// 持有人当前用得了的；模板和手选清单只能二选一。
func TestResolveAllowance(t *testing.T) {
	var asked []int
	models := askedCatalogue{fixedCatalogue: fixedModels(testCatalogue), asked: &asked}
	down := errors.New("the catalogue is down")
	broken := askedCatalogue{fixedCatalogue: fixedModels(testCatalogue), asked: &asked, err: down}

	// Neither a template nor a list: every model, and nothing to look up.
	limits, err := resolveAllowance("", nil, nil, 7, broken)
	require.NoError(t, err)
	require.Empty(t, limits)
	require.NotNil(t, limits, "an empty list, not null")
	limits, err = resolveAllowance("", []string{"", "  "}, nil, 7, broken)
	require.NoError(t, err)
	require.Empty(t, limits, "blanks are not a list")

	// A template is resolved as it always was; which models there are to pick
	// from by hand is not asked.
	limits, err = resolveAllowance("coding", nil, nil, 7, broken)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-sonnet-*", "deeprouter-auto", "o1*"}, limits)
	limits, err = resolveAllowance("coding", []string{" ", ""}, nil, 7, broken)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-sonnet-*", "deeprouter-auto", "o1*"}, limits, "blanks beside a template are no list either")
	_, err = resolveAllowance("everything", nil, nil, 7, models)
	require.ErrorIs(t, err, ErrKeyTemplateUnknown)
	_, err = resolveAllowance("creative", nil, nil, 7, fixedModels(nil))
	require.ErrorIs(t, err, ErrKeyTemplateEmpty)
	require.Empty(t, asked)

	// A hand-picked list: each name once, trimmed, in one order — checked
	// against what the holder of the key can be served.
	limits, err = resolveAllowance("", []string{" gpt-4o", "claude-sonnet-5", "gpt-4o ", ""}, nil, 42, models)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-sonnet-5", "gpt-4o"}, limits)
	require.Equal(t, []int{42}, asked)

	// A template and a list at once is refused, whatever the template.
	for _, template := range []string{"creative", "coding", "everything"} {
		_, err = resolveAllowance(template, []string{"gpt-4o"}, nil, 7, models)
		require.ErrorIs(t, err, ErrKeyModelsWithTemplate, template)
	}

	// A name the holder cannot be served is refused, and the refusal says which
	// — every one of them, and none of the others.
	_, err = resolveAllowance("", []string{"gpt-4o", "gpt-5-typo", "claude-sonnet-5", "retired-model"}, nil, 7, models)
	require.ErrorIs(t, err, ErrKeyModelsUnavailable)
	var unavailable *KeyModelsUnavailableError
	require.ErrorAs(t, err, &unavailable)
	require.Equal(t, []string{"gpt-5-typo", "retired-model"}, unavailable.Models)
	require.Contains(t, err.Error(), "gpt-5-typo, retired-model")
	// A rule is not a model: a hand-picked list cannot say "everything", nor
	// reach a model by a prefix or by another spelling.
	for _, rule := range []string{"*", "claude-*", "gpt-4o*", "GPT-4O", "gpt-4o,dall-e-3"} {
		_, err = resolveAllowance("", []string{rule}, nil, 7, models)
		require.ErrorIs(t, err, ErrKeyModelsUnavailable, rule)
	}
	// Where nobody can be served anything, no name passes.
	_, err = resolveAllowance("", []string{"gpt-4o"}, nil, 7, fixedCatalogue{})
	require.ErrorIs(t, err, ErrKeyModelsUnavailable)

	// What the key is limited to already may stay, whatever the catalogue says
	// today — a rule a personal purpose left on it, a model retired since.
	kept := []string{"claude-*", "retired-model", "gpt-4o"}
	limits, err = resolveAllowance("", []string{"retired-model", "claude-*", "dall-e-3"}, kept, 7, models)
	require.NoError(t, err)
	require.Equal(t, []string{"claude-*", "dall-e-3", "retired-model"}, limits)
	_, err = resolveAllowance("", []string{"claude-*", "o1*", "retired-model"}, kept, 7, models)
	require.ErrorAs(t, err, &unavailable)
	require.Equal(t, []string{"o1*"}, unavailable.Models, "a change is refused for what it adds, and for that only")

	// Taking a model out adds nothing, so it works even while the catalogue
	// cannot be read; adding one needs the catalogue, and its failure stops it.
	asked = nil
	limits, err = resolveAllowance("", []string{"gpt-4o"}, kept, 7, broken)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-4o"}, limits)
	require.Empty(t, asked)
	_, err = resolveAllowance("", []string{"gpt-4o", "dall-e-3"}, kept, 7, broken)
	require.ErrorIs(t, err, down)
}

// Acceptance: 创建 key 时可：选策略模板（一键填充模型白名单）、设额度/RPM/TPM/月限额
// （复用现有 token 配置能力）.
func TestCreateKey_StoresTheTemplateTheQuotaAndTheLimits(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		expiry := time.Now().Add(30 * 24 * time.Hour).Unix()

		grant, err := CreateKey(db, f.owner, KeyInput{
			Name:           "  Design team  ",
			HolderId:       f.alice.Id,
			PolicyTemplate: "creative",
			RemainQuota:    5000,
			ExpiredTime:    expiry,
			RpmLimit:       60,
			TpmLimit:       90000,
			MonthlyLimit:   3000,
		}, fixedModels(testCatalogue))
		require.NoError(t, err)

		stored := reloadKey(t, db, grant.Id)
		require.Equal(t, "Design team", stored.Name)
		require.Equal(t, f.alice.Id, stored.UserId, "the holder")
		require.Equal(t, f.org.id, stored.OrgId)
		require.Equal(t, f.owner.UserId, stored.CreatedBy)
		require.Equal(t, common.TokenStatusEnabled, stored.Status)
		require.Equal(t, "creative", stored.PolicyTemplate)
		require.True(t, stored.ModelLimitsEnabled)
		require.Equal(t, "MiniMax-H3,claude-*,dall-e-3,deeprouter-auto,gpt-4o*", stored.ModelLimits)
		require.Equal(t, 5000, stored.RemainQuota)
		require.False(t, stored.UnlimitedQuota)
		require.Equal(t, expiry, stored.ExpiredTime)
		require.Equal(t, []int{60, 90000, 3000}, []int{stored.RpmLimit, stored.TpmLimit, stored.MonthlyLimit})
		require.Len(t, stored.Key, 48)
		// The purpose fields of a personal key stay empty: they hold one
		// purpose only, and a personal save rewrites the whitelist from them.
		require.Empty(t, stored.SimplePurpose)

		require.Equal(t, KeyView{
			Id:             stored.Id,
			Name:           "Design team",
			Key:            platformmodel.MaskTokenKey(stored.Key),
			Status:         common.TokenStatusEnabled,
			HolderId:       f.alice.Id,
			Holder:         "alice-of-Acme",
			DepartmentId:   f.sales.Id,
			Department:     "Sales",
			PolicyTemplate: "creative",
			ModelLimits:    []string{"MiniMax-H3", "claude-*", "dall-e-3", "deeprouter-auto", "gpt-4o*"},
			RemainQuota:    5000,
			ExpiredTime:    expiry,
			RpmLimit:       60,
			TpmLimit:       90000,
			MonthlyLimit:   3000,
			CreatedTime:    stored.CreatedTime,
			AccessedTime:   stored.AccessedTime,
		}, grant.KeyView)

		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyCreate, record.Action)
		require.Equal(t, orgmodel.AuditTargetKey, record.TargetType)
		require.Equal(t, stored.Id, record.TargetId)
		require.Equal(t, f.owner.UserId, record.ActorUserId)
		require.Equal(t, testIP, record.Ip)
		require.JSONEq(t, fmt.Sprintf(`{"after":{"name":"Design team","holder_id":%d,"holder":"alice-of-Acme",
			"department_id":%d,"department":"Sales","policy_template":"creative",
			"model_limits":["MiniMax-H3","claude-*","dall-e-3","deeprouter-auto","gpt-4o*"],
			"remain_quota":5000,"unlimited_quota":false,"expired_time":%d,
			"rpm_limit":60,"tpm_limit":90000,"monthly_limit":3000}}`, f.alice.Id, f.sales.Id, expiry), record.Detail)
	})
}

// No holder parks the key under the owner; no template lets it call every
// model; no expiry means it never expires.
func TestCreateKey_DefaultsToTheOwnerEveryModelAndNoExpiry(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		itOps := f.memberWith(t, db, "it", orgmodel.ScopeOrg, f.general.Id, packPermissions(t, "it_ops")...)

		grant, err := CreateKey(db, itOps, KeyInput{Name: "Spare", UnlimitedQuota: true}, fixedModels(testCatalogue))
		require.NoError(t, err)

		stored := reloadKey(t, db, grant.Id)
		require.Equal(t, f.org.owner.Id, stored.UserId, "parked under the owner")
		require.Equal(t, itOps.UserId, stored.CreatedBy, "who made it is kept apart from who holds it")
		require.Empty(t, stored.PolicyTemplate)
		require.False(t, stored.ModelLimitsEnabled)
		require.Empty(t, stored.ModelLimits)
		require.True(t, stored.UnlimitedQuota)
		require.EqualValues(t, -1, stored.ExpiredTime)
		require.Equal(t, f.general.Id, grant.DepartmentId)
		require.Empty(t, grant.ModelLimits)
		require.NotNil(t, grant.ModelLimits, "an empty list, not null, for the page")
	})
}

// PRD §5 (D33): 不套模板时可以手动指定模型——这把 key 只能调用选中的那几个。
func TestCreateKey_TakesAHandPickedListOfModels(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		var asked []int
		models := askedCatalogue{fixedCatalogue: fixedModels(testCatalogue), asked: &asked}

		grant, err := CreateKey(db, f.owner, KeyInput{
			Name: "Writers", HolderId: f.alice.Id, ModelLimits: []string{"gpt-4o", " claude-sonnet-5 ", "gpt-4o"},
		}, models)
		require.NoError(t, err)

		stored := reloadKey(t, db, grant.Id)
		require.Empty(t, stored.PolicyTemplate, "a hand-picked list is not a template")
		require.True(t, stored.ModelLimitsEnabled)
		require.Equal(t, "claude-sonnet-5,gpt-4o", stored.ModelLimits)
		require.Equal(t, []string{"claude-sonnet-5", "gpt-4o"}, grant.ModelLimits)
		require.Empty(t, grant.PolicyTemplate)
		// Asked of the gateway's own matcher: those two, and nothing else.
		for name, allowed := range map[string]bool{"gpt-4o": true, "claude-sonnet-5": true, "claude-opus-4-8": false, "gpt-4o-mini": false, "dall-e-3": false} {
			require.Equal(t, allowed, platformmodel.MatchModelLimit(stored.GetModelLimitsMap(), name), name)
		}
		require.Equal(t, []int{f.alice.Id}, asked, "what can be picked is the holder's to be served, not the creator's")

		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyCreate, record.Action)
		require.JSONEq(t, fmt.Sprintf(`{"after":{"name":"Writers","holder_id":%d,"holder":"alice-of-Acme",
			"department_id":%d,"department":"Sales","policy_template":"",
			"model_limits":["claude-sonnet-5","gpt-4o"],
			"remain_quota":0,"unlimited_quota":false,"expired_time":-1,
			"rpm_limit":0,"tpm_limit":0,"monthly_limit":0}}`, f.alice.Id, f.sales.Id), record.Detail)

		// A parked key is checked against what the owner can be served.
		asked = nil
		_, err = CreateKey(db, f.owner, KeyInput{Name: "Spare", ModelLimits: []string{"dall-e-3"}}, models)
		require.NoError(t, err)
		require.Equal(t, []int{f.org.owner.Id}, asked)
	})
}

// PRD §5: 模板名在服务端校验 … 解析结果为空时拒绝建 key. And nothing half-made is
// left behind by any refusal.
func TestCreateKey_RefusesWhatItCannotMake(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		other := newKeysFixture(t, db, "Other")
		models := fixedModels(testCatalogue)
		past := time.Now().Add(-time.Hour).Unix()
		tooMuch := int(1000000000*common.QuotaPerUnit) + 1

		for name, tc := range map[string]struct {
			input  KeyInput
			models ModelCatalogue
			want   error
		}{
			"an unknown template":            {KeyInput{Name: "K", PolicyTemplate: "everything"}, models, ErrKeyTemplateUnknown},
			"a template with nothing in it":  {KeyInput{Name: "K", PolicyTemplate: "creative"}, fixedModels(nil), ErrKeyTemplateEmpty},
			"a template and a list at once":  {KeyInput{Name: "K", PolicyTemplate: "creative", ModelLimits: []string{"gpt-4o"}}, models, ErrKeyModelsWithTemplate},
			"a model nobody can be served":   {KeyInput{Name: "K", ModelLimits: []string{"gpt-4o", "gpt-5-typo"}}, models, ErrKeyModelsUnavailable},
			"a rule in place of a model":     {KeyInput{Name: "K", ModelLimits: []string{"*"}}, models, ErrKeyModelsUnavailable},
			"no name":                        {KeyInput{Name: "   "}, models, ErrInvalidKeyName},
			"a name of 51 characters":        {KeyInput{Name: strings.Repeat("钥", KeyNameMaxLength+1)}, models, ErrInvalidKeyName},
			"a negative quota":               {KeyInput{Name: "K", RemainQuota: -1}, models, ErrInvalidKeyQuota},
			"a quota beyond the platform's":  {KeyInput{Name: "K", RemainQuota: tooMuch}, models, ErrInvalidKeyQuota},
			"a negative requests limit":      {KeyInput{Name: "K", RpmLimit: -1}, models, ErrInvalidKeyLimit},
			"a negative tokens limit":        {KeyInput{Name: "K", TpmLimit: -1}, models, ErrInvalidKeyLimit},
			"a negative monthly limit":       {KeyInput{Name: "K", MonthlyLimit: -1}, models, ErrInvalidKeyLimit},
			"an expiry in the past":          {KeyInput{Name: "K", ExpiredTime: past}, models, ErrInvalidKeyExpiry},
			"a holder who does not exist":    {KeyInput{Name: "K", HolderId: 424242}, models, ErrMemberNotFound},
			"a holder of another company":    {KeyInput{Name: "K", HolderId: other.alice.Id}, models, ErrMemberNotFound},
			"another company's owner":        {KeyInput{Name: "K", HolderId: other.org.owner.Id}, models, ErrMemberNotFound},
			"another company's service acct": {KeyInput{Name: "K", HolderId: other.bot.Id}, models, ErrMemberNotFound},
		} {
			before := orgRowCounts(t, db, f.org.id)
			_, err := CreateKey(db, f.owner, tc.input, tc.models)
			require.ErrorIs(t, err, tc.want, name)
			require.Equal(t, before, orgRowCounts(t, db, f.org.id), name)
		}
		require.Zero(t, keyRows(t, db, f.org.id))
		require.Zero(t, keyRows(t, db, other.org.id))

		// The longest name that is allowed, in characters rather than bytes.
		longest := strings.Repeat("钥", KeyNameMaxLength)
		grant, err := CreateKey(db, f.owner, KeyInput{Name: longest}, models)
		require.NoError(t, err)
		require.Equal(t, longest, reloadKey(t, db, grant.Id).Name)
	})
}

// Acceptance: 组织 key 的明文不在任何界面显示 … 服务账号的 key 在归到服务账号名下时
// 向操作者展示一次.
func TestCreateKey_ShowsTheValueOnlyForAServiceAccount(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		models := fixedModels(testCatalogue)
		var values []string

		// A person's key — a colleague's, a parked one, even one the creator
		// makes for themselves — comes back without its value.
		for who, holderID := range map[string]int{"a colleague": f.alice.Id, "the owner": 0, "the creator": f.owner.UserId} {
			grant, err := CreateKey(db, f.owner, KeyInput{Name: "For " + who, HolderId: holderID}, models)
			require.NoError(t, err, who)
			stored := reloadKey(t, db, grant.Id)
			values = append(values, stored.Key)
			require.Empty(t, grant.Value, who)
			encoded, err := common.Marshal(grant)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), stored.Key, who)
			require.NotContains(t, string(encoded), `"value"`, who)
			require.NotContains(t, lastAudit(t, db, f.org.id).Detail, "value_shown", who)
		}

		// A service account cannot sign in to fetch its key, so whoever makes
		// it is shown the value — this once.
		grant, err := CreateKey(db, f.owner, KeyInput{Name: "For the pipeline", HolderId: f.bot.Id}, models)
		require.NoError(t, err)
		stored := reloadKey(t, db, grant.Id)
		values = append(values, stored.Key)
		require.Equal(t, stored.Key, grant.Value)
		require.Len(t, grant.Value, 48)
		require.Equal(t, platformmodel.MaskTokenKey(stored.Key), grant.Key, "the listed form stays masked")
		require.True(t, grant.HolderIsService)
		require.Contains(t, lastAudit(t, db, f.org.id).Detail, `"value_shown":true`, "the log says the value was seen")

		// Listing it afterwards never shows the value again.
		listed, err := ListKeys(db, f.owner)
		require.NoError(t, err)
		encoded, err := common.Marshal(listed)
		require.NoError(t, err)
		for _, value := range values {
			require.NotContains(t, string(encoded), value)
		}
		requireNoKeyValueInAudit(t, db, f.org.id, values...)
	})
}

// PRD D30: 建 key 时直接指定持有人，算"创建 + 分配"，key.create 和 key.assign 都要有。
// 只有 key.create 的角色只能把新 key 建到 owner 名下（暂挂）。
func TestCreateKey_ForSomeoneTakesTheRightToAssignToo(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		models := fixedModels(testCatalogue)
		create := func(actor *Actor, holderID int) error {
			_, err := CreateKey(db, actor, KeyInput{Name: "K", HolderId: holderID}, models)
			return err
		}

		// Creating only: the key can be made, and parked — handed to nobody.
		minter := f.memberWith(t, db, "minter", orgmodel.ScopeOrg, f.general.Id, "key.create")
		require.NoError(t, create(minter, 0))
		require.NoError(t, create(minter, f.org.owner.Id), "naming the owner is parking it")
		require.ErrorIs(t, create(minter, f.alice.Id), ErrForbidden)
		require.ErrorIs(t, create(minter, f.bot.Id), ErrForbidden, "a service account's key would show them its value")
		require.ErrorIs(t, create(minter, minter.UserId), ErrForbidden, "nor a key for themselves")

		// Assigning only — the preset manager — cannot make keys at all.
		manager := actorFor(t, db, seedMemberIn(t, db, f.org, "manager", presetRoleID(t, db, orgmodel.RoleManager), f.sales.Id).Id)
		for _, holderID := range []int{0, f.alice.Id, manager.UserId} {
			require.ErrorIs(t, create(manager, holderID), ErrForbidden)
		}

		// Both, within departments: for the people of those departments only.
		// The owner sits elsewhere, so this role cannot park a key either.
		desk := f.memberWith(t, db, "desk", orgmodel.ScopeDept, f.sales.Id, "key.create", "key.assign")
		require.NoError(t, create(desk, f.alice.Id))
		require.NoError(t, create(desk, f.bot.Id))
		require.NoError(t, create(desk, desk.UserId))
		require.ErrorIs(t, create(desk, f.bob.Id), ErrForbidden, "Product is not theirs")
		require.ErrorIs(t, create(desk, 0), ErrForbidden)
		require.ErrorIs(t, create(desk, f.org.owner.Id), ErrForbidden)

		// Both, across the organization — what IT Ops has: anyone.
		itOps := f.memberWith(t, db, "it", orgmodel.ScopeOrg, f.general.Id, packPermissions(t, "it_ops")...)
		for _, holderID := range []int{0, f.alice.Id, f.bob.Id, f.bot.Id, itOps.UserId} {
			require.NoError(t, create(itOps, holderID))
		}

		require.EqualValues(t, 2+3+5, keyRows(t, db, f.org.id), "the refusals created nothing")
	})
}

// One account holds only so many keys, an organization's as much as anyone's.
func TestCreateKey_StopsAtTheMostKeysOneAccountMayHold(t *testing.T) {
	setting := operation_setting.GetTokenSetting()
	previous := setting.MaxUserTokens
	setting.MaxUserTokens = 2
	t.Cleanup(func() { setting.MaxUserTokens = previous })

	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		models := fixedModels(testCatalogue)
		for i := 0; i < 2; i++ {
			_, err := CreateKey(db, f.owner, KeyInput{Name: "K", HolderId: f.alice.Id}, models)
			require.NoError(t, err)
		}
		_, err := CreateKey(db, f.owner, KeyInput{Name: "K", HolderId: f.alice.Id}, models)
		require.ErrorIs(t, err, ErrKeyLimitReached)
		_, err = CreateKey(db, f.owner, KeyInput{Name: "K", HolderId: f.bob.Id}, models)
		require.NoError(t, err, "the limit is each holder's own")
		require.EqualValues(t, 3, keyRows(t, db, f.org.id))
	})
}

// Each role is shown the keys within its reach, and never a value.
func TestListKeys_ShowsEachRoleTheKeysWithinItsReach(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		other := newKeysFixture(t, db, "Other")
		parked := seedKey(t, db, f.org, f.org.owner.Id, "parked")
		alices := seedKey(t, db, f.org, f.alice.Id, "alice's")
		bobs := seedKey(t, db, f.org, f.bob.Id, "bob's")
		bots := seedKey(t, db, f.org, f.bot.Id, "the pipeline's")
		// Three keys that must never show up: a personal one of a member,
		// another company's, and a deleted one.
		personal := platformmodel.Token{UserId: f.alice.Id, Name: "personal", Key: "personal-key-of-alice", Status: common.TokenStatusEnabled}
		require.NoError(t, db.Create(&personal).Error)
		theirs := seedKey(t, db, other.org, other.alice.Id, "theirs")
		deleted := seedKey(t, db, f.org, f.alice.Id, "deleted")
		require.NoError(t, db.Delete(&platformmodel.Token{}, deleted.Id).Error)

		ids := func(views []KeyView) []int {
			listed := []int{}
			for _, view := range views {
				listed = append(listed, view.Id)
			}
			return listed
		}

		// The whole organization, newest first.
		listed, err := ListKeys(db, f.owner)
		require.NoError(t, err)
		require.Equal(t, []int{bots.Id, bobs.Id, alices.Id, parked.Id}, ids(listed))
		require.Equal(t, KeyView{
			Id:              bots.Id,
			Name:            "the pipeline's",
			Key:             platformmodel.MaskTokenKey(bots.Key),
			Status:          common.TokenStatusEnabled,
			HolderId:        f.bot.Id,
			Holder:          "CI",
			HolderIsService: true,
			DepartmentId:    f.sales.Id,
			Department:      "Sales",
			ModelLimits:     []string{},
			RemainQuota:     750,
			UsedQuota:       250,
			ExpiredTime:     -1,
			CreatedTime:     1700000000,
			AccessedTime:    1700000000,
		}, listed[0])
		require.Equal(t, f.org.owner.Username, listed[3].Holder)
		require.Equal(t, f.general.Id, listed[3].DepartmentId)
		encoded, err := common.Marshal(listed)
		require.NoError(t, err)
		for _, key := range []platformmodel.Token{parked, alices, bobs, bots, theirs, deleted} {
			require.NotContains(t, string(encoded), key.Key, key.Name)
		}
		require.NotContains(t, string(encoded), personal.Key)

		readonly := actorFor(t, db, seedMember(t, db, f.org, "readonly", orgmodel.RoleReadonly).Id)
		listed, err = ListKeys(db, readonly)
		require.NoError(t, err)
		require.Len(t, listed, 4)

		// A manager: the keys held in the departments they manage.
		manager := actorFor(t, db, seedMemberIn(t, db, f.org, "manager", presetRoleID(t, db, orgmodel.RoleManager), f.sales.Id).Id)
		listed, err = ListKeys(db, manager)
		require.NoError(t, err)
		require.Equal(t, []int{bots.Id, alices.Id}, ids(listed))

		// Nobody else: staff see their own keys on their own keys page.
		finance := f.memberWith(t, db, "finance", orgmodel.ScopeOrg, f.general.Id, "usage.read")
		for _, actor := range []*Actor{actorFor(t, db, f.alice.Id), actorFor(t, db, f.bot.Id), finance} {
			_, err = ListKeys(db, actor)
			require.ErrorIs(t, err, ErrForbidden)
		}

		// A holder whose account is gone is still named: the key is still there.
		require.NoError(t, db.Delete(&platformmodel.User{}, f.alice.Id).Error)
		listed, err = ListKeys(db, f.owner)
		require.NoError(t, err)
		require.Equal(t, "alice-of-Acme", listed[2].Holder)
		require.Equal(t, f.sales.Id, listed[2].DepartmentId)
	})
}

// The create form offers the people a key can be made out to — which is what
// CreateKey would accept from the same actor.
func TestListKeyHolders_AreThePeopleAKeyCanBeMadeOutTo(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		newKeysFixture(t, db, "Other")
		minter := f.memberWith(t, db, "minter", orgmodel.ScopeOrg, f.general.Id, "key.create")
		desk := f.memberWith(t, db, "desk", orgmodel.ScopeDept, f.sales.Id, "key.create", "key.assign")
		clerk := f.memberWith(t, db, "clerk", orgmodel.ScopeDept, f.sales.Id, "key.create", "key.assign", "member.read")
		manager := actorFor(t, db, seedMemberIn(t, db, f.org, "manager", presetRoleID(t, db, orgmodel.RoleManager), f.sales.Id).Id)
		// roleID looks a role up by name: a preset, or one of Acme's own.
		roleID := func(name string) int {
			var role orgmodel.OrgRole
			require.NoError(t, db.Where("name = ? AND org_id IN ?", name, []int{0, f.org.id}).First(&role).Error)
			return role.Id
		}
		staff := roleID(orgmodel.RoleStaff)

		// Whoever may read the members is told each one's role, to find people by.
		holders, err := ListKeyHolders(db, f.owner)
		require.NoError(t, err)
		require.Equal(t, []KeyHolderView{
			{Id: f.org.owner.Id, Name: f.org.owner.Username, DepartmentId: f.general.Id, Department: "General", RoleId: roleID(orgmodel.RoleOwner), Role: "owner", IsOwner: true},
			{Id: f.alice.Id, Name: "alice-of-Acme", DepartmentId: f.sales.Id, Department: "Sales", RoleId: staff, Role: "staff"},
			{Id: f.bob.Id, Name: "bob-of-Acme", DepartmentId: f.product.Id, Department: "Product", RoleId: staff, Role: "staff"},
			{Id: f.bot.Id, Name: "CI", DepartmentId: f.sales.Id, Department: "Sales", RoleId: staff, Role: "staff", IsService: true},
			{Id: minter.UserId, Name: "minter", DepartmentId: f.general.Id, Department: "General", RoleId: roleID("Role of minter"), Role: "Role of minter"},
			{Id: desk.UserId, Name: "desk", DepartmentId: f.sales.Id, Department: "Sales", RoleId: roleID("Role of desk"), Role: "Role of desk"},
			{Id: clerk.UserId, Name: "clerk", DepartmentId: f.sales.Id, Department: "Sales", RoleId: roleID("Role of clerk"), Role: "Role of clerk"},
			{Id: manager.UserId, Name: "manager", DepartmentId: f.sales.Id, Department: "Sales", RoleId: roleID(orgmodel.RoleManager), Role: "manager"},
		}, holders)

		// Creating only: the owner, to park keys under. Without the right to read
		// members, nobody's role is told — not even that the owner is the owner's.
		holders, err = ListKeyHolders(db, minter)
		require.NoError(t, err)
		require.Equal(t, []KeyHolderView{{Id: f.org.owner.Id, Name: f.org.owner.Username, DepartmentId: f.general.Id, Department: "General", IsOwner: true}}, holders)

		// Creating and assigning within Sales: the people of Sales, by name and
		// department alone.
		holders, err = ListKeyHolders(db, desk)
		require.NoError(t, err)
		require.Equal(t, []KeyHolderView{
			{Id: f.alice.Id, Name: "alice-of-Acme", DepartmentId: f.sales.Id, Department: "Sales"},
			{Id: f.bot.Id, Name: "CI", DepartmentId: f.sales.Id, Department: "Sales", IsService: true},
			{Id: desk.UserId, Name: "desk", DepartmentId: f.sales.Id, Department: "Sales"},
			{Id: clerk.UserId, Name: "clerk", DepartmentId: f.sales.Id, Department: "Sales"},
			{Id: manager.UserId, Name: "manager", DepartmentId: f.sales.Id, Department: "Sales"},
		}, holders)

		// The same reach with the right to read members: the roles come along.
		holders, err = ListKeyHolders(db, clerk)
		require.NoError(t, err)
		require.Equal(t, []KeyHolderView{
			{Id: f.alice.Id, Name: "alice-of-Acme", DepartmentId: f.sales.Id, Department: "Sales", RoleId: staff, Role: "staff"},
			{Id: f.bot.Id, Name: "CI", DepartmentId: f.sales.Id, Department: "Sales", RoleId: staff, Role: "staff", IsService: true},
			{Id: desk.UserId, Name: "desk", DepartmentId: f.sales.Id, Department: "Sales", RoleId: roleID("Role of desk"), Role: "Role of desk"},
			{Id: clerk.UserId, Name: "clerk", DepartmentId: f.sales.Id, Department: "Sales", RoleId: roleID("Role of clerk"), Role: "Role of clerk"},
			{Id: manager.UserId, Name: "manager", DepartmentId: f.sales.Id, Department: "Sales", RoleId: roleID(orgmodel.RoleManager), Role: "manager"},
		}, holders)

		for _, actor := range []*Actor{manager, actorFor(t, db, f.alice.Id)} {
			_, err = ListKeyHolders(db, actor)
			require.ErrorIs(t, err, ErrForbidden, "whoever cannot create keys has nobody to create them for")
		}
	})
}

// The key form offers, for a hand-picked list, the models the key's holder can
// be served — to whoever may fill in that form for that member.
func TestListKeyModels_AreWhatTheHolderCanBeServed(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		other := newKeysFixture(t, db, "Other")
		var asked []int
		models := askedCatalogue{fixedCatalogue: fixedModels(testCatalogue), asked: &asked}

		// In name order, each once — asked about the member the key is for.
		listed, err := ListKeyModels(db, f.owner, f.alice.Id, models)
		require.NoError(t, err)
		require.Equal(t, []string{"MiniMax-H3", "claude-opus-4-8", "claude-sonnet-5", "dall-e-3", "gpt-4o"}, listed)
		require.Equal(t, []int{f.alice.Id}, asked)
		require.Equal(t, []string{"gpt-4o", "MiniMax-H3"}, testServable[:2], "the catalogue's own list is left as it was")

		// A member nobody serves anything gets an empty list, not null.
		listed, err = ListKeyModels(db, f.owner, f.alice.Id, fixedCatalogue{})
		require.NoError(t, err)
		require.NotNil(t, listed)
		require.Empty(t, listed)

		// Whoever fills in a key form: to create a key, or to change one —
		// across the organization, for anybody in it.
		minter := f.memberWith(t, db, "minter", orgmodel.ScopeOrg, f.general.Id, "key.create")
		editor := f.memberWith(t, db, "editor", orgmodel.ScopeOrg, f.general.Id, "key.update")
		for _, actor := range []*Actor{minter, editor} {
			for _, holderID := range []int{f.alice.Id, f.bob.Id, f.bot.Id, f.org.owner.Id} {
				_, err = ListKeyModels(db, actor, holderID, models)
				require.NoError(t, err)
			}
		}
		// Within departments: for the members of those departments only.
		desk := f.memberWith(t, db, "desk", orgmodel.ScopeDept, f.sales.Id, "key.update")
		for _, holderID := range []int{f.alice.Id, f.bot.Id, desk.UserId} {
			_, err = ListKeyModels(db, desk, holderID, models)
			require.NoError(t, err)
		}
		for _, holderID := range []int{f.bob.Id, f.org.owner.Id} {
			_, err = ListKeyModels(db, desk, holderID, models)
			require.ErrorIs(t, err, ErrForbidden)
		}

		// Nobody else — a manager and a read-only member see keys and fill in
		// no form — and they are told so whoever they ask about, before
		// anything is looked up.
		manager := actorFor(t, db, seedMemberIn(t, db, f.org, "manager", presetRoleID(t, db, orgmodel.RoleManager), f.sales.Id).Id)
		readonly := actorFor(t, db, seedMember(t, db, f.org, "readonly", orgmodel.RoleReadonly).Id)
		stopper := f.memberWith(t, db, "stopper", orgmodel.ScopeOrg, f.general.Id, "key.assign", "key.rotate", "key.freeze", "key.delete")
		asked = nil
		for _, actor := range []*Actor{manager, readonly, stopper, actorFor(t, db, f.alice.Id), actorFor(t, db, f.bot.Id)} {
			for _, holderID := range []int{f.alice.Id, actor.UserId, 424242, other.alice.Id} {
				_, err = ListKeyModels(db, actor, holderID, models)
				require.ErrorIs(t, err, ErrForbidden)
			}
		}
		require.Empty(t, asked)

		// A member of another company, and nobody at all, are not found.
		for _, holderID := range []int{other.alice.Id, other.org.owner.Id, 424242, 0} {
			_, err = ListKeyModels(db, f.owner, holderID, models)
			require.ErrorIs(t, err, ErrMemberNotFound)
		}
		require.Empty(t, asked)

		// When the catalogue cannot be read, that is the answer.
		down := errors.New("the catalogue is down")
		_, err = ListKeyModels(db, f.owner, f.alice.Id, askedCatalogue{fixedCatalogue: fixedModels(nil), asked: &asked, err: down})
		require.ErrorIs(t, err, down)
	})
}

// PRD D31: 修改 key 的额度、限流、模板、有效期. A field the patch does not name is
// left alone — the quota above all, which the key spends while the form is open.
func TestUpdateKey_ChangesOnlyWhatThePatchNames(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		models := fixedModels(testCatalogue)
		key := seedKey(t, db, f.org, f.alice.Id, "Old name")

		view, err := UpdateKey(db, f.owner, key.Id, KeyPatch{Name: strPtr("  New name ")}, models)
		require.NoError(t, err)
		require.Equal(t, "New name", view.Name)
		renamed := reloadKey(t, db, key.Id)
		expected := key
		expected.Name = "New name"
		require.Equal(t, expected, renamed, "nothing but the name")
		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyUpdate, record.Action)
		require.Equal(t, key.Id, record.TargetId)
		side := `{"name":"%s","holder_id":%d,"holder":"alice-of-Acme","department_id":%d,"department":"Sales","policy_template":"","model_limits":[],
			"remain_quota":750,"unlimited_quota":false,"expired_time":-1,"rpm_limit":0,"tpm_limit":0,"monthly_limit":0}`
		require.JSONEq(t, fmt.Sprintf(`{"before":`+side+`,"after":`+side+`}`,
			"Old name", f.alice.Id, f.sales.Id, "New name", f.alice.Id, f.sales.Id), record.Detail)

		// The key spends some quota while an administrator has the form open,
		// and then saves a change to something else: the quota is not reset.
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).
			Updates(map[string]any{"remain_quota": 700, "used_quota": 300}).Error)
		_, err = UpdateKey(db, f.owner, key.Id, KeyPatch{RpmLimit: intPtr(30)}, models)
		require.NoError(t, err)
		spent := reloadKey(t, db, key.Id)
		require.Equal(t, []int{700, 300, 30}, []int{spent.RemainQuota, spent.UsedQuota, spent.RpmLimit})

		// Everything else an administrator decides about a key.
		expiry := time.Now().Add(7 * 24 * time.Hour).Unix()
		view, err = UpdateKey(db, f.owner, key.Id, KeyPatch{
			RemainQuota:  intPtr(9000),
			ExpiredTime:  int64Ptr(expiry),
			TpmLimit:     intPtr(40000),
			MonthlyLimit: intPtr(2000),
		}, models)
		require.NoError(t, err)
		changed := reloadKey(t, db, key.Id)
		require.Equal(t, []int{9000, 300, 30, 40000, 2000}, []int{changed.RemainQuota, changed.UsedQuota, changed.RpmLimit, changed.TpmLimit, changed.MonthlyLimit})
		require.Equal(t, expiry, changed.ExpiredTime)
		require.Equal(t, 9000, view.RemainQuota)
		require.Equal(t, 300, view.UsedQuota)

		_, err = UpdateKey(db, f.owner, key.Id, KeyPatch{UnlimitedQuota: boolPtr(true), ExpiredTime: int64Ptr(0), RpmLimit: intPtr(0)}, models)
		require.NoError(t, err)
		changed = reloadKey(t, db, key.Id)
		require.True(t, changed.UnlimitedQuota)
		require.EqualValues(t, -1, changed.ExpiredTime, "zero means never")
		require.Zero(t, changed.RpmLimit, "zero lifts a limit")

		// Who holds it, whether it is frozen and what its value is are not
		// this action's to change.
		require.Equal(t, []any{key.UserId, key.Status, key.Key, key.OrgId, key.CreatedBy},
			[]any{changed.UserId, changed.Status, changed.Key, changed.OrgId, changed.CreatedBy})
		requireNoKeyValueInAudit(t, db, f.org.id, key.Key)
	})
}

// PRD §5: 图片、视频是从实时模型目录推导的精确名单，目录上新后要"重新套用模板"才生效.
func TestUpdateKey_AppliesATemplateAfresh(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		grant, err := CreateKey(db, f.owner, KeyInput{Name: "Design", HolderId: f.alice.Id, PolicyTemplate: "creative"}, fixedModels(testCatalogue))
		require.NoError(t, err)

		// The catalogue gains an image model. The key does not, until its
		// template is applied again.
		grown := map[string][]string{"chat": testCatalogue["chat"], "coding": testCatalogue["coding"], "video": testCatalogue["video"], "image": {"dall-e-3", "flux-1"}}
		_, err = UpdateKey(db, f.owner, grant.Id, KeyPatch{Name: strPtr("Design team")}, fixedModels(grown))
		require.NoError(t, err)
		require.NotContains(t, reloadKey(t, db, grant.Id).ModelLimits, "flux-1")

		view, err := UpdateKey(db, f.owner, grant.Id, KeyPatch{PolicyTemplate: strPtr("creative")}, fixedModels(grown))
		require.NoError(t, err)
		require.Equal(t, []string{"MiniMax-H3", "claude-*", "dall-e-3", "deeprouter-auto", "flux-1", "gpt-4o*"}, view.ModelLimits)
		require.Equal(t, "MiniMax-H3,claude-*,dall-e-3,deeprouter-auto,flux-1,gpt-4o*", reloadKey(t, db, grant.Id).ModelLimits)

		// Another template replaces the list; no template lifts it.
		_, err = UpdateKey(db, f.owner, grant.Id, KeyPatch{PolicyTemplate: strPtr("coding")}, fixedModels(grown))
		require.NoError(t, err)
		stored := reloadKey(t, db, grant.Id)
		require.Equal(t, []string{"coding", "claude-sonnet-*,deeprouter-auto,o1*"}, []string{stored.PolicyTemplate, stored.ModelLimits})
		require.True(t, stored.ModelLimitsEnabled)

		_, err = UpdateKey(db, f.owner, grant.Id, KeyPatch{PolicyTemplate: strPtr("")}, fixedModels(grown))
		require.NoError(t, err)
		stored = reloadKey(t, db, grant.Id)
		require.Equal(t, []string{"", ""}, []string{stored.PolicyTemplate, stored.ModelLimits})
		require.False(t, stored.ModelLimitsEnabled)

		// A template that cannot be applied changes nothing.
		for want, patch := range map[error]KeyPatch{
			ErrKeyTemplateUnknown: {Name: strPtr("Renamed"), PolicyTemplate: strPtr("everything")},
			ErrKeyTemplateEmpty:   {Name: strPtr("Renamed"), PolicyTemplate: strPtr("coding")},
		} {
			records := len(auditRecords(t, db, f.org.id))
			_, err = UpdateKey(db, f.owner, grant.Id, patch, fixedModels(nil))
			require.ErrorIs(t, err, want)
			require.Equal(t, stored, reloadKey(t, db, grant.Id))
			require.Len(t, auditRecords(t, db, f.org.id), records)
		}

		// A key that was limited some other way — a founder's first key
		// carries a personal purpose — keeps its limits when only something
		// else is changed.
		legacy := seedKey(t, db, f.org, f.org.owner.Id, "first key")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", legacy.Id).
			Updates(map[string]any{"model_limits_enabled": true, "model_limits": "claude-*"}).Error)
		view, err = UpdateKey(db, f.owner, legacy.Id, KeyPatch{Name: strPtr("Founder's key")}, fixedModels(grown))
		require.NoError(t, err)
		require.Equal(t, []string{"claude-*"}, view.ModelLimits)
		require.Equal(t, "claude-*", reloadKey(t, db, legacy.Id).ModelLimits)
	})
}

// PRD §5 (D33): what a key may call is one thing said in two fields — a policy
// template, or a hand-picked list — and a change replaces it as a whole.
func TestUpdateKey_ReplacesWhatTheKeyMayCall(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		models := fixedModels(testCatalogue)
		grant, err := CreateKey(db, f.owner, KeyInput{Name: "Design", HolderId: f.alice.Id, PolicyTemplate: "creative"}, models)
		require.NoError(t, err)
		// allowance reads the template and the list the key has now.
		allowance := func() []string {
			stored := reloadKey(t, db, grant.Id)
			require.Equal(t, stored.ModelLimits != "", stored.ModelLimitsEnabled, "limited exactly when there is a list")
			return []string{stored.PolicyTemplate, stored.ModelLimits}
		}
		change := func(patch KeyPatch) error {
			_, err := UpdateKey(db, f.owner, grant.Id, patch, models)
			return err
		}

		// From a template to a hand-picked list: the template goes.
		view, err := UpdateKey(db, f.owner, grant.Id, KeyPatch{ModelLimits: modelList("gpt-4o", "claude-sonnet-5")}, models)
		require.NoError(t, err)
		require.Equal(t, []string{"", "claude-sonnet-5,gpt-4o"}, allowance())
		require.Empty(t, view.PolicyTemplate)
		require.Equal(t, []string{"claude-sonnet-5", "gpt-4o"}, view.ModelLimits)
		before, after := keyChange(t, lastAudit(t, db, f.org.id))
		require.Equal(t, "creative", before.PolicyTemplate)
		require.Equal(t, []string{"MiniMax-H3", "claude-*", "dall-e-3", "deeprouter-auto", "gpt-4o*"}, before.ModelLimits)
		require.Empty(t, after.PolicyTemplate)
		require.Equal(t, []string{"claude-sonnet-5", "gpt-4o"}, after.ModelLimits)

		// The same list in another order, or with a name said twice, is no
		// change — and neither is saying "no template" next to it.
		records := len(auditRecords(t, db, f.org.id))
		require.NoError(t, change(KeyPatch{ModelLimits: modelList("gpt-4o", "claude-sonnet-5", "gpt-4o")}))
		require.NoError(t, change(KeyPatch{PolicyTemplate: strPtr(""), ModelLimits: modelList("claude-sonnet-5", "gpt-4o")}))
		require.Len(t, auditRecords(t, db, f.org.id), records)

		// A change to something else leaves the list alone.
		require.NoError(t, change(KeyPatch{Name: strPtr("Design team"), RpmLimit: intPtr(30)}))
		require.Equal(t, []string{"", "claude-sonnet-5,gpt-4o"}, allowance())

		// A longer list, then a shorter one.
		require.NoError(t, change(KeyPatch{ModelLimits: modelList("gpt-4o", "claude-sonnet-5", "dall-e-3")}))
		require.Equal(t, []string{"", "claude-sonnet-5,dall-e-3,gpt-4o"}, allowance())
		require.NoError(t, change(KeyPatch{ModelLimits: modelList("dall-e-3")}))
		require.Equal(t, []string{"", "dall-e-3"}, allowance())

		// From a list to a template: the template's models replace the list.
		require.NoError(t, change(KeyPatch{PolicyTemplate: strPtr("coding")}))
		require.Equal(t, []string{"coding", "claude-sonnet-*,deeprouter-auto,o1*"}, allowance())

		// Naming both is refused, and nothing changes.
		records = len(auditRecords(t, db, f.org.id))
		require.ErrorIs(t, change(KeyPatch{Name: strPtr("Renamed"), PolicyTemplate: strPtr("coding"), ModelLimits: modelList("gpt-4o")}), ErrKeyModelsWithTemplate)
		require.Equal(t, []string{"coding", "claude-sonnet-*,deeprouter-auto,o1*"}, allowance())
		require.Equal(t, "Design team", reloadKey(t, db, grant.Id).Name)

		// So is a model the holder cannot be served, with the rest of the change.
		err = change(KeyPatch{Name: strPtr("Renamed"), ModelLimits: modelList("gpt-4o", "gpt-5-typo")})
		require.ErrorIs(t, err, ErrKeyModelsUnavailable)
		require.Equal(t, []string{"coding", "claude-sonnet-*,deeprouter-auto,o1*"}, allowance())
		require.Equal(t, "Design team", reloadKey(t, db, grant.Id).Name)
		require.Len(t, auditRecords(t, db, f.org.id), records)

		// An empty list is "every model": it lifts a template as much as a
		// list, and so does naming no template.
		require.NoError(t, change(KeyPatch{ModelLimits: modelList()}))
		require.Equal(t, []string{"", ""}, allowance())
		require.NoError(t, change(KeyPatch{ModelLimits: modelList("dall-e-3")}))
		require.NoError(t, change(KeyPatch{PolicyTemplate: strPtr("")}))
		require.Equal(t, []string{"", ""}, allowance())
	})
}

// A change to a hand-picked list is only ever refused for what it adds. What
// the key is limited to already stays, also when it is not — or is no longer —
// a model its holder can be served.
func TestUpdateKey_KeepsWhatTheKeyIsLimitedToAlready(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		var asked []int
		models := askedCatalogue{fixedCatalogue: fixedModels(testCatalogue), asked: &asked}
		down := errors.New("the catalogue is down")
		broken := askedCatalogue{fixedCatalogue: fixedModels(testCatalogue), asked: &asked, err: down}
		// A founder's first key carries the rule of a personal purpose, and the
		// name of a model that has been retired since.
		legacy := seedKey(t, db, f.org, f.org.owner.Id, "first key")
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", legacy.Id).
			Updates(map[string]any{"model_limits_enabled": true, "model_limits": "claude-*,retired-model,gpt-4o"}).Error)
		limits := func() string { return reloadKey(t, db, legacy.Id).ModelLimits }

		// A model can be added next to them.
		view, err := UpdateKey(db, f.owner, legacy.Id, KeyPatch{ModelLimits: modelList("claude-*", "retired-model", "gpt-4o", "dall-e-3")}, models)
		require.NoError(t, err)
		require.Equal(t, []string{"claude-*", "dall-e-3", "gpt-4o", "retired-model"}, view.ModelLimits)
		require.Equal(t, []int{f.org.owner.Id}, asked)

		// One can be taken out while the catalogue cannot be read…
		_, err = UpdateKey(db, f.owner, legacy.Id, KeyPatch{ModelLimits: modelList("claude-*", "gpt-4o", "dall-e-3")}, broken)
		require.NoError(t, err)
		require.Equal(t, "claude-*,dall-e-3,gpt-4o", limits())
		// …and once it is out it is a name like any other: it has to be
		// servable to come back.
		_, err = UpdateKey(db, f.owner, legacy.Id, KeyPatch{ModelLimits: modelList("claude-*", "gpt-4o", "dall-e-3", "retired-model")}, models)
		var unavailable *KeyModelsUnavailableError
		require.ErrorAs(t, err, &unavailable)
		require.Equal(t, []string{"retired-model"}, unavailable.Models)
		// Adding needs the catalogue, and its failure stops the change.
		_, err = UpdateKey(db, f.owner, legacy.Id, KeyPatch{ModelLimits: modelList("claude-*", "claude-opus-4-8")}, broken)
		require.ErrorIs(t, err, down)
		require.Equal(t, "claude-*,dall-e-3,gpt-4o", limits())

		// The catalogue is asked about the key's holder, whoever makes the change.
		asked = nil
		alices := seedKey(t, db, f.org, f.alice.Id, "alice's")
		itOps := f.memberWith(t, db, "it", orgmodel.ScopeOrg, f.general.Id, packPermissions(t, "it_ops")...)
		_, err = UpdateKey(db, itOps, alices.Id, KeyPatch{ModelLimits: modelList("gpt-4o")}, models)
		require.NoError(t, err)
		require.Equal(t, []int{f.alice.Id}, asked)
	})
}

// Saving a form nobody touched is not a change, and the audit log is not told.
func TestUpdateKey_RecordsNothingWhenNothingChanges(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		key := seedKey(t, db, f.org, f.alice.Id, "Steady")
		records := len(auditRecords(t, db, f.org.id))
		for _, patch := range []KeyPatch{
			{},
			{Name: strPtr("Steady")},
			{Name: strPtr("  Steady  "), RemainQuota: intPtr(750), UnlimitedQuota: boolPtr(false), ExpiredTime: int64Ptr(-1), RpmLimit: intPtr(0)},
			{PolicyTemplate: strPtr("")},
		} {
			view, err := UpdateKey(db, f.owner, key.Id, patch, fixedModels(testCatalogue))
			require.NoError(t, err)
			require.Equal(t, "Steady", view.Name)
			require.Equal(t, key, reloadKey(t, db, key.Id))
			require.Len(t, auditRecords(t, db, f.org.id), records)
		}
	})
}

func TestUpdateKey_RefusesBadValues(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		key := seedKey(t, db, f.org, f.alice.Id, "Steady")
		for name, tc := range map[string]struct {
			patch KeyPatch
			want  error
		}{
			"no name":                  {KeyPatch{Name: strPtr(" ")}, ErrInvalidKeyName},
			"a name of 51 characters":  {KeyPatch{Name: strPtr(strings.Repeat("k", KeyNameMaxLength+1))}, ErrInvalidKeyName},
			"a negative quota":         {KeyPatch{RemainQuota: intPtr(-1)}, ErrInvalidKeyQuota},
			"a negative limit":         {KeyPatch{Name: strPtr("Renamed"), TpmLimit: intPtr(-5)}, ErrInvalidKeyLimit},
			"a negative monthly limit": {KeyPatch{MonthlyLimit: intPtr(-5)}, ErrInvalidKeyLimit},
			"an expiry in the past":    {KeyPatch{ExpiredTime: int64Ptr(time.Now().Add(-time.Minute).Unix())}, ErrInvalidKeyExpiry},
		} {
			_, err := UpdateKey(db, f.owner, key.Id, tc.patch, fixedModels(testCatalogue))
			require.ErrorIs(t, err, tc.want, name)
			require.Equal(t, key, reloadKey(t, db, key.Id), name)
		}
		_, err := UpdateKey(db, f.owner, 424242, KeyPatch{Name: strPtr("Renamed")}, fixedModels(testCatalogue))
		require.ErrorIs(t, err, ErrKeyNotFound)
	})
}

// Acceptance: 一键 rotation：key 值更换、配置/归属/历史用量保留、旧值立即失效.
func TestRotateKey_ReplacesTheValueAndKeepsEverythingElse(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		created, err := CreateKey(db, f.owner, KeyInput{
			Name: "Design", HolderId: f.alice.Id, PolicyTemplate: "creative", RemainQuota: 5000, RpmLimit: 60, MonthlyLimit: 3000,
		}, fixedModels(testCatalogue))
		require.NoError(t, err)
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", created.Id).
			Updates(map[string]any{"remain_quota": 4200, "used_quota": 800}).Error)
		before := reloadKey(t, db, created.Id)

		grant, err := RotateKey(db, f.owner, created.Id)
		require.NoError(t, err)

		after := reloadKey(t, db, created.Id)
		require.NotEqual(t, before.Key, after.Key)
		require.Len(t, after.Key, 48)
		// The old value answers to no key at all; the new one to this row.
		require.Zero(t, keysWithValue(t, db, before.Key), "the old value must be dead at once")
		require.EqualValues(t, 1, keysWithValue(t, db, after.Key))
		// Holder, settings, template and what it has spent are untouched.
		expected := before
		expected.Key = after.Key
		require.Equal(t, expected, after)
		require.Equal(t, 800, after.UsedQuota)

		// A person's key: the new value is not shown; its holder runs
		// one-click setup again.
		require.Empty(t, grant.Value)
		require.Equal(t, platformmodel.MaskTokenKey(after.Key), grant.Key)
		require.Equal(t, 800, grant.UsedQuota)

		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyRotate, record.Action)
		require.Equal(t, created.Id, record.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{"before":{"name":"Design","key":%q},"after":{"name":"Design","key":%q}}`,
			platformmodel.MaskTokenKey(before.Key), platformmodel.MaskTokenKey(after.Key)), record.Detail,
			"which value went and which came, both masked")
		requireNoKeyValueInAudit(t, db, f.org.id, before.Key, after.Key)

		// Rotating again works the same; a value is never handed out twice.
		_, err = RotateKey(db, f.owner, created.Id)
		require.NoError(t, err)
		again := reloadKey(t, db, created.Id)
		require.NotContains(t, []string{before.Key, after.Key}, again.Key)
		require.Zero(t, keysWithValue(t, db, after.Key))

		_, err = RotateKey(db, f.owner, 424242)
		require.ErrorIs(t, err, ErrKeyNotFound)
	})
}

// PRD D15: 服务账号的 key 换值后向操作者展示一次.
func TestRotateKey_ShowsTheNewValueOnlyForAServiceAccount(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		key := seedKey(t, db, f.org, f.bot.Id, "the pipeline's")

		grant, err := RotateKey(db, f.owner, key.Id)
		require.NoError(t, err)
		after := reloadKey(t, db, key.Id)
		require.Equal(t, after.Key, grant.Value)
		require.NotEqual(t, key.Key, grant.Value)
		require.Equal(t, platformmodel.MaskTokenKey(after.Key), grant.Key)
		require.Contains(t, lastAudit(t, db, f.org.id).Detail, `"value_shown":true`)
		requireNoKeyValueInAudit(t, db, f.org.id, key.Key, after.Key)
	})
}

// Acceptance: 一键吊销（冻结）与解冻.
func TestFreezeAndUnfreezeKey(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		key := seedKey(t, db, f.org, f.alice.Id, "alice's")
		statusOf := func() int { return reloadKey(t, db, key.Id).Status }

		require.NoError(t, FreezeKey(db, f.owner, key.Id))
		require.Equal(t, common.TokenStatusDisabled, statusOf())
		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyFreeze, record.Action)
		require.Equal(t, key.Id, record.TargetId)
		require.JSONEq(t, `{"before":{"name":"alice's","status":1},"after":{"name":"alice's","status":2}}`, record.Detail)
		frozen := key
		frozen.Status = common.TokenStatusDisabled
		require.Equal(t, frozen, reloadKey(t, db, key.Id), "freezing changes the status and nothing else")

		// Freezing what is frozen does nothing, and records nothing.
		records := len(auditRecords(t, db, f.org.id))
		require.NoError(t, FreezeKey(db, f.owner, key.Id))
		require.Len(t, auditRecords(t, db, f.org.id), records)

		require.NoError(t, UnfreezeKey(db, f.owner, key.Id))
		require.Equal(t, common.TokenStatusEnabled, statusOf())
		record = lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyUnfreeze, record.Action)
		require.JSONEq(t, `{"before":{"name":"alice's","status":2},"after":{"name":"alice's","status":1}}`, record.Detail)
		records = len(auditRecords(t, db, f.org.id))
		require.NoError(t, UnfreezeKey(db, f.owner, key.Id))
		require.Len(t, auditRecords(t, db, f.org.id), records)

		// A key that ran out or expired stays unusable until that is put right.
		set := func(values map[string]any) {
			require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).Updates(values).Error)
		}
		set(map[string]any{"status": common.TokenStatusDisabled, "expired_time": time.Now().Add(-time.Hour).Unix()})
		require.ErrorIs(t, UnfreezeKey(db, f.owner, key.Id), ErrKeyExpired)
		set(map[string]any{"expired_time": -1, "remain_quota": 0})
		require.ErrorIs(t, UnfreezeKey(db, f.owner, key.Id), ErrKeyExhausted)
		require.Equal(t, common.TokenStatusDisabled, statusOf())
		require.Len(t, auditRecords(t, db, f.org.id), records)
		set(map[string]any{"unlimited_quota": true})
		require.NoError(t, UnfreezeKey(db, f.owner, key.Id), "a key without a quota cannot run out")
		// The statuses the gateway sets itself are lifted the same way.
		for _, status := range []int{common.TokenStatusExpired, common.TokenStatusExhausted} {
			set(map[string]any{"status": status})
			require.NoError(t, UnfreezeKey(db, f.owner, key.Id))
			require.Equal(t, common.TokenStatusEnabled, statusOf())
		}

		require.ErrorIs(t, FreezeKey(db, f.owner, 424242), ErrKeyNotFound)
		require.ErrorIs(t, UnfreezeKey(db, f.owner, 424242), ErrKeyNotFound)
	})
}

// Acceptance: 删除 key 后历史账单保留.
func TestDeleteKey_KeepsTheRowAndWhatItSpent(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, db.AutoMigrate(&platformmodel.Log{}))
		f := newKeysFixture(t, db, "Acme")
		key := seedKey(t, db, f.org, f.alice.Id, "alice's")
		kept := seedKey(t, db, f.org, f.alice.Id, "kept")
		usage := platformmodel.Log{UserId: f.alice.Id, TokenId: key.Id, TokenName: key.Name, Quota: 250, OrgId: f.org.id, DepartmentId: f.sales.Id, Content: "consumed"}
		require.NoError(t, db.Create(&usage).Error)

		require.NoError(t, DeleteKey(db, f.owner, key.Id))

		listed, err := ListKeys(db, f.owner)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, kept.Id, listed[0].Id)
		// The row is marked, not removed, and still says what the key spent…
		gone := reloadKey(t, db, key.Id)
		require.True(t, gone.DeletedAt.Valid)
		require.Equal(t, 250, gone.UsedQuota)
		// …and the usage lines are nobody's to touch.
		var line platformmodel.Log
		require.NoError(t, db.First(&line, usage.Id).Error)
		require.Equal(t, []any{key.Id, "alice's", 250}, []any{line.TokenId, line.TokenName, line.Quota})

		record := lastAudit(t, db, f.org.id)
		require.Equal(t, orgmodel.AuditKeyDelete, record.Action)
		require.Equal(t, key.Id, record.TargetId)
		require.JSONEq(t, fmt.Sprintf(`{"before":{"name":"alice's","holder_id":%d,"holder":"alice-of-Acme","department_id":%d,"department":"Sales",
			"policy_template":"","model_limits":[],"remain_quota":750,"unlimited_quota":false,"expired_time":-1,
			"rpm_limit":0,"tpm_limit":0,"monthly_limit":0}}`, f.alice.Id, f.sales.Id), record.Detail)
		requireNoKeyValueInAudit(t, db, f.org.id, key.Key)

		// A deleted key is gone for every other action.
		require.ErrorIs(t, DeleteKey(db, f.owner, key.Id), ErrKeyNotFound)
		_, err = RotateKey(db, f.owner, key.Id)
		require.ErrorIs(t, err, ErrKeyNotFound)
		require.ErrorIs(t, UnfreezeKey(db, f.owner, key.Id), ErrKeyNotFound)
	})
}

// "旧值立即失效" has a second half the database cannot show: the gateway answers
// a request from its token cache for up to a minute without asking the
// database. So every change to a key has to tell that cache — at once, under
// the value the cache knows the key by, and once more for a request that was
// already in flight.
func TestKeyChanges_TellTheGatewaysTokenCache(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		dropped := watchKeyCache(t, true)
		models := fixedModels(testCatalogue)

		for name, change := range map[string]func(keyID int) error{
			"rotating": func(keyID int) error {
				_, err := RotateKey(db, f.owner, keyID)
				return err
			},
			"changing": func(keyID int) error {
				_, err := UpdateKey(db, f.owner, keyID, KeyPatch{RemainQuota: intPtr(1)}, models)
				return err
			},
			// Narrowing what a key may call is no use a minute from now.
			"limiting": func(keyID int) error {
				_, err := UpdateKey(db, f.owner, keyID, KeyPatch{ModelLimits: modelList("gpt-4o")}, models)
				return err
			},
			"freezing":   func(keyID int) error { return FreezeKey(db, f.owner, keyID) },
			"unfreezing": func(keyID int) error { return UnfreezeKey(db, f.owner, keyID) },
			"deleting":   func(keyID int) error { return DeleteKey(db, f.owner, keyID) },
			// A key that changed hands must stop answering to the value its
			// last holder has — now, not a minute from now (P6).
			"handing over": func(keyID int) error {
				_, err := AssignKey(db, f.owner, keyID, f.bob.Id)
				return err
			},
			"taking back": func(keyID int) error {
				_, err := ReclaimKey(db, f.owner, keyID)
				return err
			},
		} {
			key := seedKey(t, db, f.org, f.alice.Id, name)
			if name == "unfreezing" {
				require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", key.Id).Update("status", common.TokenStatusDisabled).Error)
			}
			require.NoError(t, change(key.Id), name)
			require.GreaterOrEqual(t, dropped(key.Key), 1, "%s a key must drop it from the cache before it answers", name)
			require.Eventually(t, func() bool { return dropped(key.Key) == 2 }, 2*time.Second, 5*time.Millisecond,
				"%s a key must drop it once more shortly after", name)
			if name == "rotating" || name == "handing over" || name == "taking back" {
				require.Zero(t, dropped(reloadKey(t, db, key.Id).Key), "the new value was never cached, and is not dropped")
			}
		}

		// What changes nothing tells the cache nothing: a refusal, a form saved
		// untouched, a bad value, unfreezing a key that is not frozen.
		key := seedKey(t, db, f.org, f.alice.Id, "steady")
		_, err := RotateKey(db, actorFor(t, db, f.alice.Id), key.Id)
		require.ErrorIs(t, err, ErrForbidden)
		_, err = UpdateKey(db, f.owner, key.Id, KeyPatch{Name: strPtr("steady")}, models)
		require.NoError(t, err)
		_, err = UpdateKey(db, f.owner, key.Id, KeyPatch{RpmLimit: intPtr(-1)}, models)
		require.ErrorIs(t, err, ErrInvalidKeyLimit)
		require.NoError(t, UnfreezeKey(db, f.owner, key.Id))
		require.Zero(t, dropped(key.Key))
		// A new key is in no cache yet.
		grant, err := CreateKey(db, f.owner, KeyInput{Name: "new", HolderId: f.bot.Id}, models)
		require.NoError(t, err)
		require.Zero(t, dropped(grant.Value))
	})
}

// Where the gateway keeps no cache — no Redis, as in every other test here —
// a change asks once, hears that, and leaves no timer running behind it.
func TestKeyChanges_DoNotComeBackWhereThereIsNoCache(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		dropped := watchKeyCache(t, false)
		key := seedKey(t, db, f.org, f.alice.Id, "steady")

		require.NoError(t, FreezeKey(db, f.owner, key.Id))
		require.Equal(t, 1, dropped(key.Key))
		require.Never(t, func() bool { return dropped(key.Key) > 1 }, 10*keyCacheSweepDelay, 5*time.Millisecond)
	})
}

// keyActions is every change to an existing key, each as one call.
func keyActions(db *gorm.DB) map[string]func(actor *Actor, keyID int) error {
	return map[string]func(actor *Actor, keyID int) error{
		"key.update": func(actor *Actor, keyID int) error {
			_, err := UpdateKey(db, actor, keyID, KeyPatch{Name: strPtr("Renamed")}, fixedModels(testCatalogue))
			return err
		},
		"key.rotate": func(actor *Actor, keyID int) error {
			_, err := RotateKey(db, actor, keyID)
			return err
		},
		"key.freeze": func(actor *Actor, keyID int) error { return FreezeKey(db, actor, keyID) },
		"key.delete": func(actor *Actor, keyID int) error { return DeleteKey(db, actor, keyID) },
		// Taking a key back is the half of key.assign that needs no second
		// member; handing one over is walked in assign_test.go.
		"key.assign": func(actor *Actor, keyID int) error {
			_, err := ReclaimKey(db, actor, keyID)
			return err
		},
	}
}

// Acceptance (the key part of it): 部门作用域硬检查：manager 类角色对非所管部门的
// 成员/key/报表操作一律 403.
func TestKeyActions_StopAtTheDepartmentsTheActorManages(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		desk := f.memberWith(t, db, "desk", orgmodel.ScopeDept, f.sales.Id, everyKeyWrite...)
		actions := keyActions(db)
		actions["unfreeze"] = func(actor *Actor, keyID int) error { return UnfreezeKey(db, actor, keyID) }

		for name, act := range actions {
			inSales := seedKey(t, db, f.org, f.alice.Id, "in Sales")
			inProduct := seedKey(t, db, f.org, f.bob.Id, "in Product")
			parked := seedKey(t, db, f.org, f.org.owner.Id, "parked")
			if name == "unfreeze" {
				require.NoError(t, db.Model(&platformmodel.Token{}).Where("id IN ?", []int{inSales.Id, inProduct.Id, parked.Id}).
					Update("status", common.TokenStatusDisabled).Error)
				inSales, inProduct, parked = reloadKey(t, db, inSales.Id), reloadKey(t, db, inProduct.Id), reloadKey(t, db, parked.Id)
			}
			records := len(auditRecords(t, db, f.org.id))

			require.ErrorIs(t, act(desk, inProduct.Id), ErrForbidden, "%s on a key held in Product", name)
			require.ErrorIs(t, act(desk, parked.Id), ErrForbidden, "%s on a key parked under the owner", name)
			require.Equal(t, inProduct, reloadKey(t, db, inProduct.Id), name)
			require.Equal(t, parked, reloadKey(t, db, parked.Id), name)
			require.Len(t, auditRecords(t, db, f.org.id), records, "%s: a refusal records nothing", name)

			require.NoError(t, act(desk, inSales.Id), "%s on a key held in Sales", name)
			require.NotEqual(t, inSales, reloadKey(t, db, inSales.Id), name)
			require.Len(t, auditRecords(t, db, f.org.id), records+1, name)
		}

		// Reach follows the departments, not the keys: give the desk Product
		// as well and its key there opens up.
		inProduct := seedKey(t, db, f.org, f.bob.Id, "in Product")
		require.NoError(t, UpdateMember(db, f.owner, desk.UserId, MemberPatch{ManagedDepartmentIds: intsPtr(f.product.Id)}))
		desk = actorFor(t, db, desk.UserId)
		_, err := RotateKey(db, desk, inProduct.Id)
		require.NoError(t, err)

		// A key whose holder is no longer in the organization belongs to no
		// department: only a role reaching all of it can act on it.
		orphan := seedKey(t, db, f.org, f.alice.Id, "orphan")
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", f.alice.Id).
			Updates(map[string]any{"org_id": 0, "role_id": 0, "department_id": 0}).Error)
		_, err = RotateKey(db, desk, orphan.Id)
		require.ErrorIs(t, err, ErrForbidden)
		_, err = RotateKey(db, f.owner, orphan.Id)
		require.NoError(t, err)
	})
}

// Each change to a key is a permission of its own. A role that holds every key
// primitive but one is refused that one thing, and a role that holds that one
// alone is allowed it and nothing else — which is what lets a company build
// "may freeze, may not delete" (PRD §2: the HR pack) or "may change, may not
// replace the value".
func TestKeyActions_EachAsksForItsOwnPrimitive(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		allBut := func(primitive string) []string {
			var others []string
			for _, write := range everyKeyWrite {
				if write != primitive {
					others = append(others, write)
				}
			}
			return others
		}

		for primitive, act := range keyActions(db) {
			label := strings.TrimPrefix(primitive, "key.")
			without := f.memberWith(t, db, "all-but-"+label, orgmodel.ScopeOrg, f.general.Id, allBut(primitive)...)
			only := f.memberWith(t, db, "only-"+label, orgmodel.ScopeOrg, f.general.Id, primitive)

			key := seedKey(t, db, f.org, f.alice.Id, label)
			require.ErrorIs(t, act(without, key.Id), ErrForbidden, "every key primitive but %s", primitive)
			require.Equal(t, key, reloadKey(t, db, key.Id), primitive)

			// The one primitive brings nothing it does not name…
			for other, otherAct := range keyActions(db) {
				if other != primitive {
					require.ErrorIs(t, otherAct(only, key.Id), ErrForbidden, "%s does not bring %s", primitive, other)
				}
			}
			require.Equal(t, key, reloadKey(t, db, key.Id), primitive)
			// …and is enough for the thing it does name.
			require.NoError(t, act(only, key.Id), "%s alone", primitive)
		}

		// Unfreezing is the other half of key.freeze, not a permission of its own.
		frozen := seedKey(t, db, f.org, f.alice.Id, "frozen")
		require.NoError(t, FreezeKey(db, f.owner, frozen.Id))
		require.ErrorIs(t, UnfreezeKey(db, f.memberWith(t, db, "no-thaw", orgmodel.ScopeOrg, f.general.Id, allBut("key.freeze")...), frozen.Id), ErrForbidden)
		require.NoError(t, UnfreezeKey(db, f.memberWith(t, db, "thaw", orgmodel.ScopeOrg, f.general.Id, "key.freeze"), frozen.Id))
		require.Equal(t, common.TokenStatusEnabled, reloadKey(t, db, frozen.Id).Status)
	})
}

// Whoever may not do a thing is told so before anything is looked up: the
// answer must not depend on whether the key exists, or whose it is.
func TestKeyActions_RefuseBeforeLookingAnythingUp(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		other := newKeysFixture(t, db, "Other")
		own := seedKey(t, db, f.org, f.alice.Id, "alice's")
		theirs := seedKey(t, db, other.org, other.alice.Id, "theirs")
		manager := actorFor(t, db, seedMemberIn(t, db, f.org, "manager", presetRoleID(t, db, orgmodel.RoleManager), f.sales.Id).Id)
		readonly := actorFor(t, db, seedMember(t, db, f.org, "readonly", orgmodel.RoleReadonly).Id)
		holder := actorFor(t, db, f.alice.Id)
		actions := keyActions(db)
		actions["unfreeze"] = func(actor *Actor, keyID int) error { return UnfreezeKey(db, actor, keyID) }

		before := orgRowCounts(t, db, f.org.id)
		for _, actor := range []*Actor{manager, readonly, holder, actorFor(t, db, f.bot.Id)} {
			for name, act := range actions {
				// The preset manager does hold key.assign, in Sales: what that
				// opens and where it stops is walked in assign_test.go.
				if actor == manager && name == "key.assign" {
					continue
				}
				for target, keyID := range map[string]int{"a key in reach": own.Id, "no key": 424242, "another company's": theirs.Id} {
					require.ErrorIs(t, act(actor, keyID), ErrForbidden, "%s, %s", name, target)
				}
			}
		}
		// Handing a key over is refused the same way, whoever it names — the
		// owner included, which would be taking the key back.
		for _, actor := range []*Actor{readonly, holder, actorFor(t, db, f.bot.Id)} {
			for target, keyID := range map[string]int{"a key in reach": own.Id, "no key": 424242, "another company's": theirs.Id} {
				for whom, holderID := range map[string]int{"a colleague": f.bob.Id, "themselves": actor.UserId, "the owner": f.org.owner.Id, "nobody": 424242, "another company's member": other.alice.Id} {
					_, err := AssignKey(db, actor, keyID, holderID)
					require.ErrorIs(t, err, ErrForbidden, "%s to %s", target, whom)
				}
			}
		}
		require.Equal(t, before, orgRowCounts(t, db, f.org.id))
		require.Equal(t, own, reloadKey(t, db, own.Id), "not even the holder changes their own key")
	})
}

// A key of one company does not exist for the members of another.
func TestKeyActions_StayInsideTheActorsOrganization(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := newKeysFixture(t, db, "Acme")
		other := newKeysFixture(t, db, "Other")
		theirs := seedKey(t, db, other.org, other.alice.Id, "theirs")
		before := orgRowCounts(t, db, other.org.id)
		actions := keyActions(db)
		actions["unfreeze"] = func(actor *Actor, keyID int) error { return UnfreezeKey(db, actor, keyID) }

		for name, act := range actions {
			require.ErrorIs(t, act(acme.owner, theirs.Id), ErrKeyNotFound, name)
		}
		_, err := CreateKey(db, acme.owner, KeyInput{Name: "Plant", HolderId: other.alice.Id}, fixedModels(testCatalogue))
		require.ErrorIs(t, err, ErrMemberNotFound)
		listed, err := ListKeys(db, acme.owner)
		require.NoError(t, err)
		require.Empty(t, listed)
		holders, err := ListKeyHolders(db, acme.owner)
		require.NoError(t, err)
		for _, holder := range holders {
			require.NotContains(t, []int{other.org.owner.Id, other.alice.Id, other.bob.Id, other.bot.Id}, holder.Id)
		}

		require.Equal(t, theirs, reloadKey(t, db, theirs.Id))
		require.Equal(t, before, orgRowCounts(t, db, other.org.id))
	})
}

// Acceptance: 成员的 key 只能由持有人本人经一键配置下发到自己的工具，每次下发留审计
// 记录 (PRD D32: 记在安装脚本被取走的那一刻).
func TestRecordKeyDelivery(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		f := newKeysFixture(t, db, "Acme")
		key := seedKey(t, db, f.org, f.alice.Id, "alice's")
		records := len(auditRecords(t, db, f.org.id))

		require.NoError(t, RecordKeyDelivery(db, &key, []string{"claude-code", "codex"}, "198.51.100.9"))

		record := lastAudit(t, db, f.org.id)
		require.Len(t, auditRecords(t, db, f.org.id), records+1)
		require.Equal(t, orgmodel.AuditKeyDeliver, record.Action)
		require.Equal(t, orgmodel.AuditTargetKey, record.TargetType)
		require.Equal(t, key.Id, record.TargetId)
		require.Equal(t, f.alice.Id, record.ActorUserId, "it is the holder who took it")
		require.Equal(t, "198.51.100.9", record.Ip, "the machine it went to")
		require.NotZero(t, record.CreatedTime)
		require.JSONEq(t, `{"after":{"name":"alice's","tools":["claude-code","codex"]}}`, record.Detail)
		requireNoKeyValueInAudit(t, db, f.org.id, key.Key)

		// A personal key is nobody's business: nothing is written anywhere.
		personal := platformmodel.Token{Id: 9001, UserId: f.alice.Id, Name: "personal", Key: "personal-key"}
		var total int64
		require.NoError(t, db.Model(&orgmodel.OrgAuditLog{}).Count(&total).Error)
		require.NoError(t, RecordKeyDelivery(db, &personal, []string{"codex"}, "198.51.100.9"))
		var after int64
		require.NoError(t, db.Model(&orgmodel.OrgAuditLog{}).Count(&after).Error)
		require.Equal(t, total, after)
	})
}
