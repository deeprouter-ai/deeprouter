package model

// What an audit record says was done (org_audit_logs.action). Where the act is
// a primitive the record carries the primitive's name; an act done under an
// inherent power gets a name of its own, because "department.manage" would not
// tell a rename from a deletion.
const (
	AuditDepartmentCreate     = "department.create"
	AuditDepartmentRename     = "department.rename"
	AuditDepartmentDelete     = "department.delete"
	AuditRoleCreate           = "role.create"
	AuditRoleUpdate           = "role.update"
	AuditRoleDelete           = "role.delete"
	AuditRoleAssign           = "role.assign"    // a member was given another role
	AuditMemberMove           = "member.move"    // a member was moved to another department
	AuditMemberManages        = "member.manages" // the departments a member manages besides their own changed
	AuditMemberInvite         = "member.invite"  // an invite link was issued
	AuditInviteRevoke         = "invite.revoke"
	AuditMemberJoin           = "member.join" // someone signed up through an invite link
	AuditServiceAccountCreate = "service_account.create"
)

// What an audit record is about (org_audit_logs.target_type).
const (
	AuditTargetDepartment = "department"
	AuditTargetRole       = "role"
	AuditTargetMember     = "member"
	AuditTargetInvite     = "invite"
)
