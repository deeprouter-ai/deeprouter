package claude

// ModelList is the Anthropic model catalogue exposed by this channel.
//
// Grouped newest-first. Anthropic's current IDs carry no date suffix — use
// "claude-opus-5", never "claude-opus-5-20260xxx". The dated IDs further down
// belong to older generations that still use snapshot naming.
//
// Retired IDs are deliberately kept (see the last block) so channels that were
// configured against them keep resolving instead of disappearing from the admin
// UI; requests to them now 404 upstream.
var ModelList = []string{
	// ── Claude 5 generation (current) ──
	// Opus 5 is the default Opus-tier target. Fable 5 sits above it; Mythos 5 is
	// the same model behind Project Glasswing.
	"claude-opus-5",
	"claude-opus-5-max",
	"claude-opus-5-xhigh",
	"claude-opus-5-high",
	"claude-opus-5-medium",
	"claude-opus-5-low",
	"claude-opus-5-thinking",
	"claude-sonnet-5",
	"claude-sonnet-5-thinking",
	"claude-fable-5",
	"claude-fable-5-thinking",
	"claude-mythos-5",
	"claude-mythos-5-thinking",

	// ── Claude 4.6 – 4.8 ──
	"claude-opus-4-8",
	"claude-opus-4-8-max",
	"claude-opus-4-8-xhigh",
	"claude-opus-4-8-high",
	"claude-opus-4-8-medium",
	"claude-opus-4-8-low",
	"claude-opus-4-8-thinking",
	"claude-opus-4-7",
	"claude-opus-4-7-max",
	"claude-opus-4-7-xhigh",
	"claude-opus-4-7-high",
	"claude-opus-4-7-medium",
	"claude-opus-4-7-low",
	"claude-opus-4-7-thinking",
	"claude-opus-4-6",
	"claude-opus-4-6-max",
	"claude-opus-4-6-high",
	"claude-opus-4-6-medium",
	"claude-opus-4-6-low",
	"claude-sonnet-4-6",
	"claude-sonnet-4-6-thinking",

	// ── Claude 4.0 – 4.5 (dated snapshots) ──
	"claude-haiku-4-5-20251001",
	"claude-opus-4-5-20251101",
	"claude-opus-4-5-20251101-thinking",
	"claude-sonnet-4-5-20250929",
	"claude-sonnet-4-5-20250929-thinking",
}

var ChannelName = "claude"
