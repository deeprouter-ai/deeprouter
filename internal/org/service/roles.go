package service

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// RoleNameMaxLength is the longest role name, in characters.
const RoleNameMaxLength = 64

var (
	// ErrRoleNotFound means the role is neither a preset nor one of this organization's.
	ErrRoleNotFound = errors.New("role not found in this organization")
	// ErrInvalidRoleName means the role name is blank or too long.
	ErrInvalidRoleName = errors.New("role name must be 1 to 64 characters")
	// ErrRoleExists means the name is a preset's or already one of the
	// organization's roles.
	ErrRoleExists = errors.New("a role with this name already exists")
	// ErrPresetRole means the action would change or delete a preset role.
	// Presets are the platform's, the same for every organization.
	ErrPresetRole = errors.New("a preset role cannot be changed or deleted")
	// ErrInvalidRoleScope means a custom role asked for a scope other than the
	// whole organization or the departments its holder manages.
	ErrInvalidRoleScope = errors.New("a custom role reaches the whole organization or the departments its holder manages")
	// ErrInvalidRolePermissions means a custom role lists no permission, or
	// something that is not one of the primitives.
	ErrInvalidRolePermissions = errors.New("a custom role needs at least one permission from the list")
	// ErrAuditNeedsOrgScope means a department-scoped role asked for audit.read.
	// The audit log belongs to the organization as a whole.
	ErrAuditNeedsOrgScope = errors.New("audit.read needs a role that reaches the whole organization")
	// ErrRolePackNotFound means there is no role pack with that key.
	ErrRolePackNotFound = errors.New("role pack not found")
)

// RoleView is a role as the management pages show it.
type RoleView struct {
	Id          int      `json:"id"`
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
	// Powers is the inherent powers that come with holding the role: all of
	// them for the owner preset, the shared ones for the admin preset, and
	// none for any other role — a custom role can never carry one.
	Powers   []string `json:"powers"`
	IsPreset bool     `json:"is_preset"`
}

// RoleInput is what a custom role is made of.
type RoleInput struct {
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

// RolePackView is a role pack as the roles page offers it.
type RolePackView struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

// PermissionCatalog is everything a role can be made of, and what the roles
// page draws its matrix from: the page holds no list of its own, so it cannot
// drift from what the backend enforces.
type PermissionCatalog struct {
	Primitives []string                 `json:"primitives"`
	Powers     []orgmodel.InherentPower `json:"powers"`
	RolePacks  []RolePackView           `json:"role_packs"`
}

// removedRole is what the audit log keeps of a deleted role: the role, and the
// members who held it and became staff.
type removedRole struct {
	RoleView
	MemberIds []int `json:"member_ids"`
}

// isPreset reports whether a role is the platform preset with the given name.
// A custom role that happens to carry a preset's name is not that preset.
func isPreset(role *orgmodel.OrgRole, name string) bool {
	return role.OrgId == 0 && role.Name == name
}

// viewRole renders a role for the management pages and the audit log.
func viewRole(role *orgmodel.OrgRole) RoleView {
	// The engine is asked rather than a list consulted, so the roles page
	// shows what Can would answer and nothing else.
	holder := orgmodel.Subject{
		IsOwner: isPreset(role, orgmodel.RoleOwner),
		IsAdmin: isPreset(role, orgmodel.RoleAdmin),
	}
	powers := []string{}
	for _, power := range orgmodel.InherentPowers {
		if orgmodel.Can(holder, power.Name, wholeOrganization) {
			powers = append(powers, power.Name)
		}
	}
	return RoleView{
		Id:          role.Id,
		Name:        role.Name,
		Scope:       role.Scope,
		Permissions: orgmodel.SplitPermissions(role.Permissions),
		Powers:      powers,
		IsPreset:    role.IsPreset,
	}
}

// findRole loads a role an organization may use: a platform preset or one of
// its own. Anything else would be another company's custom role.
func findRole(db *gorm.DB, orgID int, roleID int) (*orgmodel.OrgRole, error) {
	var found []orgmodel.OrgRole
	if err := db.Where("id = ? AND org_id IN ?", roleID, []int{0, orgID}).
		Limit(1).Find(&found).Error; err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, ErrRoleNotFound
	}
	return &found[0], nil
}

// findCustomRole loads one of the organization's own roles — the only kind
// that can be changed or deleted.
func findCustomRole(db *gorm.DB, orgID int, roleID int) (*orgmodel.OrgRole, error) {
	role, err := findRole(db, orgID, roleID)
	if err != nil {
		return nil, err
	}
	if role.OrgId == 0 {
		return nil, ErrPresetRole
	}
	return role, nil
}

// loadRoles returns the roles an organization may use, presets first.
func loadRoles(db *gorm.DB, orgID int) ([]orgmodel.OrgRole, error) {
	var roles []orgmodel.OrgRole
	err := db.Where("org_id IN ?", []int{0, orgID}).Order("org_id").Order("id").Find(&roles).Error
	return roles, err
}

// roleHolders returns the ids of the members who hold a role.
func roleHolders(db *gorm.DB, orgID int, roleID int) ([]int, error) {
	holders := []int{}
	err := db.Model(&platformmodel.User{}).
		Where("org_id = ? AND role_id = ?", orgID, roleID).Order("id").Pluck("id", &holders).Error
	return holders, err
}

// normalizeRole checks what a custom role is asked to be and returns it in the
// form it is stored in.
func normalizeRole(input RoleInput) (*orgmodel.OrgRole, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || utf8.RuneCountInString(name) > RoleNameMaxLength {
		return nil, ErrInvalidRoleName
	}
	// "Only themselves" is what the staff preset is; a custom role exists to
	// manage something, so it reaches the organization or some departments.
	if input.Scope != orgmodel.ScopeOrg && input.Scope != orgmodel.ScopeDept {
		return nil, ErrInvalidRoleScope
	}
	permissions, known := orgmodel.NormalizePermissions(input.Permissions)
	if !known || len(permissions) == 0 {
		return nil, ErrInvalidRolePermissions
	}
	// Stored with the reads its writes bring, so the role reads the same on
	// every page as it behaves.
	permissions = orgmodel.WithImpliedReads(permissions)
	if input.Scope == orgmodel.ScopeDept && slices.Contains(permissions, "audit.read") {
		return nil, ErrAuditNeedsOrgScope
	}
	return &orgmodel.OrgRole{
		Name:        name,
		Scope:       input.Scope,
		Permissions: orgmodel.JoinPermissions(permissions),
	}, nil
}

// checkRoleName refuses a name a custom role cannot take: a preset's, in any
// letter case, or one another role of the organization already has. The pages
// tell roles apart by name, so two with one name would be one to the eye.
func checkRoleName(db *gorm.DB, orgID int, name string, exceptID int) error {
	for _, preset := range orgmodel.PresetRoles {
		if strings.EqualFold(preset.Name, name) {
			return ErrRoleExists
		}
	}
	var taken int64
	if err := db.Model(&orgmodel.OrgRole{}).
		Where("org_id = ? AND name = ? AND id <> ?", orgID, name, exceptID).Count(&taken).Error; err != nil {
		return err
	}
	if taken > 0 {
		return ErrRoleExists
	}
	return nil
}

// ListRoles returns the roles members of the organization can hold. Roles
// belong to no department, so holding member.read anywhere is enough to see
// them.
func ListRoles(db *gorm.DB, actor *Actor) ([]RoleView, error) {
	if !actor.Holds("member.read") {
		return nil, ErrForbidden
	}
	roles, err := loadRoles(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	views := make([]RoleView, 0, len(roles))
	for i := range roles {
		views = append(views, viewRole(&roles[i]))
	}
	return views, nil
}

// GetPermissionCatalog returns what roles are made of: the primitives, the
// inherent powers and the role packs. Whoever may see the roles may see it.
func GetPermissionCatalog(actor *Actor) (*PermissionCatalog, error) {
	if !actor.Holds("member.read") {
		return nil, ErrForbidden
	}
	packs := make([]RolePackView, 0, len(orgmodel.RolePacks))
	for _, pack := range orgmodel.RolePacks {
		packs = append(packs, RolePackView{Key: pack.Key, Name: pack.Name, Scope: pack.Scope, Permissions: pack.Permissions})
	}
	return &PermissionCatalog{
		Primitives: orgmodel.Primitives,
		Powers:     orgmodel.InherentPowers,
		RolePacks:  packs,
	}, nil
}

// createRole stores a new custom role under a permit already granted.
func createRole(db *gorm.DB, permit *writePermit, role *orgmodel.OrgRole) (*RoleView, error) {
	var view RoleView
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := checkRoleName(tx, role.OrgId, role.Name, 0); err != nil {
			return err
		}
		if err := tx.Create(role).Error; err != nil {
			return err
		}
		view = viewRole(role)
		return permit.record(tx, orgmodel.AuditRoleCreate, orgmodel.AuditTargetRole, role.Id, nil, view)
	})
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// CreateRole adds a custom role built from the primitives.
func CreateRole(db *gorm.DB, actor *Actor, input RoleInput) (*RoleView, error) {
	permit, err := actor.permit(orgmodel.PowerRoles, wholeOrganization)
	if err != nil {
		return nil, err
	}
	role, err := normalizeRole(input)
	if err != nil {
		return nil, err
	}
	role.OrgId = actor.OrgId
	return createRole(db, permit, role)
}

// AdoptRolePack copies a platform role pack into the organization as a custom
// role of its own, which it can then change like any other. A blank name
// means the pack's own.
func AdoptRolePack(db *gorm.DB, actor *Actor, key string, name string) (*RoleView, error) {
	permit, err := actor.permit(orgmodel.PowerRoles, wholeOrganization)
	if err != nil {
		return nil, err
	}
	for _, pack := range orgmodel.RolePacks {
		if pack.Key != key {
			continue
		}
		if strings.TrimSpace(name) == "" {
			name = pack.Name
		}
		role, err := normalizeRole(RoleInput{Name: name, Scope: pack.Scope, Permissions: pack.Permissions})
		if err != nil {
			return nil, err
		}
		role.OrgId = actor.OrgId
		return createRole(db, permit, role)
	}
	return nil, ErrRolePackNotFound
}

// UpdateRole changes a custom role. What it grants changes for its holders at
// once: their next request is judged by the new permissions.
func UpdateRole(db *gorm.DB, actor *Actor, roleID int, input RoleInput) (*RoleView, error) {
	permit, err := actor.permit(orgmodel.PowerRoles, wholeOrganization)
	if err != nil {
		return nil, err
	}
	wanted, err := normalizeRole(input)
	if err != nil {
		return nil, err
	}
	var view RoleView
	err = db.Transaction(func(tx *gorm.DB) error {
		role, err := findCustomRole(tx, actor.OrgId, roleID)
		if err != nil {
			return err
		}
		if err := checkRoleName(tx, actor.OrgId, wanted.Name, role.Id); err != nil {
			return err
		}
		before := viewRole(role)
		if err := tx.Model(&orgmodel.OrgRole{}).Where("id = ? AND org_id = ?", role.Id, actor.OrgId).
			Updates(map[string]any{
				"name":        wanted.Name,
				"scope":       wanted.Scope,
				"permissions": wanted.Permissions,
			}).Error; err != nil {
			return err
		}
		// A role that stops having department scope leaves its holders
		// managing no department.
		if role.Scope == orgmodel.ScopeDept && wanted.Scope != orgmodel.ScopeDept {
			holders, err := roleHolders(tx, actor.OrgId, role.Id)
			if err != nil {
				return err
			}
			if err := clearExtraDepartments(tx, holders); err != nil {
				return err
			}
		}
		role.Name, role.Scope, role.Permissions = wanted.Name, wanted.Scope, wanted.Permissions
		view = viewRole(role)
		return permit.record(tx, orgmodel.AuditRoleUpdate, orgmodel.AuditTargetRole, role.Id, before, view)
	})
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// DeleteRole removes a custom role. Whoever pointed at it — the members who
// held it, and invite links not yet used — falls back to the staff role, the
// one that grants nothing (PRD D29).
func DeleteRole(db *gorm.DB, actor *Actor, roleID int) error {
	permit, err := actor.permit(orgmodel.PowerRoles, wholeOrganization)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		role, err := findCustomRole(tx, actor.OrgId, roleID)
		if err != nil {
			return err
		}
		staffRoleID, err := orgmodel.PresetRoleID(tx, orgmodel.RoleStaff)
		if err != nil {
			return err
		}
		holders, err := roleHolders(tx, actor.OrgId, role.Id)
		if err != nil {
			return err
		}
		// 🔴 Only org columns, never users.role.
		if err := tx.Model(&platformmodel.User{}).
			Where("org_id = ? AND role_id = ?", actor.OrgId, role.Id).
			Update("role_id", staffRoleID).Error; err != nil {
			return err
		}
		// Staff manage no department.
		if err := clearExtraDepartments(tx, holders); err != nil {
			return err
		}
		if err := tx.Model(&orgmodel.OrgInvite{}).
			Where("org_id = ? AND role_id = ?", actor.OrgId, role.Id).
			Update("role_id", staffRoleID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&orgmodel.OrgRole{}, role.Id).Error; err != nil {
			return err
		}
		return permit.record(tx, orgmodel.AuditRoleDelete, orgmodel.AuditTargetRole, role.Id,
			removedRole{RoleView: viewRole(role), MemberIds: holders}, nil)
	})
}
