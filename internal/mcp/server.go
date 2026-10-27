// Package mcp serves DeepRouter's MCP endpoint: a read-only information desk
// that an AI tool (Claude Code, Codex, Cursor...) adds once and then asks which
// models its key can use, how to call each one and how to set up a client.
//
// Nothing here generates or spends: every tool only reads. The answers come
// from the code the HTTP API itself runs - /v1/models, the public pricing
// catalog, the integration guides this gateway publishes - so what an AI is
// told is what the API will do. The AI still calls the API to get work done;
// MCP only tells it how (meta-repo docs/deeprouter-mcp-prd.md, D4).
package mcp

import (
	"io/fs"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/discovery"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Path is where the endpoint is served.
const Path = "/mcp"

// Public addresses the answers point people to.
const (
	siteURL    = "https://deeprouter.co"
	keysURL    = siteURL + "/keys"
	signUpURL  = siteURL + "/sign-up"
	walletURL  = siteURL + "/wallet"
	pricingURL = siteURL + "/pricing"
	guidesURL  = siteURL + "/docs/integrations/"
)

// instructions is what an MCP client is told about this server when it
// connects; most clients put it in front of the model.
const instructions = `DeepRouter is one API key for many AI models (chat, image, video, speech and more) behind OpenAI-, Anthropic- and Gemini-compatible endpoints.

This server only answers questions; it never generates anything and never spends money. To do the work, call the DeepRouter API with the user's key as these tools describe.

- list_models: which models this key can use, what each is for, its endpoint and price.
- get_model: how to call one model - path, required parameters, async steps for video, billing.
- get_connection_config: the exact base URL, auth header and model to enter in a client such as Cursor, Codex or Cherry Studio.
- check_key: whether the key works, its limit, expiry and balance.
- get_started: what DeepRouter is and how to get a key.

Never guess a model id or an endpoint: take them from list_models and get_model. Never show the user's key in full. Full API reference: ` + discovery.GuideURL

// NewHandler returns the gin handler for Path. engine is the gateway's own
// engine: a key is checked through it exactly as /v1/models checks one (client
// IP, groups, an organization's wallet). guides holds the published
// integration guides (docs/integrations/*.md); nil leaves get_connection_config
// saying it has none rather than failing to start.
func NewHandler(engine *gin.Engine, guides fs.FS) gin.HandlerFunc {
	shelf, err := loadGuides(guides)
	if err != nil {
		common.SysLog("mcp: integration guides unavailable: " + err.Error())
	}
	d := &desk{engine: engine, guides: shelf}

	server := mcp.NewServer(&mcp.Implementation{
		Name:       "deeprouter",
		Title:      "DeepRouter",
		Version:    "1.0.0",
		WebsiteURL: siteURL,
	}, &mcp.ServerOptions{Instructions: instructions})
	d.register(server)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		// Every request stands alone: no session to lose behind a proxy, on a
		// restart or across instances. The key travels on every request anyway.
		Stateless: true,
		// Plain JSON answers rather than an event stream, so no proxy buffering
		// or compression in front of the gateway can hold a reply back.
		JSONResponse: true,
		// The guard is for MCP servers running on a user's own machine. This
		// one is public, behind Caddy, and authenticates by a bearer key a
		// browser never attaches on its own; the guard would only turn away a
		// self-hosted gateway that sits behind a local reverse proxy.
		DisableLocalhostProtection: true,
		// Requests are a handful of short JSON-RPC messages.
		MaxRequestBodyBytes: 1 << 20,
	})

	return func(c *gin.Context) {
		ctx := withCaller(c.Request.Context(), newCaller(c.Request, engine))
		handler.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
	}
}

// desk holds what the tools answer from.
type desk struct {
	engine *gin.Engine
	guides *guideShelf
}
