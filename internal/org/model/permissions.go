package model

import "strings"

// Primitives is the fixed list custom roles are built from (PRD §2). Names are
// "resource.action" with no wildcards; how far a primitive reaches is decided
// by the role's scope, never by the name.
var Primitives = []string{
	"key.read",
	"key.create",
	"key.update",
	"key.assign",
	"key.rotate",
	"key.freeze",
	"key.delete",
	"member.read",
	"member.invite",
	"member.remove",
	"usage.read",
	"alert.read",
	"audit.read",
}

// Names of the five platform preset roles.
const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleManager  = "manager"
	RoleStaff    = "staff"
	RoleReadonly = "readonly"
)

// PresetRole is the code-side definition of one platform preset role.
type PresetRole struct {
	Name        string
	Scope       string
	Permissions []string
}

// PresetRoles is the one and only definition of the five preset roles (PRD §2,
// decision D23). The org_roles rows with org_id = 0 are synced to it on every
// boot, so the database, the API and the UI can never disagree with it.
//
// Managing the organization itself — roles, departments, settings, appointing
// admins, the wallet — is not a primitive. Those powers belong to owner and
// admin by identity and cannot be granted through a role.
var PresetRoles = []PresetRole{
	{Name: RoleOwner, Scope: ScopeOrg, Permissions: Primitives},
	{Name: RoleAdmin, Scope: ScopeOrg, Permissions: Primitives},
	{Name: RoleManager, Scope: ScopeDept, Permissions: []string{
		"key.read", "key.assign", "member.read", "member.invite", "usage.read", "alert.read",
	}},
	{Name: RoleStaff, Scope: ScopeSelf, Permissions: nil},
	{Name: RoleReadonly, Scope: ScopeOrg, Permissions: []string{
		"key.read", "member.read", "usage.read", "alert.read", "audit.read",
	}},
}

// Names of the inherent powers: managing the organization itself (PRD §2).
// They come with being the owner or an admin, are judged by who the member is
// rather than by what their role lists, and can never be put into a custom
// role.
const (
	PowerRoles           = "role.manage"            // create, change and delete custom roles; give members a role
	PowerDepartments     = "department.manage"      // create, rename and delete departments; move members between them
	PowerServiceAccounts = "service_account.manage" // create service accounts (PRD D27)
	PowerSettings        = "org.settings"           // alert thresholds and working hours
	PowerAlerts          = "alert.handle"           // mark an alert handled or a false alarm
	PowerAdmins          = "admin.appoint"          // appoint and dismiss admins
	PowerWallet          = "wallet.manage"          // the company wallet
	PowerOwnership       = "org.transfer"           // transfer or close the organization
)

// InherentPower is one power that belongs to the owner, and unless it is
// OwnerOnly, to the admins as well.
type InherentPower struct {
	Name      string `json:"name"`
	OwnerOnly bool   `json:"owner_only"`
}

// InherentPowers is the one and only list of inherent powers, in the order the
// roles page shows them.
var InherentPowers = []InherentPower{
	{Name: PowerRoles},
	{Name: PowerDepartments},
	{Name: PowerServiceAccounts},
	{Name: PowerSettings},
	{Name: PowerAlerts},
	{Name: PowerAdmins, OwnerOnly: true},
	{Name: PowerWallet, OwnerOnly: true},
	{Name: PowerOwnership, OwnerOnly: true},
}

// RolePack is a ready-made custom role the platform offers (PRD §2, decision
// D23). Adopting one copies it into an organization, which then owns the copy
// and changes it freely; a pack edited here only shapes future adoptions.
type RolePack struct {
	Key         string
	Name        string // the name the copy gets unless the adopter gives another
	Scope       string
	Permissions []string
}

// RolePacks is the one and only definition of the role packs. An auditor needs
// no pack: that job is the readonly preset.
var RolePacks = []RolePack{
	{Key: "it_ops", Name: "IT Ops", Scope: ScopeOrg, Permissions: []string{
		"key.read", "key.create", "key.update", "key.assign", "key.rotate", "key.freeze", "key.delete",
		"member.read", "usage.read", "alert.read",
	}},
	{Key: "hr_ops", Name: "HR Ops", Scope: ScopeOrg, Permissions: []string{
		"key.read", "key.freeze", "key.delete", "member.read", "member.invite", "member.remove",
	}},
	{Key: "finance", Name: "Finance Ops", Scope: ScopeOrg, Permissions: []string{
		"usage.read",
	}},
}

// NormalizePermissions returns the requested primitives without duplicates, in
// the order of Primitives — the one form org_roles.permissions is stored in.
// It reports false when a request names something that is not a primitive.
func NormalizePermissions(requested []string) ([]string, bool) {
	wanted := make(map[string]bool, len(requested))
	for _, name := range requested {
		wanted[strings.TrimSpace(name)] = true
	}
	normalized := make([]string, 0, len(wanted))
	for _, primitive := range Primitives {
		if wanted[primitive] {
			normalized = append(normalized, primitive)
			delete(wanted, primitive)
		}
	}
	return normalized, len(wanted) == 0
}

// WithImpliedReads returns the primitives together with the reads they bring:
// a write includes the read of the same resource (PRD §2), so a role holding
// key.assign holds key.read as well. Custom roles are stored in this form and
// members are told their permissions in it, which leaves every client with a
// plain list to look things up in — the rule itself lives here and in Can.
func WithImpliedReads(permissions []string) []string {
	subject := Subject{Permissions: permissions}
	expanded := make([]string, 0, len(permissions))
	for _, primitive := range Primitives {
		if subject.Holds(primitive) {
			expanded = append(expanded, primitive)
		}
	}
	return expanded
}

// JoinPermissions renders primitives the way org_roles.permissions stores them.
func JoinPermissions(permissions []string) string {
	return strings.Join(permissions, ",")
}

// SplitPermissions parses org_roles.permissions back into primitives.
func SplitPermissions(stored string) []string {
	permissions := []string{}
	for _, p := range strings.Split(stored, ",") {
		if p = strings.TrimSpace(p); p != "" {
			permissions = append(permissions, p)
		}
	}
	return permissions
}
