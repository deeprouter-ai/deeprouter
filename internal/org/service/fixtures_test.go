package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// testIP is the address test actors act from, so audit records can be checked
// for carrying it.
const testIP = "203.0.113.7"

// testOrg is an organization with its owner, as a sign-up leaves them.
type testOrg struct {
	id    int
	owner platformmodel.User
}

// seedOrg founds an organization owned by a new user. Its preset departments
// carry their English names.
func seedOrg(t *testing.T, db *gorm.DB, name string) testOrg {
	t.Helper()
	founder := seedUser(t, db, "owner-of-"+name, common.RoleCommonUser)
	var org *orgmodel.Organization
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		org, err = CreateForOwnerTx(tx, founder.Id, name, "en")
		return err
	}))
	return testOrg{id: org.Id, owner: reloadUser(t, db, founder.Id)}
}

// seedMember adds a person to an organization with a preset role, in its
// default department — what joining by invite leaves behind.
func seedMember(t *testing.T, db *gorm.DB, org testOrg, username string, role string) platformmodel.User {
	t.Helper()
	return seedMemberIn(t, db, org, username, presetRoleID(t, db, role), defaultDepartment(t, db, org.id).Id)
}

// seedMemberIn adds a person to an organization with any role, in any
// department. It writes the columns directly, so it works whatever the rules
// say about who may hold what.
func seedMemberIn(t *testing.T, db *gorm.DB, org testOrg, username string, roleID int, departmentID int) platformmodel.User {
	t.Helper()
	user := seedUser(t, db, username, common.RoleCommonUser)
	require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"org_id":        org.id,
		"role_id":       roleID,
		"department_id": departmentID,
	}).Error)
	return reloadUser(t, db, user.Id)
}

// seedRole writes a custom role straight into the table, without the checks
// CreateRole makes.
func seedRole(t *testing.T, db *gorm.DB, orgID int, name string, scope string, permissions ...string) orgmodel.OrgRole {
	t.Helper()
	role := orgmodel.OrgRole{OrgId: orgID, Name: name, Scope: scope, Permissions: orgmodel.JoinPermissions(permissions)}
	require.NoError(t, db.Create(&role).Error)
	return role
}

// packPermissions returns the primitives of a role pack.
func packPermissions(t *testing.T, key string) []string {
	t.Helper()
	for _, pack := range orgmodel.RolePacks {
		if pack.Key == key {
			return pack.Permissions
		}
	}
	t.Fatalf("no role pack %s", key)
	return nil
}

// actorFor loads a member as the one performing a management action, acting
// from testIP.
func actorFor(t *testing.T, db *gorm.DB, userID int) *Actor {
	t.Helper()
	actor, err := LoadActor(db, userID)
	require.NoError(t, err)
	actor.Ip = testIP
	return actor
}

// presetRoleID returns the id of a preset role.
func presetRoleID(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	id, err := orgmodel.PresetRoleID(db, name)
	require.NoError(t, err)
	return id
}

// departmentNamed loads a live department of an organization by its name.
func departmentNamed(t *testing.T, db *gorm.DB, orgID int, name string) orgmodel.Department {
	t.Helper()
	var department orgmodel.Department
	require.NoError(t, db.Where("org_id = ? AND name = ?", orgID, name).First(&department).Error)
	return department
}

// managerRows returns the departments added for a member in
// department_managers, in id order.
func managerRows(t *testing.T, db *gorm.DB, userID int) []int {
	t.Helper()
	added, err := extraDepartments(db, []int{userID})
	require.NoError(t, err)
	return added[userID]
}

// auditRecords returns an organization's audit log, oldest first.
func auditRecords(t *testing.T, db *gorm.DB, orgID int) []orgmodel.OrgAuditLog {
	t.Helper()
	var records []orgmodel.OrgAuditLog
	require.NoError(t, db.Where("org_id = ?", orgID).Order("id").Find(&records).Error)
	return records
}

// lastAudit returns the newest audit record of an organization.
func lastAudit(t *testing.T, db *gorm.DB, orgID int) orgmodel.OrgAuditLog {
	t.Helper()
	records := auditRecords(t, db, orgID)
	require.NotEmpty(t, records, "the audit log is empty")
	return records[len(records)-1]
}

// testCatalogue is what the purposes of the policy templates resolve to in
// these tests: two rule tables that share an entry, and two exact lists.
var testCatalogue = map[string][]string{
	"chat":   {"claude-*", "gpt-4o*", "deeprouter-auto"},
	"coding": {"claude-sonnet-*", "o1*", "deeprouter-auto"},
	"image":  {"dall-e-3"},
	"video":  {"MiniMax-H3"},
}

// testServable is what every member can be served in these tests: the models
// a hand-picked list is chosen from. It is deliberately out of order, and
// names one model twice, the way a catalogue read channel by channel does.
var testServable = []string{"gpt-4o", "MiniMax-H3", "claude-sonnet-5", "dall-e-3", "claude-opus-4-8", "gpt-4o"}

// fixedCatalogue is a ModelCatalogue that answers from tables, whoever holds
// the key.
type fixedCatalogue struct {
	purposes map[string][]string
	servable []string
}

// PurposeModels answers a purpose from the table.
func (c fixedCatalogue) PurposeModels(_ int, purpose string) ([]string, error) {
	return c.purposes[purpose], nil
}

// ServableModels answers with the same list for every holder.
func (c fixedCatalogue) ServableModels(int) ([]string, error) {
	return c.servable, nil
}

// fixedModels is a catalogue whose policy templates resolve from table and
// whose members can all be served testServable.
func fixedModels(table map[string][]string) fixedCatalogue {
	return fixedCatalogue{purposes: table, servable: testServable}
}

// seedKey writes an organization key held by holderID straight into the table,
// without the checks CreateKey makes. It has spent a quarter of its quota.
func seedKey(t *testing.T, db *gorm.DB, org testOrg, holderID int, name string) platformmodel.Token {
	t.Helper()
	value, err := common.GenerateKey()
	require.NoError(t, err)
	key := platformmodel.Token{
		UserId:       holderID,
		OrgId:        org.id,
		CreatedBy:    org.owner.Id,
		Name:         name,
		Key:          value,
		Status:       common.TokenStatusEnabled,
		CreatedTime:  1700000000,
		AccessedTime: 1700000000,
		ExpiredTime:  -1,
		RemainQuota:  750,
		UsedQuota:    250,
	}
	require.NoError(t, db.Create(&key).Error)
	return reloadKey(t, db, key.Id)
}

// reloadKey reads a key row back from the database, deleted or not.
func reloadKey(t *testing.T, db *gorm.DB, id int) platformmodel.Token {
	t.Helper()
	var key platformmodel.Token
	require.NoError(t, db.Unscoped().First(&key, id).Error)
	return key
}

// intPtr returns a pointer to n, for the optional fields of a MemberPatch.
func intPtr(n int) *int { return &n }

// intsPtr returns a pointer to a list, for MemberPatch.ManagedDepartmentIds.
func intsPtr(ns ...int) *[]int {
	list := append([]int{}, ns...)
	return &list
}
