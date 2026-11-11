package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/internal/mcp"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestSetMCPRouter_ServesMCPBehindTheRelayMiddleware registers /mcp the way
// SetRouter does - after the relay router, whose CORS it relies on - and
// checks the three things that registration decides: it does not panic next
// to the other root routes, an MCP client's first request is answered, and a
// browser's preflight gets the CORS headers. The tools themselves are covered
// in internal/mcp.
func TestSetMCPRouter_ServesMCPBehindTheRelayMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.NotPanics(t, func() {
		SetApiRouter(engine)
		SetRelayRouter(engine)
		SetConnectRouter(engine)
		SetMCPRouter(engine, ThemeAssets{})
	})

	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}}`
	req := httptest.NewRequest(http.MethodPost, mcp.Path, strings.NewReader(initialize))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"name":"deeprouter"`)

	// A browser opening /mcp is told the method is wrong, not handed a page.
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, mcp.Path, nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	preflight := httptest.NewRequest(http.MethodOptions, mcp.Path, nil)
	// Not example.com: that is httptest's own host, and CORS lets a same-origin
	// request through as not cross-origin at all.
	preflight.Header.Set("Origin", "https://chat.example.org")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "authorization, content-type, mcp-protocol-version")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, preflight)
	require.Less(t, rec.Code, 300, "preflight answered with %d", rec.Code)
	require.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}
