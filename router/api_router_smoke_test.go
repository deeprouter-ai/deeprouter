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
