package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestSetApiRouter_RegistersWithoutPanic exists because gin validates its
// routing tree at registration time: two wildcards with different names at
// the same position (e.g. /api/skills/:slug next to /api/skills/:id) panic
// the process on startup, and nothing short of running the registration
// catches that. Added with the Skill Marketplace V2 P3 split of /api/skills
// (public) vs /api/admin/skills (admin).
func TestSetApiRouter_RegistersWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SetApiRouter panicked: %v", r)
		}
	}()
	SetApiRouter(gin.New())
}

// TestSetConnectRouter_RegistersWithoutPanic covers the same failure mode for
// one-click setup, whose routes sit at the ROOT — /i/:token, /uninstall and
// /d/:token/:file — and are therefore registered alongside each other and the
// api tree rather than inside a group of their own. Registered on the same
// engine as the api router, because that is the arrangement main.go builds and
// a conflict only exists between routes sharing a tree. Added with the
// downloadable installers (one-click PRD §11).
func TestSetConnectRouter_RegistersWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SetConnectRouter panicked: %v", r)
		}
	}()
	engine := gin.New()
	SetApiRouter(engine)
	SetConnectRouter(engine)
}
