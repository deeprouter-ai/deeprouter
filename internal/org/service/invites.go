package service

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// InviteValidity is how long an invite link keeps admitting new members.
const InviteValidity = 7 * 24 * time.Hour

// inviteCodeLength is the length of the random part of an invite link. The
// code is the only credential a newcomer presents, so it is long enough that
// guessing one is not a plan.
const inviteCodeLength = 32

// ErrInviteNotFound means the invite code is unknown, expired or revoked. The
// three are deliberately one answer: a stranger probing codes learns nothing
// from the difference.
var ErrInviteNotFound = errors.New("invite not found, expired or revoked")

// InviteView is an invite link as the management page lists it.
type InviteView struct {
	Id           int    `json:"id"`
	Code         string `json:"code"`
	RoleId       int    `json:"role_id"`
	Role         string `json:"role"`
	DepartmentId int    `json:"department_id"`
	ExpiresTime  int64  `json:"expires_time"`
}

// InvitePreview is what the sign-up page shows someone holding an invite link
// before they join.
type InvitePreview struct {
	OrgName    string `json:"org_name"`
	Role       string `json:"role"`
	Department string `json:"department"`
}

// findLiveInvite loads an invite that can still be used.
func findLiveInvite(db *gorm.DB, code string) (*orgmodel.OrgInvite, error) {
	if code == "" {
		return nil, ErrInviteNotFound
	}
	var found []orgmodel.OrgInvite
	if err := db.Where("code = ? AND expires_time > ?", code, common.GetTimestamp()).
		Limit(1).Find(&found).Error; err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, ErrInviteNotFound
	}
	return &found[0], nil
}

// CreateInvite issues an invite link that admits new members with a fixed
// role and department. The link can be used by any number of people until it
// expires or is revoked, so one link serves a whole team. A zero departmentID
// means the default department.
func CreateInvite(db *gorm.DB, actor *Actor, roleID int, departmentID int) (*InviteView, error) {
	if err := actor.requireManager(); err != nil {
		return nil, err
	}
	role, err := findRole(db, actor.OrgId, roleID)
	if err != nil {
		return nil, err
	}
	if isPreset(role, orgmodel.RoleOwner) {
		return nil, ErrOwnerImmutable
	}
	// An invite that makes admins is an appointment, and appointing admins is
	// the owner's alone.
	if isPreset(role, orgmodel.RoleAdmin) && !actor.IsOwner {
		return nil, ErrOwnerOnly
	}
	resolvedDepartmentID, err := resolveDepartment(db, actor.OrgId, departmentID)
	if err != nil {
		return nil, err
	}
	// Admins sit in the default department, so an invite cannot bring one in
	// anywhere else (PRD D26).
	if isPreset(role, orgmodel.RoleAdmin) {
		defaultID, err := defaultDepartmentID(db, actor.OrgId)
		if err != nil {
			return nil, err
		}
		if resolvedDepartmentID != defaultID {
			return nil, ErrAdminDepartment
		}
	}
	code, err := common.GenerateRandomCharsKey(inviteCodeLength)
	if err != nil {
		return nil, err
	}
	invite := orgmodel.OrgInvite{
		OrgId:        actor.OrgId,
		Code:         code,
		RoleId:       role.Id,
		DepartmentId: resolvedDepartmentID,
		ExpiresTime:  time.Now().Add(InviteValidity).Unix(),
	}
	if err := db.Create(&invite).Error; err != nil {
		return nil, err
	}
	return &InviteView{
		Id:           invite.Id,
		Code:         invite.Code,
		RoleId:       invite.RoleId,
		Role:         role.Name,
		DepartmentId: invite.DepartmentId,
		ExpiresTime:  invite.ExpiresTime,
	}, nil
}

// ListInvites returns the organization's invite links that can still be used,
// newest first.
func ListInvites(db *gorm.DB, actor *Actor) ([]InviteView, error) {
	if err := actor.requireManager(); err != nil {
		return nil, err
	}
	var invites []orgmodel.OrgInvite
	if err := db.Where("org_id = ? AND expires_time > ?", actor.OrgId, common.GetTimestamp()).
		Order("id DESC").Find(&invites).Error; err != nil {
		return nil, err
	}
	roles, err := loadRoles(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	roleNames := make(map[int]string, len(roles))
	for _, role := range roles {
		roleNames[role.Id] = role.Name
	}
	views := make([]InviteView, 0, len(invites))
	for _, invite := range invites {
		views = append(views, InviteView{
			Id:           invite.Id,
			Code:         invite.Code,
			RoleId:       invite.RoleId,
			Role:         roleNames[invite.RoleId],
			DepartmentId: invite.DepartmentId,
			ExpiresTime:  invite.ExpiresTime,
		})
	}
	return views, nil
}

// RevokeInvite makes an invite link stop working at once.
func RevokeInvite(db *gorm.DB, actor *Actor, inviteID int) error {
	if err := actor.requireManager(); err != nil {
		return err
	}
	result := db.Where("id = ? AND org_id = ?", inviteID, actor.OrgId).Delete(&orgmodel.OrgInvite{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrInviteNotFound
	}
	return nil
}

// LookupInvite tells the holder of an invite code what joining with it means.
// It is the one organization read that needs no sign-in: the reader has no
// account yet.
func LookupInvite(db *gorm.DB, code string) (*InvitePreview, error) {
	invite, err := findLiveInvite(db, code)
	if err != nil {
		return nil, err
	}
	var org orgmodel.Organization
	if err := db.Select("name").Where("id = ?", invite.OrgId).First(&org).Error; err != nil {
		return nil, err
	}
	role, err := findRole(db, invite.OrgId, invite.RoleId)
	if err != nil {
		return nil, err
	}
	department, err := findDepartment(db, invite.OrgId, invite.DepartmentId)
	if err != nil {
		return nil, err
	}
	return &InvitePreview{OrgName: org.Name, Role: role.Name, Department: department.Name}, nil
}

// JoinByInviteTx puts a freshly created personal account into the
// organization an invite code belongs to, with the invite's role and
// department. It runs inside the sign-up transaction, so an account that asked
// to join a company is never left behind without one. The invite is kept: it
// serves everyone who holds the link.
func JoinByInviteTx(tx *gorm.DB, userID int, code string) error {
	invite, err := findLiveInvite(tx, code)
	if err != nil {
		return err
	}
	// 🔴 Only the org columns are written — never users.role. The org_id = 0
	// guard keeps an account that already belongs somewhere where it is
	// (PRD D20: one person, one organization).
	result := tx.Model(&platformmodel.User{}).
		Where("id = ? AND org_id = ?", userID, 0).
		Updates(map[string]any{
			"org_id":        invite.OrgId,
			"role_id":       invite.RoleId,
			"department_id": invite.DepartmentId,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrNotPersonalAccount
	}
	return nil
}
