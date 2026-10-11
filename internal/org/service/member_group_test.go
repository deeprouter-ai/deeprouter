package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P11 (meta-repo docs/enterprise-org-prd.md D49), acceptance
// items "平台设置'组织成员所在分组'打开后……三条路径的行为与改动前完全一致" and
// "开关打开后，后台任务……把仍在 default 的现有组织成员移到设置的分组……".

// setMemberGroup switches org_setting.member_group for one test and puts the
// default back afterwards. Nothing in this package runs tests in parallel, so
// changing the shared setting is safe.
func setMemberGroup(t *testing.T, group string, enabled bool) {
	t.Helper()
	saved := memberGroupSetting
	memberGroupSetting = MemberGroupSetting{Group: group, Enabled: enabled}
	t.Cleanup(func() { memberGroupSetting = saved })
}

// groupOf reads users.group back from the database.
func groupOf(t *testing.T, db *gorm.DB, userID int) string {
	t.Helper()
	return reloadUser(t, db, userID).Group
}

// joinAllThreeWays puts one account into an organization by each entry point
// — founding it, signing up through an invite, being created as a service
// account — and returns their ids in that order.
func joinAllThreeWays(t *testing.T, db *gorm.DB) []int {
	t.Helper()
	acme := seedOrg(t, db, "Acme")
	owner := actorFor(t, db, acme.owner.Id)
	invite, err := CreateInvite(db, owner, presetRoleID(t, db, orgmodel.RoleStaff), 0)
	require.NoError(t, err)
	newcomer := seedUser(t, db, "newcomer", common.RoleCommonUser)
	require.NoError(t, joinByInvite(db, newcomer.Id, invite.Code))
	bot, err := CreateServiceAccount(db, owner, "CI", 0)
	require.NoError(t, err)
	return []int{acme.owner.Id, newcomer.Id, bot.Id}
}

func TestMemberGroup_SwitchedOffTheEntryPointsLeaveDefault(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		// The shipped default: the name is set, the switch is off.
		setMemberGroup(t, "enterprise", false)
		for _, id := range joinAllThreeWays(t, db) {
			require.Equal(t, "default", groupOf(t, db, id))
		}
	})
}

func TestMemberGroup_SwitchedOnTheEntryPointsAssignTheGroup(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		setMemberGroup(t, "enterprise", true)
		for _, id := range joinAllThreeWays(t, db) {
			require.Equal(t, "enterprise", groupOf(t, db, id))
		}
	})
}

func TestMemberGroup_AnUnusableGroupIsNeverWritten(t *testing.T) {
	for _, tc := range []struct{ name, group string }{
		{"not in the group ratios", "no-such-group"},
		{"empty", "  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forEachDialect(t, func(t *testing.T, db *gorm.DB) {
				setMemberGroup(t, tc.group, true)
				for _, id := range joinAllThreeWays(t, db) {
					require.Equal(t, "default", groupOf(t, db, id))
				}
			})
		})
	}
}

func TestApplyMemberGroup_MovesOnlyOrganizationAccountsStillInDefault(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		// Everyone joins while the switch is off, as before this feature.
		setMemberGroup(t, "enterprise", false)
		acme := seedOrg(t, db, "Acme")
		staff := seedMember(t, db, acme, "staff", orgmodel.RoleStaff)
		bot, err := CreateServiceAccount(db, actorFor(t, db, acme.owner.Id), "CI", 0)
		require.NoError(t, err)
		vip := seedMember(t, db, acme, "vip-by-hand", orgmodel.RoleStaff)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", vip.Id).
			Update("group", "vip").Error)
		personal := seedUser(t, db, "personal", common.RoleCommonUser)
		removed := seedMember(t, db, acme, "removed", orgmodel.RoleStaff)
		require.NoError(t, db.Delete(&platformmodel.User{}, removed.Id).Error)

		setMemberGroup(t, "enterprise", true)
		moved, err := ApplyMemberGroup(db)
		require.NoError(t, err)

		require.Equal(t, 3, moved) // owner, staff, service account
		for _, id := range []int{acme.owner.Id, staff.Id, bot.Id} {
			require.Equal(t, "enterprise", groupOf(t, db, id))
		}
		require.Equal(t, "vip", groupOf(t, db, vip.Id), "a group set by hand is kept")
		require.Equal(t, "default", groupOf(t, db, personal.Id), "a personal account is never touched")
		var gone platformmodel.User
		require.NoError(t, db.Unscoped().First(&gone, removed.Id).Error)
		require.Equal(t, "default", gone.Group, "a removed member is never touched")

		again, err := ApplyMemberGroup(db)
		require.NoError(t, err)
		require.Zero(t, again, "a second pass has nothing left to move")
	})
}

func TestApplyMemberGroup_DoesNothingUnlessThereIsAGroupToMoveTo(t *testing.T) {
	for _, tc := range []struct {
		name    string
		group   string
		enabled bool
	}{
		{"switched off", "enterprise", false},
		{"not in the group ratios", "no-such-group", true},
		{"empty", "", true},
		{"the default group itself", "default", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forEachDialect(t, func(t *testing.T, db *gorm.DB) {
				setMemberGroup(t, "enterprise", false)
				acme := seedOrg(t, db, "Acme")
				staff := seedMember(t, db, acme, "staff", orgmodel.RoleStaff)

				setMemberGroup(t, tc.group, tc.enabled)
				moved, err := ApplyMemberGroup(db)
				require.NoError(t, err)
				require.Zero(t, moved)
				require.Equal(t, "default", groupOf(t, db, acme.owner.Id))
				require.Equal(t, "default", groupOf(t, db, staff.Id))
			})
		})
	}
}
