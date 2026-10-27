package router

import (
	"io/fs"

	"github.com/QuantumNous/new-api/internal/mcp"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetMCPRouter serves the MCP endpoint (internal/mcp): a read-only desk where
// an AI tool asks which models a key can use and how to connect a client.
//
// It must be registered after SetRelayRouter, whose CORS it inherits, and
// before SetWebRouter, so the web app's static files and fallback never answer
// /mcp. Every method reaches the handler: POST carries MCP, OPTIONS is a CORS
// preflight, and the SDK answers anything else with 405.
func SetMCPRouter(router *gin.Engine, assets ThemeAssets) {
	// The integration guides the web app publishes at /docs/integrations/,
	// read from the same embedded build so get_connection_config and the
	// guide a person reads are one text.
	guides, err := fs.Sub(assets.DefaultBuildFS, "web/default/dist/docs/integrations")
	if err != nil {
		guides = nil
	}
	router.Any(mcp.Path, middleware.RouteTag("mcp"), middleware.GlobalAPIRateLimit(), mcp.NewHandler(router, guides))
}
