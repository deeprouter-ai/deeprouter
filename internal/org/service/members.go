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

// ServiceAccountNameMaxLength is the longest service account name, in
// characters — the limit the platform puts on every display name.
const ServiceAccountNameMaxLength = 20

var (
	// ErrMemberNotFound means the user is not a member of this organization.
	ErrMemberNotFound = errors.New("member not found in this organization")
	// ErrRoleNotFound means the role is neither a preset nor one of this organization's.
	ErrRoleNotFound = errors.New("role not found in this organization")
	// ErrOwnerImmutable means the action would change the owner's role or
	// make someone else an owner. Ownership only moves by hand, through the
	// platform (PRD D19).
	ErrOwnerImmutable = errors.New("the owner's role cannot be changed and the owner role cannot be granted")
	// ErrOwnerOnly means the action appoints or dismisses an admin, which only
	// the owner may do (PRD D10).
	ErrOwnerOnly = errors.New("only the owner can appoint or dismiss admins")
	// ErrAdminDepartment means the action would put the owner or an admin in
	// a department other than the default one. They run the whole
	// organization, so they belong to no business unit (PRD D26).
	ErrAdminDepartment = errors.New("the owner and admins always belong to the default department")
	// ErrServiceAccountRole means the action would give a service account a role.
	ErrServiceAccountRole = errors.New("a service account's role cannot be changed")
	// ErrInvalidServiceAccountName means the service account name is blank or too long.
	ErrInvalidServiceAccountName = errors.New("service account name must be 1 to 20 characters")
)

// RoleView is a role as the management page offers it.
type RoleView struct {
	Id          int      `json:"id"`
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
	IsPreset    bool     `json:"is_preset"`
}

// MemberView is one member as the management page lists it. It carries no
// credentials and no balance.
type MemberView struct {
	Id           int    `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	RoleId       int    `json:"role_id"`
	Role         string `json:"role"`
	DepartmentId int    `json:"department_id"`
	IsOwner      bool   `json:"is_owner"`
	IsService    bool   `json:"is_service"`
}

// MemberPatch is a change to one member; a nil field is left as it is.
type MemberPatch struct {
	RoleId       *int `json:"role_id"`
	DepartmentId *int `json:"department_id"`
}

// isPreset reports whether a role is the platform preset with the given name.
// A custom role that happens to carry a preset's name is not that preset.
func isPreset(role *orgmodel.OrgRole, name string) bool {
	return role.OrgId == 0 && role.Name == name
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

// loadRoles returns the roles an organization may use, presets first.
func loadRoles(db *gorm.DB, orgID int) ([]orgmodel.OrgRole, error) {
	var roles []orgmodel.OrgRole
	err := db.Where("org_id IN ?", []int{0, orgID}).Order("org_id").Order("id").Find(&roles).Error
	return roles, err
}

// findMember loads the membership columns of one member of an organization.
func findMember(db *gorm.DB, orgID int, userID int) (*platformmodel.User, error) {
	var found []platformmodel.User
	if err := db.Select("id", "org_id", "role_id", "department_id", "is_service").
		Where("id = ? AND org_id = ?", userID, orgID).Limit(1).Find(&found).Error; err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, ErrMemberNotFound
	}
	return &found[0], nil
}

// ownerUserID returns the id of the user who owns an organization.
func ownerUserID(db *gorm.DB, orgID int) (int, error) {
	var org orgmodel.Organization
	if err := db.Select("owner_user_id").Where("id = ?", orgID).First(&org).Error; err != nil {
		return 0, err
	}
	return org.OwnerUserId, nil
}

// ListRoles returns the roles members of the organization can hold.
func ListRoles(db *gorm.DB, actor *Actor) ([]RoleView, error) {
	if err := actor.requireManager(); err != nil {
		return nil, err
	}
	roles, err := loadRoles(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	views := make([]RoleView, 0, len(roles))
	for _, role := range roles {
		views = append(views, RoleView{
			Id:          role.Id,
			Name:        role.Name,
			Scope:       role.Scope,
			Permissions: orgmodel.SplitPermissions(role.Permissions),
			IsPreset:    role.IsPreset,
		})
	}
	return views, nil
}

// ListMembers returns everyone in the organization, service accounts included.
func ListMembers(db *gorm.DB, actor *Actor) ([]MemberView, error) {
	if err := actor.requireManager(); err != nil {
		return nil, err
	}
	ownerID, err := ownerUserID(db, actor.OrgId)
	if err != nil {
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
	// The columns are named one by one: a member list must never carry a
	// password hash, an access token or a balance.
	var users []platformmodel.User
	if err := db.Select("id", "username", "display_name", "email", "role_id", "department_id", "is_service").
		Where("org_id = ?", actor.OrgId).Order("id").Find(&users).Error; err != nil {
		return nil, err
	}
	views := make([]MemberView, 0, len(users))
	for _, user := range users {
		views = append(views, MemberView{
			Id:           user.Id,
			Username:     user.Username,
			DisplayName:  user.DisplayName,
			Email:        user.Email,
			RoleId:       user.OrgRoleId,
			Role:         roleNames[user.OrgRoleId],
			DepartmentId: user.DepartmentId,
			IsOwner:      user.Id == ownerID,
			IsService:    user.IsService,
		})
	}
	return views, nil
}

// UpdateMember moves a member to another department, gives them another role,
// or both. Appointing and dismissing admins goes through here too.
func UpdateMember(db *gorm.DB, actor *Actor, memberID int, patch MemberPatch) error {
	if err := actor.requireManager(); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		member, err := findMember(tx, actor.OrgId, memberID)
		if err != nil {
			return err
		}
		updates := map[string]any{}
		roleID := member.OrgRoleId
		if patch.RoleId != nil && *patch.RoleId != member.OrgRoleId {
			if err := checkRoleChange(tx, actor, member, *patch.RoleId); err != nil {
				return err
			}
			roleID = *patch.RoleId
			updates["role_id"] = roleID
		}
		// The department follows from the role the member ends up with, so it
		// is decided after the role.
		departmentID, err := placeMember(tx, actor.OrgId, roleID, patch.DepartmentId)
		if err != nil {
			return err
		}
		if departmentID != 0 && departmentID != member.DepartmentId {
			updates["department_id"] = departmentID
		}
		if len(updates) == 0 {
			return nil
		}
		// 🔴 Only org columns, never users.role — an org admin is still a
		// common user on the platform.
		return tx.Model(&platformmodel.User{}).
			Where("id = ? AND org_id = ?", member.Id, actor.OrgId).Updates(updates).Error
	})
}

// runsOrganization reports whether roleID is the preset owner or admin role.
// Their holders run the whole organization, so they belong to its default
// department and to no business unit (PRD D26).
func runsOrganization(db *gorm.DB, roleID int) (bool, error) {
	for _, name := range []string{orgmodel.RoleOwner, orgmodel.RoleAdmin} {
		presetID, err := orgmodel.PresetRoleID(db, name)
		if err != nil {
			return false, err
		}
		if roleID == presetID {
			return true, nil
		}
	}
	return false, nil
}

// placeMember decides the department of a member who will hold roleID. The
// owner and admins go to the default department, and asking for any other is
// refused; everyone else goes where the request says. Zero means the member
// stays where they are.
func placeMember(tx *gorm.DB, orgID int, roleID int, requested *int) (int, error) {
	pinned, err := runsOrganization(tx, roleID)
	if err != nil {
		return 0, err
	}
	if pinned {
		defaultID, err := defaultDepartmentID(tx, orgID)
		if err != nil {
			return 0, err
		}
		if requested != nil && *requested != defaultID {
			return 0, ErrAdminDepartment
		}
		return defaultID, nil
	}
	if requested == nil {
		return 0, nil
	}
	department, err := findDepartment(tx, orgID, *requested)
	if err != nil {
		return 0, err
	}
	return department.Id, nil
}

// checkRoleChange decides whether actor may give member the role newRoleID.
func checkRoleChange(tx *gorm.DB, actor *Actor, member *platformmodel.User, newRoleID int) error {
	ownerID, err := ownerUserID(tx, actor.OrgId)
	if err != nil {
		return err
	}
	if member.Id == ownerID {
		return ErrOwnerImmutable
	}
	if member.IsService {
		return ErrServiceAccountRole
	}
	newRole, err := findRole(tx, actor.OrgId, newRoleID)
	if err != nil {
		return err
	}
	if isPreset(newRole, orgmodel.RoleOwner) {
		return ErrOwnerImmutable
	}
	adminRoleID, err := orgmodel.PresetRoleID(tx, orgmodel.RoleAdmin)
	if err != nil {
		return err
	}
	appointsOrDismissesAdmin := newRole.Id == adminRoleID || member.OrgRoleId == adminRoleID
	if appointsOrDismissesAdmin && !actor.IsOwner {
		return ErrOwnerOnly
	}
	return nil
}

// CreateServiceAccount adds a member that is not a person: a CI pipeline or a
// bot that holds keys and shows up in usage, and can never sign in. A zero
// departmentID puts it in the default department.
func CreateServiceAccount(db *gorm.DB, actor *Actor, rawName string, departmentID int) (*MemberView, error) {
	if err := actor.requireManager(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(rawName)
	if name == "" || utf8.RuneCountInString(name) > ServiceAccountNameMaxLength {
		return nil, ErrInvalidServiceAccountName
	}
	var view *MemberView
	err := db.Transaction(func(tx *gorm.DB) error {
		resolvedDepartmentID, err := resolveDepartment(tx, actor.OrgId, departmentID)
		if err != nil {
			return err
		}
		staffRoleID, err := orgmodel.PresetRoleID(tx, orgmodel.RoleStaff)
		if err != nil {
			return err
		}
		// Written directly rather than through User.Insert, which hands every
		// new account the sign-up trial credit — a company could mint credit
		// by creating service accounts. No password, no email, no third-party
		// binding and no access token: there is nothing to sign in with.
		// Usernames are unique across the whole platform, so the name the
		// company chose goes into display_name and the username is generated.
		account := platformmodel.User{
			Username:     "svc-" + strings.ToLower(common.GetRandomString(12)),
			DisplayName:  name,
			AffCode:      common.GetRandomString(16),
			OrgId:        actor.OrgId,
			OrgRoleId:    staffRoleID,
			DepartmentId: resolvedDepartmentID,
			IsService:    true,
		}
		if err := tx.Create(&account).Error; err != nil {
			return err
		}
		view = &MemberView{
			Id:           account.Id,
			Username:     account.Username,
			DisplayName:  account.DisplayName,
			RoleId:       staffRoleID,
			Role:         orgmodel.RoleStaff,
			DepartmentId: resolvedDepartmentID,
			IsService:    true,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// OwnsOrganization reports whether userID is the owner of an organization.
func OwnsOrganization(db *gorm.DB, userID int) (bool, error) {
	var n int64
	err := db.Model(&orgmodel.Organization{}).Where("owner_user_id = ?", userID).Count(&n).Error
	return n > 0, err
}
