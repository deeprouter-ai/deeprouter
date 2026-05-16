package dto

type UserSetting struct {
	NotifyType                       string  `json:"notify_type,omitempty"`                          // QuotaWarningType 额度预警类型
	QuotaWarningThreshold            float64 `json:"quota_warning_threshold,omitempty"`              // QuotaWarningThreshold 额度预警阈值
	WebhookUrl                       string  `json:"webhook_url,omitempty"`                          // WebhookUrl webhook地址
	WebhookSecret                    string  `json:"webhook_secret,omitempty"`                       // WebhookSecret webhook密钥
	NotificationEmail                string  `json:"notification_email,omitempty"`                   // NotificationEmail 通知邮箱地址
	BarkUrl                          string  `json:"bark_url,omitempty"`                             // BarkUrl Bark推送URL
	GotifyUrl                        string  `json:"gotify_url,omitempty"`                           // GotifyUrl Gotify服务器地址
	GotifyToken                      string  `json:"gotify_token,omitempty"`                         // GotifyToken Gotify应用令牌
	GotifyPriority                   int     `json:"gotify_priority"`                                // GotifyPriority Gotify消息优先级
	UpstreamModelUpdateNotifyEnabled bool    `json:"upstream_model_update_notify_enabled,omitempty"` // 是否接收上游模型更新定时检测通知（仅管理员）
	AcceptUnsetRatioModel            bool    `json:"accept_unset_model_ratio_model,omitempty"`       // AcceptUnsetRatioModel 是否接受未设置价格的模型
	RecordIpLog                      bool    `json:"record_ip_log,omitempty"`                        // 是否记录请求和错误日志IP
	SidebarModules                   string  `json:"sidebar_modules,omitempty"`                      // SidebarModules 左侧边栏模块配置
	BillingPreference                string  `json:"billing_preference,omitempty"`                   // BillingPreference 扣费策略（订阅/钱包）
	Language                         string  `json:"language,omitempty"`                             // Language 用户语言偏好 (zh, en)
	// Persona is a UI-tailoring preference (not a permission). Values:
	//   "casual" — non-technical end users; sidebar shrunk to chat/playground/wallet.
	//   "dev"    — developers; full sidebar minus admin.
	//   "team"   — team/enterprise; same as dev today, reserved for future.
	//   "unset"  — sentinel set by Register on new accounts. Frontend reads
	//              this as "prompt picker on first authenticated load".
	//   absent   — legacy user (predates persona). Frontend treats as "dev,
	//              no prompt" so their experience doesn't suddenly shift.
	// We keep omitempty so legacy users' setting JSON stays clean — only
	// explicitly-set "unset"/"casual"/... values round-trip.
	Persona string `json:"persona,omitempty"`

	// === Onboarding signup-captured fields (PR #8) ===
	// Set during the 4-step signup wizard. All optional; defaults are
	// safe (empty strings handled by frontend fallback logic).

	// BrandPreference: 'claude' | 'openai' | 'gemini' | 'deepseek' | ''
	// Drives the auto-created default token's simple_brand binding so
	// users who picked Claude at signup can immediately call model:
	// "deeprouter" and get Claude.
	BrandPreference string `json:"brand_preference,omitempty"`

	// PreferredClient: 'cherry-studio' | 'chatbox' | 'lobechat' | 'cursor'
	// | 'claude-code' | 'code' | 'playground' | 'dashboard' | ''
	// Drives the post-signup redirect target.
	PreferredClient string `json:"preferred_client,omitempty"`

	// AcquisitionChannel: captured silently from document.referrer +
	// utm_* params at signup time. Marketing attribution.
	AcquisitionChannel string `json:"acquisition_channel,omitempty"`

	// Timezone: IANA tz string from browser (e.g. "Asia/Shanghai").
	Timezone string `json:"timezone,omitempty"`

	// OnboardingCompletedAt: ISO timestamp set when wizard finishes
	// (skipping steps still counts as complete). Drives the
	// "Getting Started" checklist on dashboard.
	OnboardingCompletedAt string `json:"onboarding_completed_at,omitempty"`

	// === Optional profile fields (PR #9, edited in /profile not signup) ===

	// Industry: 'education' | 'finance' | 'ecommerce' | 'gaming' |
	// 'individual' | 'saas' | 'other' | ''
	Industry string `json:"industry,omitempty"`

	// ExpectedVolume: 'trying' | 'daily-low' | 'daily-medium' |
	// 'daily-high' | ''
	ExpectedVolume string `json:"expected_volume,omitempty"`

	// MarketingEmails: explicit opt-in. Default false.
	MarketingEmails bool `json:"marketing_emails,omitempty"`

	// OnboardingChecklistDismissedAt: user clicked X on the Getting
	// Started widget. Don't show again.
	OnboardingChecklistDismissedAt string `json:"onboarding_checklist_dismissed_at,omitempty"`
}

var (
	NotifyTypeEmail   = "email"   // Email 邮件
	NotifyTypeWebhook = "webhook" // Webhook
	NotifyTypeBark    = "bark"    // Bark 推送
	NotifyTypeGotify  = "gotify"  // Gotify 推送
)
