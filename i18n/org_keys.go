package i18n

// Enterprise Org messages a request can be answered with outside the
// /api/org endpoints: on the relay path, where an organization key spends the
// company wallet (meta-repo docs/enterprise-org-prd.md §7.4). The management
// endpoints keep their own keys in controller/org.go; these live here because
// middleware, service and controller all answer with them.
const (
	// MsgOrgWalletUnavailable: the key's organization has no account that can
	// be charged — its owner's account is disabled or gone.
	MsgOrgWalletUnavailable = "org.wallet_unavailable"
	// MsgOrgWalletInsufficient: the company wallet is empty. The message never
	// says how much is left; that is the owner's to know.
	MsgOrgWalletInsufficient = "org.wallet_insufficient"
	// MsgOrgPlaygroundClosed: an organization account spends through its keys
	// only, so the keyless playground endpoint is closed to it.
	MsgOrgPlaygroundClosed = "org.playground_closed"
	// MsgOrgWalletLowTitle and the two bodies are the reminder the owner and
	// the admins get when the company wallet runs low. Both bodies take
	// {{.Balance}} and {{.Link}}; the HTML one is for mail and webhooks.
	MsgOrgWalletLowTitle    = "org.wallet_low_title"
	MsgOrgWalletLowBody     = "org.wallet_low_body"
	MsgOrgWalletLowBodyHTML = "org.wallet_low_body_html"
)

// What a notification about an organization's alerts is put together from
// (service/org_alerts.go): a title that takes {{.Count}}, one line per alert,
// and two closing lines. Every line about a key takes {{.Key}} and
// {{.Holder}}; the rest is named beside it.
const (
	MsgOrgAlertTitle = "org.alert_title"
	// A warning short of the limit, and one at it: {{.Used}}, {{.Limit}} and,
	// short of the limit, {{.Percent}}.
	MsgOrgAlertQuota        = "org.alert_quota"
	MsgOrgAlertQuotaSpent   = "org.alert_quota_spent"
	MsgOrgAlertMonthly      = "org.alert_monthly"
	MsgOrgAlertMonthlySpent = "org.alert_monthly_spent"
	// The anomaly rules: {{.Spent}} and {{.Average}}, or {{.Ips}}.
	MsgOrgAlertSpike    = "org.alert_spike"
	MsgOrgAlertOffHours = "org.alert_offhours"
	MsgOrgAlertNewIP    = "org.alert_new_ip"
	// MsgOrgAlertMore stands for the alerts a long notification leaves out;
	// it takes {{.Count}}.
	MsgOrgAlertMore = "org.alert_more"
	// MsgOrgAlertFooter closes a notification that reports an anomaly: nothing
	// was blocked, and where to stop a key.
	MsgOrgAlertFooter = "org.alert_footer"
)
