package constant

// Enterprise Org (meta-repo docs/enterprise-org-prd.md §7.4): what TokenAuth
// resolves once for a request made with an organization key, for the billing
// and the usage log further down the request to read. All three are absent
// from every other request, which is what keeps a personal account's path
// exactly as it was.
const (
	// ContextKeyOrgId is the organization the key belongs to.
	ContextKeyOrgId ContextKey = "org_id"
	// ContextKeyOrgDepartmentId is the department the key's holder is in at the
	// moment of the request; the usage log is stamped with it.
	ContextKeyOrgDepartmentId ContextKey = "org_department_id"
	// ContextKeyOrgWalletUserId is the organization's owner, whose balance is
	// the company wallet.
	ContextKeyOrgWalletUserId ContextKey = "org_wallet_user_id"
)
