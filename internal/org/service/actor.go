package service

import (
	"errors"

	"gorm.io/gorm"
)

var (
	// ErrNotMember means the caller is a personal account, not a member of any
	// organization.
	ErrNotMember = errors.New("not a member of an organization")
	// ErrForbidden means the caller's organization role does not allow the action.
	ErrForbidden = errors.New("organization role does not allow this action")
)

// Actor is the member performing a management action on their organization.
type Actor struct {
	UserId  int
	OrgId   int
	IsOwner bool
	IsAdmin bool // holds the preset admin role
}

// LoadActor resolves the caller's place in their organization. It reads the
// database on every call, so a role change takes effect on the next request
// without anyone signing in again.
func LoadActor(db *gorm.DB, userID int) (*Actor, error) {
	membership, err := GetMembership(db, userID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, ErrNotMember
	}
	return &Actor{
		UserId:  userID,
		OrgId:   membership.OrgId,
		IsOwner: membership.IsOwner,
		IsAdmin: membership.IsAdmin,
	}, nil
}

// requireManager is the whole permission model of this card (P3): the owner
// and the admins run the organization, nobody else may. P4 replaces every call
// with Can(), which also knows primitives and department scope.
func (a *Actor) requireManager() error {
	if !a.IsOwner && !a.IsAdmin {
		return ErrForbidden
	}
	return nil
}
