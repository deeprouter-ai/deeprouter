package mistral

// Mistral publishes rolling "-latest" aliases; they always point at the current
// snapshot (mistral-large-latest → Large 3, mistral-medium-latest → Medium 3.5,
// mistral-small-latest → Small 4), so this list does not need per-release edits.
var ModelList = []string{
	"mistral-large-latest",
	"mistral-medium-latest",
	"mistral-small-latest",
	"magistral-medium-latest",
	"magistral-small-latest",
	"codestral-latest",
	// dated IDs, as listed on Mistral's model page (2026-10-04)
	"mistral-medium-2604", "mistral-small-2603", "mistral-large-2512", "codestral-2508",
	"pixtral-large-latest",
	"ministral-8b-latest",
	"ministral-3b-latest",
	"open-mistral-7b",
	"open-mixtral-8x7b",
	"mistral-embed",
}

var ChannelName = "mistral"
