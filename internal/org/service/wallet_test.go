package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestSpendOf_PaysFromTheOwnerAndStampsTheHoldersDepartment covers the three
// kinds of holder an organization key can have. Whoever holds the key, the
// owner's balance pays; the department is the holder's own.
func TestSpendOf_PaysFromTheOwnerAndStampsTheHoldersDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org := seedOrg(t, db, "Acme")
		general := defaultDepartment(t, db, org.id)
		sales := departmentNamed(t, db, org.id, "Sales")
		staff := seedMemberIn(t, db, org, "staff", presetRoleID(t, db, orgmodel.RoleStaff), sales.Id)
		bot, err := CreateServiceAccount(db, actorFor(t, db, org.owner.Id), "CI", sales.Id)
		require.NoError(t, err)

		spend, err := SpendOf(db, org.id, staff.Id)
		require.NoError(t, err)
		assert.Equal(t, Spend{OrgId: org.id, DepartmentId: sales.Id, WalletUserId: org.owner.Id}, spend,
			"a member's key spends the owner's balance and is stamped with the member's department")

		spend, err = SpendOf(db, org.id, bot.Id)
		require.NoError(t, err)
		assert.Equal(t, Spend{OrgId: org.id, DepartmentId: sales.Id, WalletUserId: org.owner.Id}, spend,
			"a service account's key does the same")

		spend, err = SpendOf(db, org.id, org.owner.Id)
		require.NoError(t, err)
		assert.Equal(t, Spend{OrgId: org.id, DepartmentId: general.Id, WalletUserId: org.owner.Id}, spend,
			"the owner's own key spends their own balance, stamped with the default department")
	})
}

// TestSpendOf_FollowsTheHolderToTheirNewDepartment is why nothing is cached: a
// member who is moved is stamped with the new department from the next request.
func TestSpendOf_FollowsTheHolderToTheirNewDepartment(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org := seedOrg(t, db, "Acme")
		sales := departmentNamed(t, db, org.id, "Sales")
		product := departmentNamed(t, db, org.id, "Product")
		staff := seedMemberIn(t, db, org, "staff", presetRoleID(t, db, orgmodel.RoleStaff), sales.Id)

		before, err := SpendOf(db, org.id, staff.Id)
		require.NoError(t, err)
		require.NoError(t, UpdateMember(db, actorFor(t, db, org.owner.Id), staff.Id, MemberPatch{DepartmentId: intPtr(product.Id)}))
		after, err := SpendOf(db, org.id, staff.Id)
		require.NoError(t, err)

		assert.Equal(t, sales.Id, before.DepartmentId)
		assert.Equal(t, product.Id, after.DepartmentId)
	})
}

// TestSpendOf_KeepsOrganizationsApart makes sure one company's key never
// reaches another company's wallet.
func TestSpendOf_KeepsOrganizationsApart(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		globex := seedOrg(t, db, "Globex")
		acmeStaff := seedMember(t, db, acme, "acme-staff", orgmodel.RoleStaff)
		globexStaff := seedMember(t, db, globex, "globex-staff", orgmodel.RoleStaff)

		spend, err := SpendOf(db, acme.id, acmeStaff.Id)
		require.NoError(t, err)
		assert.Equal(t, acme.owner.Id, spend.WalletUserId)

		spend, err = SpendOf(db, globex.id, globexStaff.Id)
		require.NoError(t, err)
		assert.Equal(t, globex.owner.Id, spend.WalletUserId)

		// A key that says Acme, held by somebody at Globex: no wallet answers.
		_, err = SpendOf(db, acme.id, globexStaff.Id)
		assert.ErrorIs(t, err, ErrNoWallet)
	})
}

// TestSpendOf_RefusesWhenNothingCanPay lists every way an organization key can
// be left without a wallet. Each must be refused, never billed to anyone else.
func TestSpendOf_RefusesWhenNothingCanPay(t *testing.T) {
	cases := []struct {
		name    string
		breakIt func(t *testing.T, db *gorm.DB, org testOrg, staff platformmodel.User)
	}{
		{"the holder was removed from the organization", func(t *testing.T, db *gorm.DB, org testOrg, staff platformmodel.User) {
			_, err := RemoveMember(db, actorFor(t, db, org.owner.Id), staff.Id)
			require.NoError(t, err)
		}},
		{"the holder's account was deleted by the platform", func(t *testing.T, db *gorm.DB, _ testOrg, staff platformmodel.User) {
			require.NoError(t, db.Delete(&platformmodel.User{}, staff.Id).Error)
		}},
		{"the holder is not in an organization", func(t *testing.T, db *gorm.DB, _ testOrg, staff platformmodel.User) {
			require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", staff.Id).Update("org_id", 0).Error)
		}},
		{"the owner's account is disabled", func(t *testing.T, db *gorm.DB, org testOrg, _ platformmodel.User) {
			require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", org.owner.Id).
				Update("status", common.UserStatusDisabled).Error)
		}},
		{"the owner's account was deleted", func(t *testing.T, db *gorm.DB, org testOrg, _ platformmodel.User) {
			require.NoError(t, db.Delete(&platformmodel.User{}, org.owner.Id).Error)
		}},
		{"the organization is gone", func(t *testing.T, db *gorm.DB, org testOrg, _ platformmodel.User) {
			require.NoError(t, db.Delete(&orgmodel.Organization{}, org.id).Error)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forEachDialect(t, func(t *testing.T, db *gorm.DB) {
				org := seedOrg(t, db, "Acme")
				staff := seedMember(t, db, org, "staff", orgmodel.RoleStaff)
				_, err := SpendOf(db, org.id, staff.Id)
				require.NoError(t, err, "the key can be billed before anything is broken")

				tc.breakIt(t, db, org, staff)

				spend, err := SpendOf(db, org.id, staff.Id)
				assert.ErrorIs(t, err, ErrNoWallet)
				assert.Equal(t, Spend{}, spend)
			})
		})
	}
}

// TestSpendOf_FollowsTheOrganizationToANewOwner covers the hand-over the
// platform does by hand (PRD D19): the next request is paid by the new owner.
func TestSpendOf_FollowsTheOrganizationToANewOwner(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org := seedOrg(t, db, "Acme")
		admin := seedMember(t, db, org, "admin", orgmodel.RoleAdmin)
		staff := seedMember(t, db, org, "staff", orgmodel.RoleStaff)

		require.NoError(t, db.Model(&orgmodel.Organization{}).Where("id = ?", org.id).
			Update("owner_user_id", admin.Id).Error)

		spend, err := SpendOf(db, org.id, staff.Id)
		require.NoError(t, err)
		assert.Equal(t, admin.Id, spend.WalletUserId)
	})
}

// TestWalletWatchers_AreTheOwnerAndTheAdmins pins who hears that the company
// wallet is running low — and, by leaving everyone else out, who does not.
func TestWalletWatchers_AreTheOwnerAndTheAdmins(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, org.owner.Id)
		admin := seedMember(t, db, org, "admin", orgmodel.RoleAdmin)
		secondAdmin := seedMember(t, db, org, "second-admin", orgmodel.RoleAdmin)
		seedMember(t, db, org, "manager", orgmodel.RoleManager)
		seedMember(t, db, org, "staff", orgmodel.RoleStaff)
		seedMember(t, db, org, "readonly", orgmodel.RoleReadonly)
		// A custom role holding every primitive is still not an admin.
		everything := seedRole(t, db, org.id, "Everything", orgmodel.ScopeOrg, orgmodel.Primitives...)
		seedMemberIn(t, db, org, "almost-admin", everything.Id, defaultDepartment(t, db, org.id).Id)
		_, err := CreateServiceAccount(db, owner, "CI", 0)
		require.NoError(t, err)
		// An admin whose account is disabled is not written to.
		disabled := seedMember(t, db, org, "disabled-admin", orgmodel.RoleAdmin)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", disabled.Id).
			Update("status", common.UserStatusDisabled).Error)
		// Another company's owner and admin are none of this one's business.
		other := seedOrg(t, db, "Globex")
		seedMember(t, db, other, "globex-admin", orgmodel.RoleAdmin)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", admin.Id).
			Updates(map[string]any{"email": "admin@acme.test", "setting": `{"notify_type":"webhook"}`}).Error)

		watchers, err := WalletWatchers(db, org.id, org.owner.Id)
		require.NoError(t, err)

		ids := make([]int, 0, len(watchers))
		for _, watcher := range watchers {
			ids = append(ids, watcher.Id)
		}
		assert.Equal(t, []int{org.owner.Id, admin.Id, secondAdmin.Id}, ids)
		assert.Equal(t, "admin@acme.test", watchers[1].Email)
		assert.Equal(t, "webhook", watchers[1].GetSetting().NotifyType,
			"each watcher comes with the settings their notification is sent by")
	})
}

// TestWalletWatchers_IncludeAWalletHolderWhoseRoleWasNotUpdated covers an
// organization handed over by hand: the wallet's holder is told whatever their
// role column still says.
func TestWalletWatchers_IncludeAWalletHolderWhoseRoleWasNotUpdated(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org := seedOrg(t, db, "Acme")
		staff := seedMember(t, db, org, "staff", orgmodel.RoleStaff)

		watchers, err := WalletWatchers(db, org.id, staff.Id)
		require.NoError(t, err)

		require.Len(t, watchers, 1)
		assert.Equal(t, staff.Id, watchers[0].Id)
	})
}
