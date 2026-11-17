package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/smart_router_client"

	"github.com/gin-gonic/gin"
)

// GetSmartRouterStatus tells the Smart Routing settings page whether a
// TypeSafe key is saved. The options listing withholds every *api_key value,
// so without this the page could not tell "no key yet" from "key saved".
// The key itself is never returned.
func GetSmartRouterStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"jev_api_key_configured": smart_router_client.JevAPIKeyConfigured(),
		},
	})
}

// maxTestPromptRunes caps how much text one admin test may send.
const maxTestPromptRunes = 20000

// smartRouterForTest is the client the test panel uses; tests swap it for one pointed at a fake smart-router.
var smartRouterForTest = smart_router_client.Default

// smartRouterTestResult is one admin test: what smart-router said, how long the whole test took,
// and how long live traffic waits for smart-router before using the default model.
type smartRouterTestResult struct {
	// Outcome is ok (smart-router answered), not_configured or unreachable.
	Outcome string `json:"outcome"`
	*smart_router_client.ExplainResult
	ElapsedMs     int64 `json:"elapsed_ms"`
	GatewayWaitMs int64 `json:"gateway_wait_ms"`
}

// RunSmartRouterTest lets an admin type a prompt and see how deeprouter-auto would route it for
// their own account: Jev's verdict (or why Jev was not used), the matched rule and the model.
// No model is called, and the prompt is neither stored nor logged.
func RunSmartRouterTest(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		common.ApiErrorMsg(c, "prompt is empty")
		return
	}
	if utf8.RuneCountInString(prompt) > maxTestPromptRunes {
		common.ApiErrorMsg(c, "prompt is longer than "+strconv.Itoa(maxTestPromptRunes)+" characters")
		return
	}

	client := smartRouterForTest()
	start := time.Now()
	res, err := client.Explain(c.Request.Context(), strconv.Itoa(c.GetInt("id")), prompt)
	out := smartRouterTestResult{
		Outcome:       "ok",
		ExplainResult: res,
		ElapsedMs:     time.Since(start).Milliseconds(),
		GatewayWaitMs: client.Timeout().Milliseconds(),
	}
	switch {
	case errors.Is(err, smart_router_client.ErrNotConfigured):
		out.Outcome = "not_configured"
	case err != nil:
		out.Outcome = "unreachable"
	}
	common.ApiSuccess(c, out)
}
