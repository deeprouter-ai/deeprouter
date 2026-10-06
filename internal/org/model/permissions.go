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
