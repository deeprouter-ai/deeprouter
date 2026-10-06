package service

import (
	"errors"

	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"gorm.io/gorm"
)

var (
	// ErrNotMember means the caller is a personal account, not a member of any
	// organization.
	ErrNotMember = errors.New("not a member of an organization")
	// ErrForbidden means the caller's organization role does not allow the
	// action, or does not reach what it was aimed at.
	ErrForbidden = errors.New("organization role does not allow this action")
)

// wholeOrganization is the target of an action that is not about one
// department or one member: only a role reaching the whole organization, or an
// inherent power, can act on it.
var wholeOrganization = orgmodel.Target{}

// Actor is the member performing a management action on their organization:
// who they are to the permission engine, plus what the audit log records about
// the request.
type Actor struct {
	orgmodel.Subject
	OrgId int
	// Ip is the address the request came from. The handler fills it in.
	Ip string
}

// LoadActor resolves the caller's place in their organization. It reads the
// database on every call and nothing is cached, so a change to a member's role
// — or to what that role grants — takes effect on their next request without
// anyone signing in again.
func LoadActor(db *gorm.DB, userID int) (*Actor, error) {
	membership, err := GetMembership(db, userID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, ErrNotMember
	}
	return &Actor{
		Subject: orgmodel.Subject{
			UserId:      userID,
			IsOwner:     membership.IsOwner,
			IsAdmin:     membership.IsAdmin,
			Scope:       membership.RoleScope,
			Permissions: membership.Permissions,
			Departments: membership.ManagedDepartmentIds,
		},
		OrgId: membership.OrgId,
	}, nil
}

// can reports whether the actor may do action on target. Every permission
// question of this package ends here, in orgmodel.Can.
func (a *Actor) can(action string, target orgmodel.Target) bool {
	return orgmodel.Can(a.Subject, action, target)
}

// allow is the gate of a read: nil when the actor may, ErrForbidden when not.
func (a *Actor) allow(action string, target orgmodel.Target) error {
	if !a.can(action, target) {
		return ErrForbidden
	}
	return nil
}

// reach returns where the actor may read with a primitive — the whole
// organization, or only the listed departments — for lists that show a
// department-scoped member their part. Reaching nowhere is ErrForbidden.
func (a *Actor) reach(primitive string) (everywhere bool, departments []int, err error) {
	everywhere, departments = orgmodel.Reach(a.Subject, primitive)
	if !everywhere && len(departments) == 0 {
		return false, nil, ErrForbidden
	}
	return everywhere, departments, nil
}

// writePermit is leave to make one change. It ties the audit log to the
// permission check (PRD §7.5): a write is allowed by receiving a permit, and
// the audit record is written through it. A function that asks for a permit
// and then forgets the record does not build — Go rejects the unused variable
// — so the gap cannot open by oversight, only by someone discarding the permit
// on purpose.
type writePermit struct {
	actor *Actor
}

// permit is the gate of a write: leave to do action on target, or ErrForbidden.
func (a *Actor) permit(action string, target orgmodel.Target) (*writePermit, error) {
	if !a.can(action, target) {
		return nil, ErrForbidden
	}
	return &writePermit{actor: a}, nil
}

// record writes what was done under the permit to the audit log. It takes the
// transaction of the change itself, so the two commit or roll back together.
func (p *writePermit) record(tx *gorm.DB, action string, targetType string, targetID int, before any, after any) error {
	return RecordAudit(tx, AuditEntry{
		OrgId:       p.actor.OrgId,
		ActorUserId: p.actor.UserId,
		Ip:          p.actor.Ip,
		Action:      action,
		TargetType:  targetType,
		TargetId:    targetID,
		Before:      before,
		After:       after,
	})
}
