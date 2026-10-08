package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	// ErrOwnerNotRemovable means the action would take the owner out of their
	// own organization. Their account is the company wallet (PRD D10, D19).
	ErrOwnerNotRemovable = errors.New("the owner cannot be removed from the organization")
	// ErrRemoveSelf means a member tried to remove themselves. Leaving is
	// somebody else's to do, so that the audit log always names who did it.
	ErrRemoveSelf = errors.New("a member cannot remove themselves from the organization")
)

// removedMember is what the audit log keeps of a member who was removed. It
// carries names as well as ids: the account is gone, and the record has to go
// on saying who it was.
type removedMember struct {
	Name         string `json:"name"`
	Username     string `json:"username"`
	RoleId       int    `json:"role_id"`
	Role         string `json:"role"`
	DepartmentId int    `json:"department_id"`
	Department   string `json:"department"`
	IsService    bool   `json:"is_service,omitempty"`
	// KeyIds are the keys that were taken back from them; each has a
	// key.reclaim record of its own.
	KeyIds []int `json:"key_ids"`
}

// RemoveMember takes a member — a person or a service account — out of the
// organization for good, and reports how many keys it took back from them.
//
// In one transaction: every organization key they hold is reclaimed (frozen,
// given a new value, parked under the owner — see assign.go), the departments
// added for them to manage are cleared, and their account is deleted. So the
// values in their tools are dead before this returns, they can no longer sign
// in, and what they spent stays in the usage records under their name.
//
// It takes member.remove reaching the member. On top of that: nobody removes
// the owner, nobody removes themselves, and only the owner removes an admin —
// which keeps a department-scoped role that manages the default department,
// where the owner and the admins sit (PRD D26), from reaching them.
func RemoveMember(db *gorm.DB, actor *Actor, memberID int) (int, error) {
	// Whoever may remove nobody learns nothing here about who is in the
	// organization.
	if !actor.Holds("member.remove") {
		return 0, ErrForbidden
	}
	member, err := findMember(db, actor.OrgId, memberID)
	if err != nil {
		return 0, err
	}
	permit, err := actor.permit("member.remove", orgmodel.Target{DepartmentId: member.DepartmentId, UserId: member.Id})
	if err != nil {
		return 0, err
	}
	owner, err := ownerAsHolder(db, actor.OrgId)
	if err != nil {
		return 0, err
	}
	if member.Id == owner.Id {
		return 0, ErrOwnerNotRemovable
	}
	if member.Id == actor.UserId {
		return 0, ErrRemoveSelf
	}
	role, err := findRole(db, actor.OrgId, member.OrgRoleId)
	if err != nil {
		return 0, err
	}
	// Removing an admin dismisses one, and that is the owner's alone to do
	// (PRD D10).
	if isPreset(role, orgmodel.RoleAdmin) && !actor.can(orgmodel.PowerAdmins, wholeOrganization) {
		return 0, ErrOwnerOnly
	}

	var deadValues []string
	err = db.Transaction(func(tx *gorm.DB) error {
		var keys []platformmodel.Token
		if err := tx.Where("user_id = ? AND org_id = ?", member.Id, actor.OrgId).Order("id").Find(&keys).Error; err != nil {
			return err
		}
		holders, err := keyHolders(tx, actor.OrgId, []int{member.Id})
		if err != nil {
			return err
		}
		leaving := holders[member.Id]
		removed := removedMember{
			Name:         leaving.Name,
			Username:     member.Username,
			RoleId:       role.Id,
			Role:         role.Name,
			DepartmentId: leaving.DepartmentId,
			Department:   leaving.Department,
			IsService:    member.IsService,
			KeyIds:       []int{},
		}
		for i := range keys {
			if _, err := handOver(tx, permit, orgmodel.AuditKeyReclaim, &keys[i], leaving, owner, common.TokenStatusDisabled); err != nil {
				return err
			}
			deadValues = append(deadValues, keys[i].Key)
			removed.KeyIds = append(removed.KeyIds, keys[i].Id)
		}
		if err := clearExtraDepartments(tx, []int{member.Id}); err != nil {
			return err
		}
		// A soft delete, like every other deleted account on the platform: the
		// row stays, so the audit log and the usage records can go on naming
		// them — and so do its username and email, which nobody can sign up
		// with again.
		if err := tx.Where("id = ? AND org_id = ?", member.Id, actor.OrgId).Delete(&platformmodel.User{}).Error; err != nil {
			return err
		}
		return permit.record(tx, orgmodel.AuditMemberRemove, orgmodel.AuditTargetMember, member.Id, removed, nil)
	})
	if err != nil {
		return 0, err
	}
	for _, value := range deadValues {
		forgetKeyValue(value)
	}
	// The gateway caches accounts as well as keys. Nothing of theirs is left to
	// call with, so a stale entry opens nothing; it is dropped all the same.
	if err := platformmodel.InvalidateUserCache(member.Id); err != nil {
		common.SysError("org: failed to drop a removed member from the user cache: " + err.Error())
	}
	return len(deadValues), nil
}
