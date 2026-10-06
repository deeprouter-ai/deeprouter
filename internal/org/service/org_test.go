package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"github.com/QuantumNous/new-api/internal/org/orgtest"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// forEachDialect runs fn against a fresh database holding the users table and
// the migrated org tables.
func forEachDialect(t *testing.T, fn func(t *testing.T, db *gorm.DB)) {
	t.Helper()
	orgtest.ForEachDialect(t, func(t *testing.T, db *gorm.DB) {
		require.NoError(t, db.AutoMigrate(&platformmodel.User{}))
		require.NoError(t, orgmodel.Migrate(db))
		fn(t, db)
	})
}

// seedUser inserts a platform user with the given global role.
func seedUser(t *testing.T, db *gorm.DB, username string, role int) platformmodel.User {
	t.Helper()
	user := platformmodel.User{
		Username: username,
		Password: "hashed-password",
		Role:     role,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    1000,
		AffCode:  username,
	}
	require.NoError(t, db.Create(&user).Error)
	return reloadUser(t, db, user.Id)
}

// reloadUser reads a user row back from the database.
func reloadUser(t *testing.T, db *gorm.DB, id int) platformmodel.User {
	t.Helper()
	var user platformmodel.User
	require.NoError(t, db.First(&user, id).Error)
	return user
}

// defaultDepartment loads the default department of an organization.
func defaultDepartment(t *testing.T, db *gorm.DB, orgID int) orgmodel.Department {
	t.Helper()
	var department orgmodel.Department
	require.NoError(t, db.Where("org_id = ? AND is_default = ?", orgID, true).First(&department).Error)
	return department
}

// countOrganizations returns how many organizations exist.
func countOrganizations(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&orgmodel.Organization{}).Count(&count).Error)
	return count
}

func TestNormalizeName(t *testing.T) {
	longest := strings.Repeat("企", NameMaxLength)
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{"Acme", "Acme"},
		{"  Acme Pty Ltd  ", "Acme Pty Ltd"},
		{longest, longest}, // the limit counts characters, not bytes
	} {
		got, err := NormalizeName(tc.raw)
		require.NoError(t, err, tc.raw)
		require.Equal(t, tc.want, got)
	}
	for _, raw := range []string{"", "   ", "\t\n", longest + "企"} {
		_, err := NormalizeName(raw)
		require.ErrorIs(t, err, ErrInvalidName, "%q", raw)
	}
}

func TestCreateForOwnerTx_MakesTheUserTheSoleOwner(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		bystander := seedUser(t, db, "bystander", common.RoleCommonUser)
		founder := seedUser(t, db, "founder", common.RoleCommonUser)
		ownerRoleID, err := orgmodel.PresetRoleID(db, orgmodel.RoleOwner)
		require.NoError(t, err)

		var org *orgmodel.Organization
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			var err error
			org, err = CreateForOwnerTx(tx, founder.Id, "  Acme Pty Ltd  ", "en")
			return err
		}))

		var stored orgmodel.Organization
		require.NoError(t, db.First(&stored, org.Id).Error)
		require.Equal(t, "Acme Pty Ltd", stored.Name)
		require.Equal(t, founder.Id, stored.OwnerUserId)
		require.NotZero(t, stored.CreatedTime)

		// The founder gained exactly the three org columns and nothing else.
		want := founder
		want.OrgId = org.Id
		want.OrgRoleId = ownerRoleID
		want.DepartmentId = defaultDepartment(t, db, org.Id).Id
		require.Equal(t, want, reloadUser(t, db, founder.Id))
		// Nobody else was pulled in: the organization has one member, its owner.
		require.Equal(t, bystander, reloadUser(t, db, bystander.Id))
		var members int64
		require.NoError(t, db.Model(&platformmodel.User{}).Where("org_id = ?", org.Id).Count(&members).Error)
		require.EqualValues(t, 1, members)
	})
}

// PRD §7.3 red line 1: owning an organization is not a platform privilege.
// The global role must come out exactly as it went in, whatever it was.
func TestCreateForOwnerTx_NeverWritesTheGlobalRole(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
			user := seedUser(t, db, fmt.Sprintf("role-%d", role), role)
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				_, err := CreateForOwnerTx(tx, user.Id, "Org of role", "en")
				return err
			}))
			require.Equal(t, role, reloadUser(t, db, user.Id).Role)
		}
	})
}

// PRD D20: one person, one organization.
func TestCreateForOwnerTx_RejectsAUserWhoAlreadyBelongsToOne(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		founder := seedUser(t, db, "founder", common.RoleCommonUser)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			_, err := CreateForOwnerTx(tx, founder.Id, "First", "en")
			return err
		}))
		member := reloadUser(t, db, founder.Id)

		err := db.Transaction(func(tx *gorm.DB) error {
			_, err := CreateForOwnerTx(tx, founder.Id, "Second", "en")
			return err
		})
		require.ErrorIs(t, err, ErrNotPersonalAccount)
		require.EqualValues(t, 1, countOrganizations(t, db), "the second organization must roll back")
		require.Equal(t, member, reloadUser(t, db, founder.Id))
	})
}

func TestCreateForOwnerTx_RejectsAnUnknownUser(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		err := db.Transaction(func(tx *gorm.DB) error {
			_, err := CreateForOwnerTx(tx, 4242, "Ghost Inc", "en")
			return err
		})
		require.ErrorIs(t, err, ErrNotPersonalAccount)
		require.Zero(t, countOrganizations(t, db))
	})
}

func TestCreateForOwnerTx_RejectsABadNameBeforeWritingAnything(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		founder := seedUser(t, db, "founder", common.RoleCommonUser)
		err := db.Transaction(func(tx *gorm.DB) error {
			_, err := CreateForOwnerTx(tx, founder.Id, "   ", "en")
			return err
		})
		require.ErrorIs(t, err, ErrInvalidName)
		require.Zero(t, countOrganizations(t, db))
		require.Equal(t, founder, reloadUser(t, db, founder.Id))
	})
}

// The sign-up owns the transaction: if anything after the organization fails,
// the organization goes with it.
func TestCreateForOwnerTx_RollsBackWithTheCallersTransaction(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		founder := seedUser(t, db, "founder", common.RoleCommonUser)
		later := errors.New("a later sign-up step failed")
		err := db.Transaction(func(tx *gorm.DB) error {
			if _, err := CreateForOwnerTx(tx, founder.Id, "Acme", "en"); err != nil {
				return err
			}
			return later
		})
		require.ErrorIs(t, err, later)
		require.Zero(t, countOrganizations(t, db))
		require.Equal(t, founder, reloadUser(t, db, founder.Id))
	})
}

func TestGetMembership(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		personal := seedUser(t, db, "personal", common.RoleCommonUser)
		founder := seedUser(t, db, "founder", common.RoleCommonUser)
		var org *orgmodel.Organization
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			var err error
			org, err = CreateForOwnerTx(tx, founder.Id, "Acme", "en")
			return err
		}))

		ownerRoleID, err := orgmodel.PresetRoleID(db, orgmodel.RoleOwner)
		require.NoError(t, err)

		none, err := GetMembership(db, personal.Id)
		require.NoError(t, err)
		require.Nil(t, none, "a personal account has no membership")

		got, err := GetMembership(db, founder.Id)
		require.NoError(t, err)
		require.Equal(t, &Membership{
			OrgId:                org.Id,
			OrgName:              "Acme",
			IsOwner:              true,
			RoleId:               ownerRoleID,
			Role:                 orgmodel.RoleOwner,
			RoleScope:            orgmodel.ScopeOrg,
			Permissions:          orgmodel.Primitives,
			DepartmentId:         defaultDepartment(t, db, org.Id).Id,
			ManagedDepartmentIds: []int{},
		}, got)
	})
}

// A role id that points at another company's custom role must not resolve:
// showing it would leak that company's role name and permissions.
func TestGetMembership_DoesNotResolveAnotherOrganizationsRole(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		founder := seedUser(t, db, "founder", common.RoleCommonUser)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			_, err := CreateForOwnerTx(tx, founder.Id, "Acme", "en")
			return err
		}))
		foreign := orgmodel.OrgRole{OrgId: 999, Name: "their-secret-role", Scope: orgmodel.ScopeOrg, Permissions: "key.delete"}
		require.NoError(t, db.Create(&foreign).Error)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", founder.Id).
			Update("role_id", foreign.Id).Error)

		_, err := GetMembership(db, founder.Id)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}
