package service

import (
	"errors"
	"slices"
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
	// ErrNotDepartmentScoped means the action would give departments to manage
	// to a member whose role does not have department scope (PRD D28).
	ErrNotDepartmentScoped = errors.New("only a member whose role has department scope manages departments")
	// ErrServiceAccountRole means the action would give a service account a role.
	ErrServiceAccountRole = errors.New("a service account's role cannot be changed")
	// ErrInvalidServiceAccountName means the service account name is blank or too long.
	ErrInvalidServiceAccountName = errors.New("service account name must be 1 to 20 characters")
)

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
	// ManagedDepartmentIds is where a member with a department-scoped role
	// reaches: their own department first, then the ones added for them.
	// Empty for everyone else.
	ManagedDepartmentIds []int `json:"managed_department_ids"`
	// KeyCount is how many of the organization's keys they hold — what removing
	// them would take back.
	KeyCount int `json:"key_count"`
}

// MemberPatch is a change to one member; a nil field is left as it is.
type MemberPatch struct {
	RoleId       *int `json:"role_id"`
	DepartmentId *int `json:"department_id"`
	// ManagedDepartmentIds replaces the departments the member manages. The
	// department they belong to is always one of them, listed or not.
	ManagedDepartmentIds *[]int `json:"managed_department_ids"`
}

// memberRole and the two types after it are what the audit log keeps of a
// change to a member. They carry the names as well as the ids: a record must
// still read right after the role or department it mentions is gone.
type memberRole struct {
	RoleId int    `json:"role_id"`
	Role   string `json:"role"`
}

// memberDepartment is the department side of a member.move record.
type memberDepartment struct {
	DepartmentId int    `json:"department_id"`
	Department   string `json:"department"`
}

// memberManages is the department list of a member.manages record.
type memberManages struct {
	DepartmentIds []int `json:"department_ids"`
}

// findMember loads one member of an organization: what they are called and
// the membership columns, nothing else.
func findMember(db *gorm.DB, orgID int, userID int) (*platformmodel.User, error) {
	var found []platformmodel.User
	if err := db.Select("id", "username", "display_name", "org_id", "role_id", "department_id", "is_service").
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

// ListMembers returns the members the actor may see, service accounts
// included: everyone for a role that reaches the whole organization, the
// people of the departments they manage for a department-scoped one.
func ListMembers(db *gorm.DB, actor *Actor) ([]MemberView, error) {
	everywhere, departments, err := actor.reach("member.read")
	if err != nil {
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
	roleByID := make(map[int]orgmodel.OrgRole, len(roles))
	for _, role := range roles {
		roleByID[role.Id] = role
	}
	// The columns are named one by one: a member list must never carry a
	// password hash, an access token or a balance.
	query := db.Select("id", "username", "display_name", "email", "role_id", "department_id", "is_service").
		Where("org_id = ?", actor.OrgId)
	if !everywhere {
		query = query.Where("department_id IN ?", departments)
	}
	var users []platformmodel.User
	if err := query.Order("id").Find(&users).Error; err != nil {
		return nil, err
	}
	var scoped []int
	for _, user := range users {
		if roleByID[user.OrgRoleId].Scope == orgmodel.ScopeDept {
			scoped = append(scoped, user.Id)
		}
	}
	added, err := extraDepartments(db, scoped)
	if err != nil {
		return nil, err
	}
	held, err := heldKeyCounts(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	views := make([]MemberView, 0, len(users))
	for _, user := range users {
		role := roleByID[user.OrgRoleId]
		managed := []int{}
		if role.Scope == orgmodel.ScopeDept {
			managed = withOwnDepartment(user.DepartmentId, added[user.Id])
		}
		views = append(views, MemberView{
			Id:                   user.Id,
			Username:             user.Username,
			DisplayName:          user.DisplayName,
			Email:                user.Email,
			RoleId:               user.OrgRoleId,
			Role:                 role.Name,
			DepartmentId:         user.DepartmentId,
			IsOwner:              user.Id == ownerID,
			IsService:            user.IsService,
			ManagedDepartmentIds: managed,
			KeyCount:             held[user.Id],
		})
	}
	return views, nil
}

// heldKeyCounts returns how many of an organization's keys each member holds.
func heldKeyCounts(db *gorm.DB, orgID int) (map[int]int, error) {
	var rows []struct {
		UserId int
		Held   int
	}
	if err := db.Model(&platformmodel.Token{}).Select("user_id, COUNT(*) AS held").
		Where("org_id = ?", orgID).Group("user_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[int]int, len(rows))
	for _, row := range rows {
		counts[row.UserId] = row.Held
	}
	return counts, nil
}

// UpdateMember gives a member another role, moves them to another department,
// changes which departments they manage, or any of the three at once — all of
// it or none of it. Appointing and dismissing admins goes through here too.
func UpdateMember(db *gorm.DB, actor *Actor, memberID int, patch MemberPatch) error {
	// Every change below takes an inherent power, and whoever holds neither
	// learns nothing here about who is in the organization.
	if !actor.can(orgmodel.PowerRoles, wholeOrganization) && !actor.can(orgmodel.PowerDepartments, wholeOrganization) {
		return ErrForbidden
	}
	return db.Transaction(func(tx *gorm.DB) error {
		member, err := findMember(tx, actor.OrgId, memberID)
		if err != nil {
			return err
		}
		oldRole, err := findRole(tx, actor.OrgId, member.OrgRoleId)
		if err != nil {
			return err
		}
		newRole := oldRole
		updates := map[string]any{}

		if patch.RoleId != nil && *patch.RoleId != member.OrgRoleId {
			var permit *writePermit
			newRole, permit, err = permitRoleChange(tx, actor, member, oldRole, *patch.RoleId)
			if err != nil {
				return err
			}
			updates["role_id"] = newRole.Id
			if err := permit.record(tx, orgmodel.AuditRoleAssign, orgmodel.AuditTargetMember, member.Id,
				memberRole{RoleId: oldRole.Id, Role: oldRole.Name},
				memberRole{RoleId: newRole.Id, Role: newRole.Name}); err != nil {
				return err
			}
		}

		// The department follows from the role the member ends up with, so it
		// is decided after the role.
		departmentID, err := placeMember(tx, actor.OrgId, newRole, patch.DepartmentId)
		if err != nil {
			return err
		}
		if departmentID == 0 {
			departmentID = member.DepartmentId
		}
		if departmentID != member.DepartmentId {
			permit, err := actor.permit(orgmodel.PowerDepartments, wholeOrganization)
			if err != nil {
				return err
			}
			updates["department_id"] = departmentID
			if err := permit.record(tx, orgmodel.AuditMemberMove, orgmodel.AuditTargetMember, member.Id,
				describeDepartment(tx, actor.OrgId, member.DepartmentId),
				describeDepartment(tx, actor.OrgId, departmentID)); err != nil {
				return err
			}
		}

		if err := manageDepartments(tx, actor, member, oldRole, newRole, departmentID, patch.ManagedDepartmentIds); err != nil {
			return err
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

// runsOrganization reports whether a role is the preset owner or admin role.
// Their holders run the whole organization, so they belong to its default
// department and to no business unit (PRD D26).
func runsOrganization(role *orgmodel.OrgRole) bool {
	return isPreset(role, orgmodel.RoleOwner) || isPreset(role, orgmodel.RoleAdmin)
}

// placeMember decides the department of a member who will hold role. The owner
// and admins go to the default department, and asking for any other is
// refused; everyone else goes where the request says. Zero means the member
// stays where they are.
func placeMember(tx *gorm.DB, orgID int, role *orgmodel.OrgRole, requested *int) (int, error) {
	if runsOrganization(role) {
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

// permitRoleChange decides whether actor may give member the role newRoleID,
// and returns that role with the leave to assign it.
func permitRoleChange(tx *gorm.DB, actor *Actor, member *platformmodel.User, oldRole *orgmodel.OrgRole, newRoleID int) (*orgmodel.OrgRole, *writePermit, error) {
	ownerID, err := ownerUserID(tx, actor.OrgId)
	if err != nil {
		return nil, nil, err
	}
	if member.Id == ownerID {
		return nil, nil, ErrOwnerImmutable
	}
	if member.IsService {
		return nil, nil, ErrServiceAccountRole
	}
	newRole, err := findRole(tx, actor.OrgId, newRoleID)
	if err != nil {
		return nil, nil, err
	}
	if isPreset(newRole, orgmodel.RoleOwner) {
		return nil, nil, ErrOwnerImmutable
	}
	appointsOrDismissesAdmin := isPreset(newRole, orgmodel.RoleAdmin) || isPreset(oldRole, orgmodel.RoleAdmin)
	if appointsOrDismissesAdmin && !actor.can(orgmodel.PowerAdmins, wholeOrganization) {
		return nil, nil, ErrOwnerOnly
	}
	permit, err := actor.permit(orgmodel.PowerRoles, wholeOrganization)
	if err != nil {
		return nil, nil, err
	}
	return newRole, permit, nil
}

// describeDepartment names a department for the audit log. A department that
// no longer exists keeps its id and an empty name.
func describeDepartment(db *gorm.DB, orgID int, departmentID int) memberDepartment {
	described := memberDepartment{DepartmentId: departmentID}
	if department, err := findDepartment(db, orgID, departmentID); err == nil {
		described.Department = department.Name
	}
	return described
}

// manageDepartments settles which departments a member manages besides their
// own once their role and department are decided (PRD D28). Only a role with
// department scope manages any, so any other role clears them; requested, when
// given, replaces them.
func manageDepartments(tx *gorm.DB, actor *Actor, member *platformmodel.User, oldRole *orgmodel.OrgRole, newRole *orgmodel.OrgRole, departmentID int, requested *[]int) error {
	added, err := extraDepartments(tx, []int{member.Id})
	if err != nil {
		return err
	}
	current := added[member.Id]

	if newRole.Scope != orgmodel.ScopeDept {
		if requested != nil && len(*requested) > 0 {
			return ErrNotDepartmentScoped
		}
		if len(current) == 0 {
			return nil
		}
		// Losing the role is what took them away, and the role.assign record
		// says so; they get no record of their own.
		return clearExtraDepartments(tx, []int{member.Id})
	}
	if requested == nil {
		return nil
	}
	wanted := []int{}
	for _, id := range *requested {
		department, err := findDepartment(tx, actor.OrgId, id)
		if err != nil {
			return err
		}
		if department.Id != departmentID && !slices.Contains(wanted, department.Id) {
			wanted = append(wanted, department.Id)
		}
	}
	slices.Sort(wanted)
	if slices.Equal(current, wanted) {
		return nil
	}
	permit, err := actor.permit(orgmodel.PowerRoles, wholeOrganization)
	if err != nil {
		return err
	}
	if err := replaceExtraDepartments(tx, member.Id, wanted); err != nil {
		return err
	}
	before := memberManages{DepartmentIds: []int{}}
	if oldRole.Scope == orgmodel.ScopeDept {
		before.DepartmentIds = withOwnDepartment(member.DepartmentId, current)
	}
	return permit.record(tx, orgmodel.AuditMemberManages, orgmodel.AuditTargetMember, member.Id,
		before, memberManages{DepartmentIds: withOwnDepartment(departmentID, wanted)})
}

// CreateServiceAccount adds a member that is not a person: a CI pipeline or a
// bot that holds keys and shows up in usage, and can never sign in. Only the
// owner and admins create them (PRD D27). A zero departmentID puts it in the
// default department.
func CreateServiceAccount(db *gorm.DB, actor *Actor, rawName string, departmentID int) (*MemberView, error) {
	permit, err := actor.permit(orgmodel.PowerServiceAccounts, wholeOrganization)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(rawName)
	if name == "" || utf8.RuneCountInString(name) > ServiceAccountNameMaxLength {
		return nil, ErrInvalidServiceAccountName
	}
	var view *MemberView
	err = db.Transaction(func(tx *gorm.DB) error {
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
			Id:                   account.Id,
			Username:             account.Username,
			DisplayName:          account.DisplayName,
			RoleId:               staffRoleID,
			Role:                 orgmodel.RoleStaff,
			DepartmentId:         resolvedDepartmentID,
			IsService:            true,
			ManagedDepartmentIds: []int{},
		}
		return permit.record(tx, orgmodel.AuditServiceAccountCreate, orgmodel.AuditTargetMember, account.Id, nil, view)
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
