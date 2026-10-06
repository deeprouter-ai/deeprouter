package model

import (
	"slices"
	"strings"
)

// Subject is the member asking to do something: what their role grants, how
// far it reaches, and whether they are the owner or an admin.
type Subject struct {
	UserId      int
	IsOwner     bool
	IsAdmin     bool     // holds the preset admin role
	Scope       string   // ScopeOrg, ScopeDept or ScopeSelf
	Permissions []string // the primitives of their role
	Departments []int    // the departments they manage; only a ScopeDept role has any
}

// Target is what an action is on: the department and the member it belongs
// to. The zero Target is the organization as a whole, which only a role that
// reaches the whole organization can act on.
type Target struct {
	DepartmentId int
	UserId       int
}

// Can is the permission engine (PRD §7.5): the one place that decides whether
// a member may do something in their organization. It asks the two questions
// of PRD §2 — does the role grant the action, and is the target within the
// role's reach — and nothing else, so it needs no database and the whole
// permission matrix can be tested as a truth table.
//
// action is a primitive or the name of an inherent power. Anything else is
// refused.
func Can(subject Subject, action string, target Target) bool {
	if power, ok := inherentPower(action); ok {
		return subject.IsOwner || (subject.IsAdmin && !power.OwnerOnly)
	}
	// Whatever their role, a member sees what is their own: the keys assigned
	// to them and their own usage (PRD §2, the staff row).
	if target.UserId != 0 && target.UserId == subject.UserId && (action == "key.read" || action == "usage.read") {
		return true
	}
	everywhere, departments := Reach(subject, action)
	return everywhere || (target.DepartmentId != 0 && slices.Contains(departments, target.DepartmentId))
}

// Reach returns where the subject's role lets them use a primitive: everywhere
// in the organization, or only in the returned departments — none at all when
// the role does not grant it. List endpoints filter by it; Can judges a single
// target by it.
func Reach(subject Subject, primitive string) (everywhere bool, departments []int) {
	if !subject.Holds(primitive) {
		return false, nil
	}
	switch subject.Scope {
	case ScopeOrg:
		return true, nil
	case ScopeDept:
		return false, subject.Departments
	default:
		return false, nil
	}
}

// Holds reports whether the subject's role grants a primitive, wherever it
// reaches. A write brings the read of the same resource with it (PRD §2):
// whoever may assign keys can see the key list.
func (s Subject) Holds(primitive string) bool {
	if slices.Contains(s.Permissions, primitive) {
		return true
	}
	resource, action, _ := strings.Cut(primitive, ".")
	if action != "read" {
		return false
	}
	for _, held := range s.Permissions {
		if strings.HasPrefix(held, resource+".") {
			return true
		}
	}
	return false
}

// inherentPower looks an action up among the inherent powers.
func inherentPower(action string) (InherentPower, bool) {
	for _, power := range InherentPowers {
		if power.Name == action {
			return power, true
		}
	}
	return InherentPower{}, false
}
