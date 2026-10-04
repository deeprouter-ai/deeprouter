// Package discovery helps AI tools that hold a DeepRouter key but have never
// heard of DeepRouter find its documentation. Agents probe well-known paths
// (/openapi.json, /mcp, /.well-known/...) and read error messages; both now
// point at the machine-readable guide instead of answering with the web app.
package discovery

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GuideURL is the AI-readable index of every guide and API (llms.txt).
const GuideURL = "https://deeprouter.co/llms.txt"

// probePaths are exact paths agents request when looking for an API
// description. None of them is a page of the web app.
var probePaths = map[string]bool{
	"/openapi.json":  true,
	"/openapi.yaml":  true,
	"/openapi.yml":   true,
	"/swagger.json":  true,
	"/mcp":           true,
	"/sse":           true,
	"/llms-full.txt": true,
}

// IsAgentProbe reports whether an unmatched path is a machine probe that
// should get a JSON 404 rather than the single-page app's index.html.
func IsAgentProbe(path string) bool {
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimSuffix(path, "/")
	return probePaths[path] || strings.HasPrefix(path, "/.well-known/")
}

// WithGuide appends the guide pointer to an error message shown to API
// callers, so an agent that hits a wrong path or a bad key learns where the
// real reference lives.
func WithGuide(message string) string {
	return fmt.Sprintf("%s. API reference for AI tools: %s", strings.TrimRight(message, ". "), GuideURL)
}

// NotFound answers an agent probe: an OpenAI-shaped error plus a `docs`
// field, with a real 404 status so tools do not mistake the HTML app for a
// spec document.
func NotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{
		"error": gin.H{
			"message": WithGuide(fmt.Sprintf("No API description at %s %s", c.Request.Method, c.Request.URL.Path)),
			"type":    "invalid_request_error",
			"param":   "",
			"code":    "",
		},
		"docs": GuideURL,
	})
}
