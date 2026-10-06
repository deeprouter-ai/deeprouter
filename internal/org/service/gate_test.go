package service

import (
	"fmt"
	"testing"
	"time"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P3: until P4's Can() arrives, the permission model is one
// line — the owner and the admins run the organization, nobody else may. This
// pins that line on every management action, so a new action that forgets
// the gate fails here.

// managementAction is one management action, ready to be tried by any actor.
type managementAction struct {
	name string
	run  func(actor *Actor) error
}

// managementActions returns every management action of the package against
// one organization, each built so that a manager would succeed at it any
// number of times.
func managementActions(t *testing.T, db *gorm.DB, org testOrg) []managementAction {
	t.Helper()
	serial := 0
	next := func() int { serial++; return serial }
	sales := departmentNamed(t, db, org.id, "Sales")
	product := departmentNamed(t, db, org.id, "Product")
	target := seedMember(t, db, org, fmt.Sprintf("target-of-%d", org.id), orgmodel.RoleStaff)
	staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
	// Destructive actions get a fresh victim each run, written straight to
	// the database so that making it does not depend on the actor.
	spareDepartment := func() int {
		department := orgmodel.Department{OrgId: org.id, Name: fmt.Sprintf("Spare %d", next())}
		require.NoError(t, db.Create(&department).Error)
		return department.Id
	}
	spareInvite := func() int {
		invite := orgmodel.OrgInvite{
			OrgId: org.id, Code: fmt.Sprintf("spare-%d-%d", org.id, next()),
			RoleId: staffRole, DepartmentId: sales.Id, ExpiresTime: time.Now().Add(time.Hour).Unix(),
		}
		require.NoError(t, db.Create(&invite).Error)
		return invite.Id
	}
	return []managementAction{
		{"list departments", func(actor *Actor) error {
			_, err := ListDepartments(db, actor)
			return err
		}},
		{"create a department", func(actor *Actor) error {
			_, err := CreateDepartment(db, actor, fmt.Sprintf("New %d", next()))
			return err
		}},
		{"rename a department", func(actor *Actor) error {
			return RenameDepartment(db, actor, product.Id, fmt.Sprintf("Product %d", next()))
		}},
		{"delete a department", func(actor *Actor) error {
			return DeleteDepartment(db, actor, spareDepartment())
		}},
		{"list roles", func(actor *Actor) error {
			_, err := ListRoles(db, actor)
			return err
		}},
		{"list members", func(actor *Actor) error {
			_, err := ListMembers(db, actor)
			return err
		}},
		{"update a member", func(actor *Actor) error {
			return UpdateMember(db, actor, target.Id, MemberPatch{DepartmentId: intPtr(sales.Id)})
		}},
		{"create a service account", func(actor *Actor) error {
			_, err := CreateServiceAccount(db, actor, "Bot", 0)
			return err
		}},
		{"create an invite", func(actor *Actor) error {
			_, err := CreateInvite(db, actor, staffRole, 0)
			return err
		}},
		{"list invites", func(actor *Actor) error {
			_, err := ListInvites(db, actor)
			return err
		}},
		{"revoke an invite", func(actor *Actor) error {
			return RevokeInvite(db, actor, spareInvite())
		}},
	}
}

// orgRowCounts counts what management actions can create, per organization.
func orgRowCounts(t *testing.T, db *gorm.DB, orgID int) map[string]int64 {
	t.Helper()
	counts := map[string]int64{}
	for name, table := range map[string]any{
		"departments": &orgmodel.Department{},
		"invites":     &orgmodel.OrgInvite{},
		"members":     &platformmodel.User{},
	} {
		var n int64
		require.NoError(t, db.Model(table).Where("org_id = ?", orgID).Count(&n).Error)
		counts[name] = n
	}
	return counts
}

func TestManagement_IsOpenToTheOwnerAndAdminsOnly(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		actions := managementActions(t, db, acme)
		require.Len(t, actions, 11, "a new management action belongs in managementActions")

		for _, role := range []string{orgmodel.RoleManager, orgmodel.RoleStaff, orgmodel.RoleReadonly} {
			member := seedMember(t, db, acme, "plain-"+role, role)
			actor := actorFor(t, db, member.Id)
			for _, action := range actions {
				require.ErrorIs(t, action.run(actor), ErrForbidden, "%s must not %s", role, action.name)
			}
		}
		// A service account is staff; were it ever to act, it is refused too.
		bot, err := CreateServiceAccount(db, actorFor(t, db, acme.owner.Id), "CI", 0)
		require.NoError(t, err)
		for _, action := range actions {
			require.ErrorIs(t, action.run(actorFor(t, db, bot.Id)), ErrForbidden, "a service account must not %s", action.name)
		}

		admin := seedMember(t, db, acme, "the-admin", orgmodel.RoleAdmin)
		for _, userID := range []int{acme.owner.Id, admin.Id} {
			actor := actorFor(t, db, userID)
			for _, action := range actions {
				require.NoError(t, action.run(actor), "user %d could not %s", userID, action.name)
			}
		}
	})
}

// Refusals must come before anything is written: a refused actor leaves the
// organization exactly as it was.
func TestManagement_ARefusalChangesNothing(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		actions := managementActions(t, db, acme)
		staff := actorFor(t, db, seedMember(t, db, acme, "staff", orgmodel.RoleStaff).Id)
		product := departmentNamed(t, db, acme.id, "Product")

		for _, action := range actions {
			before := orgRowCounts(t, db, acme.id)
			require.ErrorIs(t, action.run(staff), ErrForbidden, action.name)
			after := orgRowCounts(t, db, acme.id)
			// The two destructive actions get a fresh victim written for them
			// before the gate runs; nothing else may appear, and nothing may go.
			switch action.name {
			case "delete a department":
				before["departments"]++
			case "revoke an invite":
				before["invites"]++
			}
			require.Equal(t, before, after, action.name)
		}
		require.Equal(t, "Product", departmentNamed(t, db, acme.id, "Product").Name)
		require.Equal(t, product.Id, departmentNamed(t, db, acme.id, "Product").Id)
	})
}

// A manager of one company is a stranger in every other: their actions
// neither see nor touch another organization's rows.
func TestManagement_StaysInsideTheActorsOrganization(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		intruder := actorFor(t, db, acme.owner.Id)
		theirOwner := actorFor(t, db, other.owner.Id)
		theirSales := departmentNamed(t, db, other.id, "Sales")
		theirMember := seedMember(t, db, other, "their-member", orgmodel.RoleStaff)
		theirInvite, err := CreateInvite(db, theirOwner, presetRoleID(t, db, orgmodel.RoleStaff), theirSales.Id)
		require.NoError(t, err)
		before := orgRowCounts(t, db, other.id)

		require.ErrorIs(t, RenameDepartment(db, intruder, theirSales.Id, "Mine now"), ErrDepartmentNotFound)
		require.ErrorIs(t, DeleteDepartment(db, intruder, theirSales.Id), ErrDepartmentNotFound)
		require.ErrorIs(t, UpdateMember(db, intruder, theirMember.Id, MemberPatch{RoleId: intPtr(presetRoleID(t, db, orgmodel.RoleReadonly))}), ErrMemberNotFound)
		require.ErrorIs(t, RevokeInvite(db, intruder, theirInvite.Id), ErrInviteNotFound)
		_, err = CreateServiceAccount(db, intruder, "Plant", theirSales.Id)
		require.ErrorIs(t, err, ErrDepartmentNotFound)

		require.Equal(t, before, orgRowCounts(t, db, other.id))
		require.Equal(t, "Sales", departmentNamed(t, db, other.id, "Sales").Name)
		require.Equal(t, theirMember, reloadUser(t, db, theirMember.Id))

		// What the intruder can list is their own company's, and only that.
		departments, err := ListDepartments(db, intruder)
		require.NoError(t, err)
		for _, department := range departments {
			require.NotEqual(t, theirSales.Id, department.Id)
		}
		members, err := ListMembers(db, intruder)
		require.NoError(t, err)
		require.Len(t, members, 1)
		require.Equal(t, acme.owner.Id, members[0].Id)
		invites, err := ListInvites(db, intruder)
		require.NoError(t, err)
		require.Empty(t, invites)
	})
}
