package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
	user := seedUser(t, db, username, common.RoleCommonUser)
	roleID, err := orgmodel.PresetRoleID(db, role)
	require.NoError(t, err)
	require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"org_id":        org.id,
		"role_id":       roleID,
		"department_id": defaultDepartment(t, db, org.id).Id,
	}).Error)
	return reloadUser(t, db, user.Id)
}

// actorFor loads a member as the one performing a management action.
func actorFor(t *testing.T, db *gorm.DB, userID int) *Actor {
	t.Helper()
	actor, err := LoadActor(db, userID)
	require.NoError(t, err)
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

// intPtr returns a pointer to n, for the optional fields of a MemberPatch.
func intPtr(n int) *int { return &n }
