package router

import (
	"github.com/QuantumNous/new-api/internal/connect"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetConnectRouter registers one-click setup: issuing a one-time token from the
// key page, and redeeming it for an install script.
//
// The two halves authenticate differently on purpose. Issuing requires a logged-in
// session, because it is the step that decides whose key this is about. Redeeming
// cannot require one — it runs in the user's terminal via `curl | sh`, which has
// no session — so there the token IS the credential: single-use, 30 minutes, and
// bound to one key that the session already proved belongs to the user.
//
// `/i/:token` sits at the root rather than under /api because it has to fit in a
// command a person retypes from a screenshot.
func SetConnectRouter(router *gin.Engine) {
	api := router.Group("/api/connect")
	api.Use(middleware.RouteTag("connect"))
	{
		api.GET("/tools", connect.ListTools)
		api.POST("/token", middleware.UserAuth(), middleware.CriticalRateLimit(), connect.IssueToken)
	}

	router.GET("/i/:token", middleware.RouteTag("connect"), connect.RedeemScript)

	// The downloadable installers (PRD §11): a file the user double-clicks
	// instead of pasting a command. It is only a wrapper around the same
	// /i/:token call, carries a token and never a key, and therefore needs no
	// session either. The filename sits in the path so that a client ignoring
	// Content-Disposition still saves something Windows will run.
	router.GET("/d/:token/:file", middleware.RouteTag("connect"), connect.DownloadInstaller)

	// Undo sits at a fixed address with no token. It reverses only what the
	// setup run recorded on that machine, so there is no secret to protect —
	// and a person removing our configuration must never meet an expiry.
	router.GET("/uninstall", middleware.RouteTag("connect"), connect.UninstallScript)
}
