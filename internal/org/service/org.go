// Package service holds the Enterprise Org logic that touches platform tables
// (meta-repo docs/enterprise-org-prd.md). The org tables themselves live in
// ../model.
//
// 🔴 Nothing in this package may write users.role. Org roles are a separate
// axis (users.role_id → org_roles); every org member, the owner included,
// stays a common user on the platform (PRD §7.3, red line 1).
package service

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// NameMaxLength is the longest organization name, in characters.
const NameMaxLength = 64

var (
	// ErrInvalidName means the organization name is blank or too long.
	ErrInvalidName = errors.New("organization name must be 1 to 64 characters")
	// ErrNotPersonalAccount means the user cannot found an organization: they
	// already belong to one (PRD D20: one person, one organization) or do not
	// exist.
	ErrNotPersonalAccount = errors.New("only a personal account can create an organization")
)

// NormalizeName trims an organization name and checks its length.
func NormalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > NameMaxLength {
		return "", ErrInvalidName
	}
	return name, nil
}

// CreateForOwnerTx creates an organization with its preset departments and
// makes userID its owner, inside the caller's transaction so it commits or
// rolls back with the sign-up. lang picks the language the preset departments
// are named in.
func CreateForOwnerTx(tx *gorm.DB, userID int, rawName string, lang string) (*orgmodel.Organization, error) {
	name, err := NormalizeName(rawName)
	if err != nil {
		return nil, err
	}
	ownerRoleID, err := orgmodel.PresetRoleID(tx, orgmodel.RoleOwner)
	if err != nil {
		return nil, err
	}
	// Checked before the insert so the caller gets this error rather than the
	// unique-index violation on organizations.owner_user_id.
	var personal int64
	if err := tx.Model(&platformmodel.User{}).
		Where("id = ? AND org_id = ?", userID, 0).Count(&personal).Error; err != nil {
		return nil, err
	}
	if personal != 1 {
		return nil, ErrNotPersonalAccount
	}
	org := &orgmodel.Organization{
		Name:        name,
		OwnerUserId: userID,
		CreatedTime: common.GetTimestamp(),
	}
	if err := tx.Create(org).Error; err != nil {
		return nil, err
	}
	defaultDepartmentID, err := seedDepartmentsTx(tx, org.Id, lang)
	if err != nil {
		return nil, err
	}
	// Only the org columns are written — never users.role. The org_id = 0
	// guard is repeated here so that losing a race with another sign-up for
	// the same user cannot move them between organizations.
	result := tx.Model(&platformmodel.User{}).
		Where("id = ? AND org_id = ?", userID, 0).
		Updates(map[string]any{"org_id": org.Id, "role_id": ownerRoleID, "department_id": defaultDepartmentID})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrNotPersonalAccount
	}
	return org, nil
}

// Membership is what a member can see about their own place in an organization.
type Membership struct {
	OrgId        int      `json:"org_id"`
	OrgName      string   `json:"org_name"`
	IsOwner      bool     `json:"is_owner"`
	IsAdmin      bool     `json:"is_admin"` // holds the preset admin role
	Role         string   `json:"role"`
	RoleScope    string   `json:"role_scope"`
	Permissions  []string `json:"permissions"`
	DepartmentId int      `json:"department_id"`
}

// GetMembership returns the organization and role of userID, or nil for a
// personal account.
func GetMembership(db *gorm.DB, userID int) (*Membership, error) {
	var user platformmodel.User
	if err := db.Select("id", "org_id", "role_id", "department_id").
		Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	if user.OrgId == 0 {
		return nil, nil
	}
	var org orgmodel.Organization
	if err := db.Where("id = ?", user.OrgId).First(&org).Error; err != nil {
		return nil, err
	}
	// A role is either a platform preset (org_id 0) or this organization's own;
	// anything else would be another company's custom role.
	var role orgmodel.OrgRole
	if err := db.Where("id = ? AND org_id IN ?", user.OrgRoleId, []int{0, user.OrgId}).
		First(&role).Error; err != nil {
		return nil, err
	}
	return &Membership{
		OrgId:        org.Id,
		OrgName:      org.Name,
		IsOwner:      org.OwnerUserId == user.Id,
		IsAdmin:      isPreset(&role, orgmodel.RoleAdmin),
		Role:         role.Name,
		RoleScope:    role.Scope,
		Permissions:  orgmodel.SplitPermissions(role.Permissions),
		DepartmentId: user.DepartmentId,
	}, nil
}
