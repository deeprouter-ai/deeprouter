package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P4 (meta-repo docs/enterprise-org-prd.md): who may do what.
// Every management action of the package is tried here by every kind of
// member, against a table that says by hand who gets through. The truth table
// in ../model pins the engine; this pins that each action asks the engine the
// right question — a new action that forgets its gate, or its audit record,
// fails here.

// The kinds of member in the cast.
const (
	kindOwner    = "owner"
	kindAdmin    = "admin"
	kindManager  = "manager" // the preset, sitting in Sales and so managing it
	kindReadonly = "readonly"
	kindStaff    = "staff"
	kindBot      = "bot"     // a service account, were it ever to act
	kindITOps    = "it-ops"  // the three role packs, adopted as custom roles
	kindHROps    = "hr-ops"  // — the one non-admin role that invites company-wide
	kindFinance  = "finance" // — and one that sees usage and nothing else
	// kindKeyDesk is a custom role with every key primitive and nothing else,
	// limited to departments and sitting in Sales: the one member whose key
	// actions stop at a department line.
	kindKeyDesk = "key-desk"
)

// allKinds lists the cast in a fixed order, so a failure names the same actor
// on every run.
var allKinds = []string{kindOwner, kindAdmin, kindManager, kindReadonly, kindStaff, kindBot, kindITOps, kindHROps, kindFinance, kindKeyDesk}

// Who an action is open to, as a space-separated list of kinds.
const (
	// runners hold the inherent powers: the owner and the admins.
	runners = kindOwner + " " + kindAdmin
	// readers hold member.read somewhere, directly or through a write.
	readers = runners + " " + kindManager + " " + kindReadonly + " " + kindITOps + " " + kindHROps
	// keyReaders hold key.read somewhere, directly or through a write.
	keyReaders = runners + " " + kindManager + " " + kindReadonly + " " + kindITOps + " " + kindHROps + " " + kindKeyDesk
	// keyMakers hold every key primitive across the whole organization.
	keyMakers = runners + " " + kindITOps
	// keyStoppers may freeze and delete keys across the whole organization:
	// the key makers, and HR Ops for cleaning up after a leaver.
	keyStoppers = keyMakers + " " + kindHROps
)

// cast is one organization with a member of every kind.
type cast struct {
	org     testOrg
	sales   orgmodel.Department
	product orgmodel.Department
	actors  map[string]*Actor
}

// newCast founds an organization and fills it with one member of every kind.
func newCast(t *testing.T, db *gorm.DB, name string) cast {
	t.Helper()
	org := seedOrg(t, db, name)
	c := cast{
		org:     org,
		sales:   departmentNamed(t, db, org.id, "Sales"),
		product: departmentNamed(t, db, org.id, "Product"),
		actors:  map[string]*Actor{kindOwner: actorFor(t, db, org.owner.Id)},
	}
	general := defaultDepartment(t, db, org.id).Id
	join := func(kind string, roleID int, departmentID int) {
		member := seedMemberIn(t, db, org, kind+"-of-"+name, roleID, departmentID)
		c.actors[kind] = actorFor(t, db, member.Id)
	}
	join(kindAdmin, presetRoleID(t, db, orgmodel.RoleAdmin), general)
	join(kindManager, presetRoleID(t, db, orgmodel.RoleManager), c.sales.Id)
	join(kindReadonly, presetRoleID(t, db, orgmodel.RoleReadonly), general)
	join(kindStaff, presetRoleID(t, db, orgmodel.RoleStaff), c.sales.Id)
	join(kindITOps, seedRole(t, db, org.id, "IT Ops", orgmodel.ScopeOrg, packPermissions(t, "it_ops")...).Id, general)
	join(kindHROps, seedRole(t, db, org.id, "HR Ops", orgmodel.ScopeOrg, packPermissions(t, "hr_ops")...).Id, general)
	join(kindFinance, seedRole(t, db, org.id, "Finance", orgmodel.ScopeOrg, packPermissions(t, "finance")...).Id, general)
	join(kindKeyDesk, seedRole(t, db, org.id, "Key Desk", orgmodel.ScopeDept, everyKeyWrite...).Id, c.sales.Id)
	bot, err := CreateServiceAccount(db, c.actors[kindOwner], "CI", 0)
	require.NoError(t, err)
	c.actors[kindBot] = actorFor(t, db, bot.Id)
	return c
}

// managementAction is one management action, ready to be tried by any actor.
type managementAction struct {
	name string
	// openTo lists the kinds of member who may do it.
	openTo string
	// audited is the audit action a success records; empty for a read.
	audited string
	// spawns names what run creates for itself before asking: an action that
	// destroys something needs a fresh victim each time.
	spawns string
	run    func(actor *Actor) error
}

// open reports whether the action is open to a kind of member.
func (a managementAction) open(kind string) bool {
	return strings.Contains(" "+a.openTo+" ", " "+kind+" ")
}

// managementActions returns every management action of the package against
// the cast's organization, each built so that whoever may do it succeeds at
// it any number of times.
func managementActions(t *testing.T, db *gorm.DB, c cast) []managementAction {
	t.Helper()
	serial := 0
	next := func() int { serial++; return serial }
	org := c.org
	support := departmentNamed(t, db, org.id, "Customer Support")
	staffRole := presetRoleID(t, db, orgmodel.RoleStaff)
	managerRole := presetRoleID(t, db, orgmodel.RoleManager)
	readonlyRole := presetRoleID(t, db, orgmodel.RoleReadonly)
	// Three members nobody in the cast is: one to move, one to give roles to,
	// and a manager to give departments to.
	moved := seedMember(t, db, org, fmt.Sprintf("moved-in-%d", org.id), orgmodel.RoleStaff)
	promoted := seedMember(t, db, org, fmt.Sprintf("promoted-in-%d", org.id), orgmodel.RoleStaff)
	lead := seedMemberIn(t, db, org, fmt.Sprintf("lead-in-%d", org.id), managerRole, support.Id)
	edited := seedRole(t, db, org.id, "Edited", orgmodel.ScopeOrg, "usage.read")
	// A holder of keys in each of two departments, and a key of each to act
	// on any number of times.
	inSales := seedMemberIn(t, db, org, fmt.Sprintf("holder-in-sales-of-%d", org.id), staffRole, c.sales.Id)
	inProduct := seedMemberIn(t, db, org, fmt.Sprintf("holder-in-product-of-%d", org.id), staffRole, c.product.Id)
	salesKey := seedKey(t, db, org, inSales.Id, "held in Sales")
	productKey := seedKey(t, db, org, inProduct.Id, "held in Product")
	models := fixedModels(testCatalogue)

	// Victims are written straight to the database, so that making one does
	// not depend on the actor.
	spareDepartment := func() int {
		department := orgmodel.Department{OrgId: org.id, Name: fmt.Sprintf("Spare %d", next())}
		require.NoError(t, db.Create(&department).Error)
		return department.Id
	}
	spareRole := func() int {
		return seedRole(t, db, org.id, fmt.Sprintf("Spare %d", next()), orgmodel.ScopeDept, "key.read").Id
	}
	spareInvite := func(roleID int, departmentID int) int {
		invite := orgmodel.OrgInvite{
			OrgId: org.id, Code: fmt.Sprintf("spare-%d-%d", org.id, next()),
			RoleId: roleID, DepartmentId: departmentID, ExpiresTime: time.Now().Add(time.Hour).Unix(),
		}
		require.NoError(t, db.Create(&invite).Error)
		return invite.Id
	}
	spareKey := func(holderID int) int {
		return seedKey(t, db, org, holderID, fmt.Sprintf("Spare %d", next())).Id
	}
	// Freezing takes a key that is not frozen and unfreezing one that is, so
	// each sets the stage first — straight in the table, like the victims.
	withStatus := func(keyID int, status int) int {
		require.NoError(t, db.Model(&platformmodel.Token{}).Where("id = ?", keyID).Update("status", status).Error)
		return keyID
	}
	createKey := func(actor *Actor, holderID int) error {
		_, err := CreateKey(db, actor, KeyInput{Name: fmt.Sprintf("Made %d", next()), HolderId: holderID}, models)
		return err
	}
	renameKey := func(actor *Actor, keyID int) error {
		_, err := UpdateKey(db, actor, keyID, KeyPatch{Name: strPtr(fmt.Sprintf("Renamed %d", next()))}, models)
		return err
	}
	rotateKey := func(actor *Actor, keyID int) error {
		_, err := RotateKey(db, actor, keyID)
		return err
	}
	// Picking whichever of two values is not the current one makes a change
	// of every call, however often it runs.
	other := func(current int, first int, second int) int {
		if current == first {
			return second
		}
		return first
	}

	return []managementAction{
		// --- reading: member.read, wherever it reaches -------------------------
		{name: "list departments", openTo: readers, run: func(actor *Actor) error {
			_, err := ListDepartments(db, actor)
			return err
		}},
		{name: "list roles", openTo: readers, run: func(actor *Actor) error {
			_, err := ListRoles(db, actor)
			return err
		}},
		{name: "read the permission catalogue", openTo: readers, run: func(actor *Actor) error {
			_, err := GetPermissionCatalog(actor)
			return err
		}},
		{name: "list members", openTo: readers, run: func(actor *Actor) error {
			_, err := ListMembers(db, actor)
			return err
		}},

		// --- inherent powers: the owner and the admins --------------------------
		{name: "create a department", openTo: runners, audited: orgmodel.AuditDepartmentCreate, run: func(actor *Actor) error {
			_, err := CreateDepartment(db, actor, fmt.Sprintf("New %d", next()))
			return err
		}},
		{name: "rename a department", openTo: runners, audited: orgmodel.AuditDepartmentRename, run: func(actor *Actor) error {
			return RenameDepartment(db, actor, c.product.Id, fmt.Sprintf("Product %d", next()))
		}},
		{name: "delete a department", openTo: runners, audited: orgmodel.AuditDepartmentDelete, spawns: "departments", run: func(actor *Actor) error {
			return DeleteDepartment(db, actor, spareDepartment())
		}},
		{name: "create a role", openTo: runners, audited: orgmodel.AuditRoleCreate, run: func(actor *Actor) error {
			_, err := CreateRole(db, actor, RoleInput{Name: fmt.Sprintf("Made %d", next()), Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}})
			return err
		}},
		{name: "adopt a role pack", openTo: runners, audited: orgmodel.AuditRoleCreate, run: func(actor *Actor) error {
			_, err := AdoptRolePack(db, actor, "finance", fmt.Sprintf("Adopted %d", next()))
			return err
		}},
		{name: "change a role", openTo: runners, audited: orgmodel.AuditRoleUpdate, run: func(actor *Actor) error {
			_, err := UpdateRole(db, actor, edited.Id, RoleInput{Name: fmt.Sprintf("Edited %d", next()), Scope: orgmodel.ScopeOrg, Permissions: []string{"usage.read"}})
			return err
		}},
		{name: "delete a role", openTo: runners, audited: orgmodel.AuditRoleDelete, spawns: "roles", run: func(actor *Actor) error {
			return DeleteRole(db, actor, spareRole())
		}},
		{name: "give a member a role", openTo: runners, audited: orgmodel.AuditRoleAssign, run: func(actor *Actor) error {
			current := reloadUser(t, db, promoted.Id).OrgRoleId
			return UpdateMember(db, actor, promoted.Id, MemberPatch{RoleId: intPtr(other(current, readonlyRole, staffRole))})
		}},
		{name: "move a member", openTo: runners, audited: orgmodel.AuditMemberMove, run: func(actor *Actor) error {
			current := reloadUser(t, db, moved.Id).DepartmentId
			return UpdateMember(db, actor, moved.Id, MemberPatch{DepartmentId: intPtr(other(current, c.sales.Id, c.product.Id))})
		}},
		{name: "set what a member manages", openTo: runners, audited: orgmodel.AuditMemberManages, run: func(actor *Actor) error {
			current := 0
			if added := managerRows(t, db, lead.Id); len(added) > 0 {
				current = added[0]
			}
			return UpdateMember(db, actor, lead.Id, MemberPatch{ManagedDepartmentIds: intsPtr(other(current, c.sales.Id, c.product.Id))})
		}},
		{name: "create a service account", openTo: runners, audited: orgmodel.AuditServiceAccountCreate, run: func(actor *Actor) error {
			_, err := CreateServiceAccount(db, actor, "Bot", 0)
			return err
		}},

		// --- member.invite: into the departments it reaches, staff only ----------
		{name: "invite staff into Sales", openTo: runners + " " + kindManager + " " + kindHROps, audited: orgmodel.AuditMemberInvite, run: func(actor *Actor) error {
			_, err := CreateInvite(db, actor, staffRole, c.sales.Id)
			return err
		}},
		{name: "invite staff into Product", openTo: runners + " " + kindHROps, audited: orgmodel.AuditMemberInvite, run: func(actor *Actor) error {
			_, err := CreateInvite(db, actor, staffRole, c.product.Id)
			return err
		}},
		{name: "invite a manager into Sales", openTo: runners, audited: orgmodel.AuditMemberInvite, run: func(actor *Actor) error {
			_, err := CreateInvite(db, actor, managerRole, c.sales.Id)
			return err
		}},
		{name: "list invites", openTo: runners + " " + kindManager + " " + kindHROps, run: func(actor *Actor) error {
			_, err := ListInvites(db, actor)
			return err
		}},
		{name: "revoke a staff invite into Sales", openTo: runners + " " + kindManager + " " + kindHROps, audited: orgmodel.AuditInviteRevoke, spawns: "invites", run: func(actor *Actor) error {
			return RevokeInvite(db, actor, spareInvite(staffRole, c.sales.Id))
		}},
		{name: "revoke a staff invite into Product", openTo: runners + " " + kindHROps, audited: orgmodel.AuditInviteRevoke, spawns: "invites", run: func(actor *Actor) error {
			return RevokeInvite(db, actor, spareInvite(staffRole, c.product.Id))
		}},
		{name: "revoke a manager invite into Sales", openTo: runners, audited: orgmodel.AuditInviteRevoke, spawns: "invites", run: func(actor *Actor) error {
			return RevokeInvite(db, actor, spareInvite(managerRole, c.sales.Id))
		}},

		// --- key.read, wherever it reaches ----------------------------------------
		{name: "list keys", openTo: keyReaders, run: func(actor *Actor) error {
			_, err := ListKeys(db, actor)
			return err
		}},
		{name: "read the policy templates", openTo: keyReaders, run: func(actor *Actor) error {
			_, err := ListKeyTemplates(actor)
			return err
		}},

		// --- key.create — with key.assign when the key is for someone (PRD D30) ----
		{name: "list who a key can be for", openTo: keyMakers + " " + kindKeyDesk, run: func(actor *Actor) error {
			_, err := ListKeyHolders(db, actor)
			return err
		}},
		{name: "create a key parked under the owner", openTo: keyMakers, audited: orgmodel.AuditKeyCreate, run: func(actor *Actor) error {
			return createKey(actor, 0)
		}},
		{name: "create a key for a member of Sales", openTo: keyMakers + " " + kindKeyDesk, audited: orgmodel.AuditKeyCreate, run: func(actor *Actor) error {
			return createKey(actor, inSales.Id)
		}},
		{name: "create a key for a member of Product", openTo: keyMakers, audited: orgmodel.AuditKeyCreate, run: func(actor *Actor) error {
			return createKey(actor, inProduct.Id)
		}},

		// --- key.create or key.update, reaching the member the key is for ---------
		{name: "list the models a key for a member of Sales can be limited to", openTo: keyMakers + " " + kindKeyDesk, run: func(actor *Actor) error {
			_, err := ListKeyModels(db, actor, inSales.Id, models)
			return err
		}},
		{name: "list the models a key for a member of Product can be limited to", openTo: keyMakers, run: func(actor *Actor) error {
			_, err := ListKeyModels(db, actor, inProduct.Id, models)
			return err
		}},

		// --- the other key primitives, each on a key held in Sales and on one
		// held in Product: the first is within the key desk's reach, the second
		// is not ---------------------------------------------------------------
		{name: "change a key held in Sales", openTo: keyMakers + " " + kindKeyDesk, audited: orgmodel.AuditKeyUpdate, run: func(actor *Actor) error {
			return renameKey(actor, salesKey.Id)
		}},
		{name: "change a key held in Product", openTo: keyMakers, audited: orgmodel.AuditKeyUpdate, run: func(actor *Actor) error {
			return renameKey(actor, productKey.Id)
		}},
		{name: "rotate a key held in Sales", openTo: keyMakers + " " + kindKeyDesk, audited: orgmodel.AuditKeyRotate, run: func(actor *Actor) error {
			return rotateKey(actor, salesKey.Id)
		}},
		{name: "rotate a key held in Product", openTo: keyMakers, audited: orgmodel.AuditKeyRotate, run: func(actor *Actor) error {
			return rotateKey(actor, productKey.Id)
		}},
		{name: "freeze a key held in Sales", openTo: keyStoppers + " " + kindKeyDesk, audited: orgmodel.AuditKeyFreeze, run: func(actor *Actor) error {
			return FreezeKey(db, actor, withStatus(salesKey.Id, common.TokenStatusEnabled))
		}},
		{name: "freeze a key held in Product", openTo: keyStoppers, audited: orgmodel.AuditKeyFreeze, run: func(actor *Actor) error {
			return FreezeKey(db, actor, withStatus(productKey.Id, common.TokenStatusEnabled))
		}},
		{name: "unfreeze a key held in Sales", openTo: keyStoppers + " " + kindKeyDesk, audited: orgmodel.AuditKeyUnfreeze, run: func(actor *Actor) error {
			return UnfreezeKey(db, actor, withStatus(salesKey.Id, common.TokenStatusDisabled))
		}},
		{name: "unfreeze a key held in Product", openTo: keyStoppers, audited: orgmodel.AuditKeyUnfreeze, run: func(actor *Actor) error {
			return UnfreezeKey(db, actor, withStatus(productKey.Id, common.TokenStatusDisabled))
		}},
		{name: "delete a key held in Sales", openTo: keyStoppers + " " + kindKeyDesk, audited: orgmodel.AuditKeyDelete, spawns: "keys", run: func(actor *Actor) error {
			return DeleteKey(db, actor, spareKey(inSales.Id))
		}},
		{name: "delete a key held in Product", openTo: keyStoppers, audited: orgmodel.AuditKeyDelete, spawns: "keys", run: func(actor *Actor) error {
			return DeleteKey(db, actor, spareKey(inProduct.Id))
		}},

		// --- audit.read, across the whole organization ---------------------------
		{name: "read the audit log", openTo: runners + " " + kindReadonly, run: func(actor *Actor) error {
			_, _, err := ListAuditLogs(db, actor, 0, 10)
			return err
		}},
	}
}

// orgRowCounts counts what management actions can create, per organization.
func orgRowCounts(t *testing.T, db *gorm.DB, orgID int) map[string]int64 {
	t.Helper()
	counts := map[string]int64{}
	for name, table := range map[string]any{
		"departments": &orgmodel.Department{},
		"roles":       &orgmodel.OrgRole{},
		"invites":     &orgmodel.OrgInvite{},
		"members":     &platformmodel.User{},
		"keys":        &platformmodel.Token{},
		"audit":       &orgmodel.OrgAuditLog{},
	} {
		var n int64
		require.NoError(t, db.Model(table).Where("org_id = ?", orgID).Count(&n).Error)
		counts[name] = n
	}
	return counts
}

// Acceptance: 部门作用域硬检查 and the rest of the role table, one action at a
// time. Every pair of action and kind of member has an expected answer.
func TestManagement_EachActionIsOpenToExactlyWhoThePRDSays(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCast(t, db, "Acme")
		actions := managementActions(t, db, c)
		require.Len(t, actions, 41, "a new management action belongs in managementActions")
		require.Len(t, c.actors, len(allKinds))

		for _, action := range actions {
			for _, kind := range allKinds {
				// Loaded again for every try: an earlier action may have
				// changed something, and a real request always starts fresh.
				actor := actorFor(t, db, c.actors[kind].UserId)
				err := action.run(actor)
				if action.open(kind) {
					require.NoError(t, err, "%s must be able to %s", kind, action.name)
				} else {
					require.ErrorIs(t, err, ErrForbidden, "%s must not %s", kind, action.name)
				}
			}
		}
	})
}

// Refusals come before anything is written: a refused actor leaves the
// organization exactly as it was — audit log included.
func TestManagement_ARefusalChangesNothing(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCast(t, db, "Acme")
		product := c.product
		for _, action := range managementActions(t, db, c) {
			for _, kind := range allKinds {
				if action.open(kind) {
					continue
				}
				before := orgRowCounts(t, db, c.org.id)
				require.ErrorIs(t, action.run(c.actors[kind]), ErrForbidden, "%s, %s", kind, action.name)
				after := orgRowCounts(t, db, c.org.id)
				// A destructive action wrote itself a fresh victim before it
				// asked; nothing else may appear, and nothing may go.
				if action.spawns != "" {
					before[action.spawns]++
				}
				require.Equal(t, before, after, "%s, %s", kind, action.name)
			}
		}
		require.Equal(t, product, departmentNamed(t, db, c.org.id, "Product"))
	})
}

// Whoever may not change members is turned away before anything is looked up.
// The answer must not depend on who the target is or whether they exist:
// otherwise a manager could learn who is in the organization, and would be
// told "the owner's role cannot be changed" where the truth is "not yours to
// ask".
func TestUpdateMember_RefusesBeforeLookingAnythingUp(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCast(t, db, "Acme")
		other := seedOrg(t, db, "Other")
		stranger := seedMember(t, db, other, "stranger", orgmodel.RoleStaff)
		ownerRole := presetRoleID(t, db, orgmodel.RoleOwner)
		targets := map[string]int{
			"the owner":                   c.org.owner.Id,
			"an admin":                    c.actors[kindAdmin].UserId,
			"a member of their own":       c.actors[kindStaff].UserId,
			"nobody":                      424242,
			"another organization's":      stranger.Id,
			"themselves (see each actor)": 0,
		}
		patches := map[string]MemberPatch{
			"nothing":               {},
			"the owner role":        {RoleId: intPtr(ownerRole)},
			"an unknown role":       {RoleId: intPtr(424242)},
			"an unknown department": {DepartmentId: intPtr(424242)},
			"departments to manage": {ManagedDepartmentIds: intsPtr(c.sales.Id)},
		}
		before := orgRowCounts(t, db, c.org.id)
		for _, kind := range []string{kindManager, kindReadonly, kindStaff, kindBot, kindITOps, kindHROps, kindFinance, kindKeyDesk} {
			actor := c.actors[kind]
			for target, memberID := range targets {
				if memberID == 0 {
					memberID = actor.UserId
				}
				for what, patch := range patches {
					require.ErrorIs(t, UpdateMember(db, actor, memberID, patch), ErrForbidden,
						"%s giving %s %s", kind, target, what)
				}
			}
		}
		require.Equal(t, before, orgRowCounts(t, db, c.org.id))
	})
}

// PRD D17: every management write goes into the audit log, with who did it and
// from where — and a read leaves no trace.
func TestManagement_EveryWriteIsRecordedAndNoReadIs(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCast(t, db, "Acme")
		admin := c.actors[kindAdmin]
		for _, action := range managementActions(t, db, c) {
			before := auditRecords(t, db, c.org.id)
			require.NoError(t, action.run(admin), action.name)
			after := auditRecords(t, db, c.org.id)
			if action.audited == "" {
				require.Len(t, after, len(before), "%s is a read and must leave no record", action.name)
				continue
			}
			require.Len(t, after, len(before)+1, "%s must leave exactly one record", action.name)
			record := after[len(after)-1]
			require.Equal(t, action.audited, record.Action, action.name)
			require.Equal(t, c.org.id, record.OrgId, action.name)
			require.Equal(t, admin.UserId, record.ActorUserId, action.name)
			require.Equal(t, testIP, record.Ip, action.name)
			require.NotEmpty(t, record.TargetType, action.name)
			require.NotZero(t, record.TargetId, action.name)
			require.NotZero(t, record.CreatedTime, action.name)
			require.NotEqual(t, "{}", record.Detail, "%s must say what changed", action.name)
		}
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
		theirRole, err := CreateRole(db, theirOwner, RoleInput{Name: "Their role", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.delete"}})
		require.NoError(t, err)
		before := orgRowCounts(t, db, other.id)

		require.ErrorIs(t, RenameDepartment(db, intruder, theirSales.Id, "Mine now"), ErrDepartmentNotFound)
		require.ErrorIs(t, DeleteDepartment(db, intruder, theirSales.Id), ErrDepartmentNotFound)
		require.ErrorIs(t, UpdateMember(db, intruder, theirMember.Id, MemberPatch{RoleId: intPtr(presetRoleID(t, db, orgmodel.RoleReadonly))}), ErrMemberNotFound)
		require.ErrorIs(t, RevokeInvite(db, intruder, theirInvite.Id), ErrInviteNotFound)
		_, err = CreateServiceAccount(db, intruder, "Plant", theirSales.Id)
		require.ErrorIs(t, err, ErrDepartmentNotFound)
		_, err = UpdateRole(db, intruder, theirRole.Id, RoleInput{Name: "Mine now", Scope: orgmodel.ScopeOrg, Permissions: []string{"key.read"}})
		require.ErrorIs(t, err, ErrRoleNotFound)
		require.ErrorIs(t, DeleteRole(db, intruder, theirRole.Id), ErrRoleNotFound)
		own := seedMember(t, db, acme, "own-member", orgmodel.RoleManager)
		require.ErrorIs(t, UpdateMember(db, intruder, own.Id, MemberPatch{ManagedDepartmentIds: intsPtr(theirSales.Id)}), ErrDepartmentNotFound)

		require.Equal(t, before, orgRowCounts(t, db, other.id))
		require.Equal(t, "Sales", departmentNamed(t, db, other.id, "Sales").Name)
		require.Equal(t, theirMember, reloadUser(t, db, theirMember.Id))
		require.Empty(t, managerRows(t, db, own.Id))

		// What the intruder can list is their own company's, and only that.
		departments, err := ListDepartments(db, intruder)
		require.NoError(t, err)
		for _, department := range departments {
			require.NotEqual(t, theirSales.Id, department.Id)
		}
		members, err := ListMembers(db, intruder)
		require.NoError(t, err)
		require.Len(t, members, 2)
		require.Equal(t, acme.owner.Id, members[0].Id)
		invites, err := ListInvites(db, intruder)
		require.NoError(t, err)
		require.Empty(t, invites)
		roles, err := ListRoles(db, intruder)
		require.NoError(t, err)
		for _, role := range roles {
			require.NotEqual(t, theirRole.Id, role.Id)
		}
		// Nor their audit log: the intruder sees no record of the other
		// company, which by now has some.
		require.NotEmpty(t, auditRecords(t, db, other.id))
		records, total, err := ListAuditLogs(db, intruder, 0, 100)
		require.NoError(t, err)
		require.Empty(t, records)
		require.Zero(t, total)
	})
}
