package connect

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// issueRequest is what the key page posts: which of the user's keys to set up,
// and which tools they ticked.
type issueRequest struct {
	KeyID int      `json:"key_id"`
	Tools []string `json:"tools"`
}

// IssueToken hands the logged-in user a one-time token for one of their own keys.
//
// 🔴 The authorization boundary is one line — GetTokenByIds scopes the lookup by
// (key id, user id) — and it is the only thing standing between a session and
// somebody else's key. Everything else here is validation.
func IssueToken(c *gin.Context) {
	userID := c.GetInt("id")
	if userID == 0 {
		// UserAuth should have caught this; refuse rather than issue a grant
		// bound to user 0.
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
		return
	}

	var req issueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式错误"})
		return
	}

	tools := NormalizeTools(req.Tools)
	if len(tools) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请先选择要配置的工具"})
		return
	}

	// Scoped by user: a key id belonging to somebody else is simply not found.
	key, err := model.GetTokenByIds(req.KeyID, userID)
	if err != nil || key == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "密钥不存在"})
		return
	}

	token, err := Issue(Grant{UserID: userID, TokenID: key.Id, Tools: tools})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "签发失败，请重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"token":       token,
			"expires_in":  int(GrantTTL.Seconds()),
			"base_url":    system_setting.ServerAddress,
			"tools":       tools,
			"script_path": "/i/" + token,
		},
	})
}

// RedeemScript exchanges a token for the install script.
//
// Unauthenticated by design: the request comes from the user's terminal, which
// has no session. The token is the credential, it dies on this call, and it
// only ever names one user's own key.
func RedeemScript(c *gin.Context) {
	grant, err := Redeem(c.Param("token"))
	if err != nil {
		// A dead token must still SPEAK. The audience is a human terminal, but
		// the transport is `curl -fsSL | sh` / `irm | iex`: comment lines print
		// nothing, and any non-2xx status makes curl's -f discard the body
		// entirely. Both were measured live - the user saw nothing at all.
		// So: HTTP 200, per-platform echo lines, exit 1 (PRD 6).
		c.String(http.StatusOK, RenderDeadTokenScript(
			PlatformFromUserAgent(c.GetHeader("User-Agent")),
			"This setup command is no longer valid.",
			"It works once and expires after 15 minutes.",
			"Open your API keys page and copy a fresh command."))
		return
	}

	key, err := model.GetTokenByIds(grant.TokenID, grant.UserID)
	if err != nil || key == nil {
		// Same transport constraints as the dead-token stub above.
		c.String(http.StatusOK, RenderDeadTokenScript(
			PlatformFromUserAgent(c.GetHeader("User-Agent")),
			"The key this command was made for no longer exists.",
			"Open your API keys page and copy a fresh command."))
		return
	}

	// Which shell is about to run this decides the whole syntax. Getting it
	// wrong is not a no-op: PowerShell echoes every line it cannot parse, and
	// the line holding the key is one of them.
	platform := PlatformFromUserAgent(c.Request.UserAgent())

	c.Header("Cache-Control", "no-store")
	c.String(http.StatusOK, RenderScript(platform, system_setting.ServerAddress, key.GetFullKey(), grant.Tools))
}

// UninstallScript serves the undo script.
//
// No token, no session, no rate limit. Undoing carries no secret — every change
// it reverses was recorded on the user's own machine — and a fixed address is
// the point: somebody trying to remove our configuration must never be told
// their command expired, or the honest answer to "can I take this back" stops
// being yes (PRD §4.6).
func UninstallScript(c *gin.Context) {
	platform := PlatformFromUserAgent(c.Request.UserAgent())
	c.Header("Cache-Control", "no-store")
	c.String(http.StatusOK, RenderUninstall(platform))
}

// ListTools tells the key page which tools this build can configure, so the
// page never offers one the server would drop.
func ListTools(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": SupportedTools})
}
