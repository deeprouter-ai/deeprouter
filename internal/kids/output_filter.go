package kids

import "strings"

// OutputVerdict is the result of classifying a chunk of response text
// against the kids_mode strict output filter (PRD §6.4-pre: NSFW + violence
// + hate, "强制最严档，不允许 tenant 配置宽松档").
type OutputVerdict struct {
	Blocked    bool
	Categories []string // e.g. ["nsfw", "violence", "hate"]
}

// OutputFilter classifies accumulated response text for kids_mode hard
// constraints. CheckOutputText / StrictKeywordFilter (below) is the V0
// baseline implementation. DRS-7 will provide an LLM-backed or
// Airbotix-classifier-backed implementation behind this same interface
// (PRD §5.2: "复用 Airbotix Kids 安全 classifier 接口") — no other code
// needs to change when that lands.
type OutputFilter interface {
	Check(text string) OutputVerdict
}

// Output filter categories, used in OutputVerdict.Categories and as the
// values of strictOutputBlocklistCategory below.
const (
	OutputCategoryNSFW     = "nsfw"
	OutputCategoryViolence = "violence"
	OutputCategoryHate     = "hate"
)

// StrictOutputBlocklist is the V0 keyword list for the kids_mode strict
// output filter. Pure substring match, case-insensitive. MUST be non-empty
// at merge time: a small seed list covering NSFW / violence / hate (a few
// representative terms per category is sufficient) so that AC1 ("Strict
// output filter applied", §3.1/§3.3) is substantively true at V0 — not just
// mechanically wired with a filter that can never match anything. The seed
// list is intentionally narrow; the full/refined word list is a
// security/compliance review follow-up (§12 item 2). DRS-7 supersedes this
// entire mechanism with a real classifier.
var StrictOutputBlocklist = []string{
	// nsfw
	"porn",
	"explicit sex",
	// violence
	"kill you",
	"murder",
	// hate
	"subhuman",
	"ethnic cleansing",
}

// strictOutputBlocklistCategory maps each StrictOutputBlocklist entry to the
// category reported in OutputVerdict.Categories.
var strictOutputBlocklistCategory = map[string]string{
	"porn":             OutputCategoryNSFW,
	"explicit sex":     OutputCategoryNSFW,
	"kill you":         OutputCategoryViolence,
	"murder":           OutputCategoryViolence,
	"subhuman":         OutputCategoryHate,
	"ethnic cleansing": OutputCategoryHate,
}

// CheckOutputText scans text against StrictOutputBlocklist. Matching is a
// case-insensitive substring search. Categories lists each distinct category
// (nsfw/violence/hate) with at least one matching keyword, in
// StrictOutputBlocklist order.
func CheckOutputText(text string) OutputVerdict {
	lower := strings.ToLower(text)
	seen := make(map[string]bool, len(strictOutputBlocklistCategory))
	var categories []string
	for _, keyword := range StrictOutputBlocklist {
		if !strings.Contains(lower, strings.ToLower(keyword)) {
			continue
		}
		category := strictOutputBlocklistCategory[keyword]
		if seen[category] {
			continue
		}
		seen[category] = true
		categories = append(categories, category)
	}
	return OutputVerdict{
		Blocked:    len(categories) > 0,
		Categories: categories,
	}
}

// StrictKeywordFilter is the V0 default OutputFilter implementation
// (§3.1/§3.3): a keyword-blocklist baseline, enabled for kids_mode=true via
// policy.Decision.EnforceStrictOutputFilter. DRS-7 will replace this with a
// real classifier behind the same OutputFilter interface — no other code
// needs to change when that lands.
type StrictKeywordFilter struct{}

func (StrictKeywordFilter) Check(text string) OutputVerdict {
	return CheckOutputText(text)
}

// SafeFallbackText returns the D-DR6 "polite refusal" template used to
// replace blocked output. V0 uses a static template (not regeneration),
// per PRD §9 D-DR6.
func SafeFallbackText() string {
	return "I can't share that response. Let's try a different question — what else would you like to know or explore?"
}
