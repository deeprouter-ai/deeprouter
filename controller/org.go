package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// i18n keys of the organization messages (i18n/locales/*.yaml).
const (
	msgOrgNameInvalid               = "org.name_invalid"
	msgOrgNotMember                 = "org.not_member"
	msgOrgForbidden                 = "org.forbidden"
	msgOrgOwnerOnly                 = "org.owner_only"
	msgOrgOwnerImmutable            = "org.owner_immutable"
	msgOrgOwnerCannotDeleteAccount  = "org.owner_cannot_delete_account"
	msgOrgMemberNotFound            = "org.member_not_found"
	msgOrgRoleNotFound              = "org.role_not_found"
	msgOrgDepartmentNotFound        = "org.department_not_found"
	msgOrgDepartmentNameInvalid     = "org.department_name_invalid"
	msgOrgDepartmentExists          = "org.department_exists"
	msgOrgDepartmentDefault         = "org.department_default"
	msgOrgAdminDepartment           = "org.admin_department"
	msgOrgServiceAccountNameInvalid = "org.service_account_name_invalid"
	msgOrgServiceAccountRole        = "org.service_account_role"
	msgOrgServiceAccountLogin       = "org.service_account_login"
	msgOrgInviteInvalid             = "org.invite_invalid"
)

// errOrgSignUpAmbiguous means a sign-up asked both to found an organization
// and to join one by invite. One person belongs to one organization (PRD D20).
var errOrgSignUpAmbiguous = errors.New("a sign-up cannot both create an organization and join one")

// orgRefusals maps what an organization action can refuse with to the message
// the caller sees and the status it travels with. Being refused for lack of
// permission is a real 403 (PRD §2); everything else is the 200 +
// success=false every other /api error uses.
var orgRefusals = []struct {
	err    error
	key    string
	status int
}{
	{orgservice.ErrNotMember, msgOrgNotMember, http.StatusForbidden},
	{orgservice.ErrForbidden, msgOrgForbidden, http.StatusForbidden},
	{orgservice.ErrOwnerOnly, msgOrgOwnerOnly, http.StatusForbidden},
	{orgservice.ErrOwnerImmutable, msgOrgOwnerImmutable, http.StatusOK},
	{orgservice.ErrInvalidName, msgOrgNameInvalid, http.StatusOK},
	{orgservice.ErrMemberNotFound, msgOrgMemberNotFound, http.StatusOK},
	{orgservice.ErrRoleNotFound, msgOrgRoleNotFound, http.StatusOK},
	{orgservice.ErrDepartmentNotFound, msgOrgDepartmentNotFound, http.StatusOK},
	{orgservice.ErrInvalidDepartmentName, msgOrgDepartmentNameInvalid, http.StatusOK},
	{orgservice.ErrDepartmentExists, msgOrgDepartmentExists, http.StatusOK},
	{orgservice.ErrDefaultDepartment, msgOrgDepartmentDefault, http.StatusOK},
	{orgservice.ErrAdminDepartment, msgOrgAdminDepartment, http.StatusOK},
	{orgservice.ErrInvalidServiceAccountName, msgOrgServiceAccountNameInvalid, http.StatusOK},
	{orgservice.ErrServiceAccountRole, msgOrgServiceAccountRole, http.StatusOK},
	{orgservice.ErrInviteNotFound, msgOrgInviteInvalid, http.StatusOK},
	{errOrgSignUpAmbiguous, i18n.MsgInvalidParams, http.StatusOK},
}

// orgError answers a failed organization action. An error that is not an
// organization refusal goes out exactly as common.ApiError would send it.
func orgError(c *gin.Context, err error) {
	for _, refusal := range orgRefusals {
		if errors.Is(err, refusal.err) {
			c.JSON(refusal.status, gin.H{
				"success": false,
				"message": common.TranslateMessage(c, refusal.key),
			})
			return
		}
	}
	common.ApiError(c, err)
}

// orgActor loads the caller as a member of their organization. When the
// caller is not in one it answers the request and reports false.
func orgActor(c *gin.Context) (*orgservice.Actor, bool) {
	actor, err := orgservice.LoadActor(model.DB, c.GetInt("id"))
	if err != nil {
		orgError(c, err)
		return nil, false
	}
	return actor, true
}

// orgPathID reads the numeric :id of an organization route.
func orgPathID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return 0, false
	}
	return id, true
}

// orgBody decodes the JSON body of an organization request.
func orgBody(c *gin.Context, into any) bool {
	if err := common.DecodeJson(c.Request.Body, into); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return false
	}
	return true
}

// GetOrgSelf returns the caller's organization, role and department, or null
// data for a personal account (Enterprise Org P2).
func GetOrgSelf(c *gin.Context) {
	membership, err := orgservice.GetMembership(model.DB, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, membership)
}

// ListOrgDepartments returns the departments of the caller's organization.
func ListOrgDepartments(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	departments, err := orgservice.ListDepartments(model.DB, actor)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, departments)
}

// orgDepartmentRequest is the body of a department create or rename.
type orgDepartmentRequest struct {
	Name string `json:"name"`
}

// CreateOrgDepartment adds a department to the caller's organization.
func CreateOrgDepartment(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	var req orgDepartmentRequest
	if !orgBody(c, &req) {
		return
	}
	department, err := orgservice.CreateDepartment(model.DB, actor, req.Name)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, department)
}

// RenameOrgDepartment renames a department of the caller's organization.
func RenameOrgDepartment(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	var req orgDepartmentRequest
	if !orgBody(c, &req) {
		return
	}
	if err := orgservice.RenameDepartment(model.DB, actor, id, req.Name); err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// DeleteOrgDepartment deletes a department; its members move to the default one.
func DeleteOrgDepartment(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	if err := orgservice.DeleteDepartment(model.DB, actor, id); err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ListOrgRoles returns the roles members of the caller's organization can hold.
func ListOrgRoles(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	roles, err := orgservice.ListRoles(model.DB, actor)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, roles)
}

// ListOrgMembers returns the members of the caller's organization.
func ListOrgMembers(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	members, err := orgservice.ListMembers(model.DB, actor)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, members)
}

// UpdateOrgMember changes a member's role, department, or both.
func UpdateOrgMember(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	var patch orgservice.MemberPatch
	if !orgBody(c, &patch) {
		return
	}
	if err := orgservice.UpdateMember(model.DB, actor, id, patch); err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// orgServiceAccountRequest is the body of a service account creation.
type orgServiceAccountRequest struct {
	Name         string `json:"name"`
	DepartmentId int    `json:"department_id"`
}

// CreateOrgServiceAccount adds a service account to the caller's organization.
func CreateOrgServiceAccount(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	var req orgServiceAccountRequest
	if !orgBody(c, &req) {
		return
	}
	account, err := orgservice.CreateServiceAccount(model.DB, actor, req.Name, req.DepartmentId)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, account)
}

// ListOrgInvites returns the usable invite links of the caller's organization.
func ListOrgInvites(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	invites, err := orgservice.ListInvites(model.DB, actor)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, invites)
}

// orgInviteRequest is the body of an invite link creation.
type orgInviteRequest struct {
	RoleId       int `json:"role_id"`
	DepartmentId int `json:"department_id"`
}

// CreateOrgInvite issues an invite link for the caller's organization.
func CreateOrgInvite(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	var req orgInviteRequest
	if !orgBody(c, &req) {
		return
	}
	invite, err := orgservice.CreateInvite(model.DB, actor, req.RoleId, req.DepartmentId)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, invite)
}

// RevokeOrgInvite makes an invite link of the caller's organization stop working.
func RevokeOrgInvite(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	if err := orgservice.RevokeInvite(model.DB, actor, id); err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// GetOrgInvitePreview tells whoever holds an invite code which organization,
// role and department it leads to. It is public: the sign-up page calls it
// for a visitor who has no account yet.
func GetOrgInvitePreview(c *gin.Context) {
	preview, err := orgservice.LookupInvite(model.DB, c.Param("code"))
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}

// insertRegisteredUser inserts a newly registered account and returns the id
// of the organization created with it (0 unless the sign-up founded one).
//
// A sign-up that founds an organization (org_name) or joins one (org_invite)
// writes the account and its membership in one transaction — an account that
// asked for a company must never be left behind without one. A sign-up that
// asks for neither is exactly the upstream insert, so personal sign-ups do
// not change at all. lang names the preset departments of a new organization.
func insertRegisteredUser(user *model.User, inviterId int, req *RegisterRequest, lang string) (int, error) {
	if req.OrgName == "" && req.OrgInvite == "" {
		return 0, user.Insert(inviterId)
	}
	if req.OrgName != "" && req.OrgInvite != "" {
		return 0, errOrgSignUpAmbiguous
	}
	// Checked before anything is written, so a bad name or a dead invite
	// costs nothing.
	if req.OrgInvite != "" {
		if _, err := orgservice.LookupInvite(model.DB, req.OrgInvite); err != nil {
			return 0, err
		}
	} else if _, err := orgservice.NormalizeName(req.OrgName); err != nil {
		return 0, err
	}
	orgId := 0
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := user.InsertWithTx(tx, inviterId); err != nil {
			return err
		}
		if req.OrgInvite != "" {
			return orgservice.JoinByInviteTx(tx, user.Id, req.OrgInvite)
		}
		org, err := orgservice.CreateForOwnerTx(tx, user.Id, req.OrgName, lang)
		if err != nil {
			return err
		}
		orgId = org.Id
		return nil
	})
	if err != nil {
		return 0, err
	}
	// InsertWithTx plus this call is what Insert does in one go: the sidebar
	// config, the sign-up log line and the inviter rewards run after the commit.
	user.FinalizeOAuthUserCreation(inviterId)
	return orgId, nil
}

// isOrgOwner reports whether a user owns an organization. The owner's account
// is the company wallet, so it is not theirs to delete (PRD D10, D19).
func isOrgOwner(user *model.User) bool {
	// A personal account answers without touching the org tables.
	if user == nil || user.OrgId == 0 {
		return false
	}
	owns, _ := orgservice.OwnsOrganization(model.DB, user.Id)
	return owns
}
