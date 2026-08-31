package connect

// The terminal AI tools one-click setup covers (PRD §3 D1). Kept server-side
// so a tool list can never be smuggled into a grant: the page proposes, this
// list decides.
const (
	ToolClaudeCode = "claude-code"
	ToolOpenCode   = "opencode"
	ToolCodex      = "codex"
	ToolGeminiCLI  = "gemini-cli"
)

// Tool is one entry of the supported list, in the order the key page shows them.
type Tool struct {
	ID string `json:"id"`
	// Name is the tool's own product name, deliberately not translated.
	Name string `json:"name"`
}

// SupportedTools is the whole catalogue. Adding one here is not enough to make
// it work — P2's script has to learn to configure it too.
var SupportedTools = []Tool{
	{ID: ToolClaudeCode, Name: "Claude Code"},
	{ID: ToolOpenCode, Name: "OpenCode"},
	{ID: ToolCodex, Name: "Codex CLI"},
	{ID: ToolGeminiCLI, Name: "Gemini CLI"},
}

// NormalizeTools keeps the requested tools that we actually support, in the
// catalogue's order and without duplicates. Unknown entries are dropped rather
// than rejected: the page and the server can disagree across a deploy, and the
// useful outcome is configuring what both understand.
func NormalizeTools(requested []string) []string {
	wanted := make(map[string]bool, len(requested))
	for _, id := range requested {
		wanted[id] = true
	}
	out := make([]string, 0, len(SupportedTools))
	for _, tool := range SupportedTools {
		if wanted[tool.ID] {
			out = append(out, tool.ID)
		}
	}
	return out
}

// ToolName maps an ID back to its display name, falling back to the ID so an
// unknown value shows up as itself instead of vanishing.
func ToolName(id string) string {
	for _, tool := range SupportedTools {
		if tool.ID == id {
			return tool.Name
		}
	}
	return id
}
