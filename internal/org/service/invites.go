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

// inviteRecord is what the audit log keeps of an invite link. It leaves the
// code out on purpose: the code is a credential, and whoever may read the
// audit log is not thereby entitled to let people into the organization.
type inviteRecord struct {
	RoleId       int    `json:"role_id"`
	Role         string `json:"role"`
	DepartmentId int    `json:"department_id"`
	Department   string `json:"department"`
	ExpiresTime  int64  `json:"expires_time,omitempty"`
}

// joinRecord is what the audit log keeps of someone joining: the link they
// used and what it gave them.
type joinRecord struct {
	InviteId int `json:"invite_id"`
	inviteRecord
}

// describeInvite renders an invite link for the audit log.
func describeInvite(db *gorm.DB, invite *orgmodel.OrgInvite, role *orgmodel.OrgRole) inviteRecord {
	return inviteRecord{
		RoleId:       role.Id,
		Role:         role.Name,
		DepartmentId: invite.DepartmentId,
		Department:   describeDepartment(db, invite.OrgId, invite.DepartmentId).Department,
		ExpiresTime:  invite.ExpiresTime,
	}
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

// mayIssue decides whether actor may issue an invite link for a role into a
// department. The same answer decides which links they see and may revoke: a
// link is a way in, so showing one to somebody who could not have made it
// would hand them that way in.
//
//   - It takes member.invite reaching the department.
//   - Nobody gives what they do not have (PRD §2): only whoever assigns roles
//     — the owner and admins — invites into a role other than staff.
//   - A link that makes admins is an appointment, the owner's alone (PRD D10),
//     and no link makes owners.
func mayIssue(actor *Actor, role *orgmodel.OrgRole, departmentID int) error {
	if !actor.can("member.invite", orgmodel.Target{DepartmentId: departmentID}) {
		return ErrForbidden
	}
	if isPreset(role, orgmodel.RoleOwner) {
		return ErrOwnerImmutable
	}
	if isPreset(role, orgmodel.RoleAdmin) && !actor.can(orgmodel.PowerAdmins, wholeOrganization) {
		return ErrOwnerOnly
	}
	if !isPreset(role, orgmodel.RoleStaff) && !actor.can(orgmodel.PowerRoles, wholeOrganization) {
		return ErrForbidden
	}
	return nil
}

// CreateInvite issues an invite link that admits new members with a fixed
// role and department. The link can be used by any number of people until it
// expires or is revoked, so one link serves a whole team. A zero departmentID
// means the default department.
func CreateInvite(db *gorm.DB, actor *Actor, roleID int, departmentID int) (*InviteView, error) {
	// Whoever may invite nowhere learns nothing about roles or departments.
	if !actor.Holds("member.invite") {
		return nil, ErrForbidden
	}
	role, err := findRole(db, actor.OrgId, roleID)
	if err != nil {
		return nil, err
	}
	resolvedDepartmentID, err := resolveDepartment(db, actor.OrgId, departmentID)
	if err != nil {
		return nil, err
	}
	if err := mayIssue(actor, role, resolvedDepartmentID); err != nil {
		return nil, err
	}
	permit, err := actor.permit("member.invite", orgmodel.Target{DepartmentId: resolvedDepartmentID})
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
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&invite).Error; err != nil {
			return err
		}
		return permit.record(tx, orgmodel.AuditMemberInvite, orgmodel.AuditTargetInvite, invite.Id,
			nil, describeInvite(tx, &invite, role))
	})
	if err != nil {
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

// ListInvites returns the usable invite links the actor could have issued
// themselves, newest first: every link for the owner, only the staff links
// into their departments for a manager.
func ListInvites(db *gorm.DB, actor *Actor) ([]InviteView, error) {
	if !actor.Holds("member.invite") {
		return nil, ErrForbidden
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
	roleByID := make(map[int]*orgmodel.OrgRole, len(roles))
	for i := range roles {
		roleByID[roles[i].Id] = &roles[i]
	}
	views := make([]InviteView, 0, len(invites))
	for _, invite := range invites {
		role, known := roleByID[invite.RoleId]
		if !known || mayIssue(actor, role, invite.DepartmentId) != nil {
			continue
		}
		views = append(views, InviteView{
			Id:           invite.Id,
			Code:         invite.Code,
			RoleId:       invite.RoleId,
			Role:         role.Name,
			DepartmentId: invite.DepartmentId,
			ExpiresTime:  invite.ExpiresTime,
		})
	}
	return views, nil
}

// RevokeInvite makes an invite link stop working at once. Whoever could have
// issued a link may revoke it.
func RevokeInvite(db *gorm.DB, actor *Actor, inviteID int) error {
	if !actor.Holds("member.invite") {
		return ErrForbidden
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var found []orgmodel.OrgInvite
		if err := tx.Where("id = ? AND org_id = ?", inviteID, actor.OrgId).
			Limit(1).Find(&found).Error; err != nil {
			return err
		}
		if len(found) == 0 {
			return ErrInviteNotFound
		}
		invite := &found[0]
		role, err := findRole(tx, actor.OrgId, invite.RoleId)
		if err != nil {
			return err
		}
		if err := mayIssue(actor, role, invite.DepartmentId); err != nil {
			return err
		}
		permit, err := actor.permit("member.invite", orgmodel.Target{DepartmentId: invite.DepartmentId})
		if err != nil {
			return err
		}
		if err := tx.Delete(&orgmodel.OrgInvite{}, invite.Id).Error; err != nil {
			return err
		}
		return permit.record(tx, orgmodel.AuditInviteRevoke, orgmodel.AuditTargetInvite, invite.Id,
			describeInvite(tx, invite, role), nil)
	})
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
// serves everyone who holds the link — which is why each use of it goes into
// the audit log, with the address it came from.
func JoinByInviteTx(tx *gorm.DB, userID int, code string, ip string) error {
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
	role, err := findRole(tx, invite.OrgId, invite.RoleId)
	if err != nil {
		return err
	}
	// Joining is nobody's management action, so it is recorded directly: the
	// newcomer is both who acted and who the record is about.
	joined := describeInvite(tx, invite, role)
	joined.ExpiresTime = 0
	return RecordAudit(tx, AuditEntry{
		OrgId:       invite.OrgId,
		ActorUserId: userID,
		Ip:          ip,
		Action:      orgmodel.AuditMemberJoin,
		TargetType:  orgmodel.AuditTargetMember,
		TargetId:    userID,
		After:       joinRecord{InviteId: invite.Id, inviteRecord: joined},
	})
}
