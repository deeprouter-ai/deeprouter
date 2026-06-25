package kids

import (
	"strings"
	"testing"
)

func TestCheckOutputText_BlocksBlocklistKeyword(t *testing.T) {
	if len(StrictOutputBlocklist) == 0 {
		t.Fatal("StrictOutputBlocklist must be non-empty at V0 (AC1 \"applied\", §3.1/§3.3) — an empty list would make StrictKeywordFilter permanently non-blocking")
	}

	cases := []struct {
		category string
		text     string
	}{
		{OutputCategoryNSFW, "Here is some porn for you."},
		{OutputCategoryViolence, "I will kill you if you do that again."},
		{OutputCategoryHate, "Those people are subhuman."},
	}
	for _, tc := range cases {
		t.Run(tc.category, func(t *testing.T) {
			got := CheckOutputText(tc.text)
			if !got.Blocked {
				t.Fatalf("CheckOutputText(%q) = %+v, want Blocked=true", tc.text, got)
			}
			found := false
			for _, c := range got.Categories {
				if c == tc.category {
					found = true
				}
			}
			if !found {
				t.Errorf("CheckOutputText(%q).Categories = %v, want to contain %q", tc.text, got.Categories, tc.category)
			}
		})
	}

	// Case-insensitive substring match against a real (non-empty) blocklist entry.
	upper := strings.ToUpper(StrictOutputBlocklist[0])
	if got := CheckOutputText("RANDOM " + upper + " TEXT"); !got.Blocked {
		t.Errorf("CheckOutputText with uppercase keyword %q = %+v, want Blocked=true (case-insensitive match)", upper, got)
	}
}

func TestCheckOutputText_AllowsCleanText(t *testing.T) {
	got := CheckOutputText("The weather today is sunny and the cat is sleeping on the porch.")
	if got.Blocked {
		t.Errorf("CheckOutputText(clean text) = %+v, want Blocked=false", got)
	}
	if len(got.Categories) != 0 {
		t.Errorf("CheckOutputText(clean text).Categories = %v, want empty", got.Categories)
	}
}

func TestSafeFallbackText_NonEmpty(t *testing.T) {
	if got := SafeFallbackText(); len(got) < 10 {
		t.Errorf("SafeFallbackText() too short, got %q", got)
	}
}

func TestStrictKeywordFilter_ImplementsOutputFilter(t *testing.T) {
	var f OutputFilter = StrictKeywordFilter{}

	const cleanText = "a perfectly ordinary sentence"
	if got := f.Check(cleanText); got.Blocked {
		t.Errorf("StrictKeywordFilter.Check(%q) = %+v, want Blocked=false", cleanText, got)
	}

	const blockedText = "Here is some porn for you."
	got := f.Check(blockedText)
	want := CheckOutputText(blockedText)
	if !got.Blocked || got.Blocked != want.Blocked || len(got.Categories) != len(want.Categories) {
		t.Errorf("StrictKeywordFilter.Check(%q) = %+v, want delegation to CheckOutputText = %+v", blockedText, got, want)
	}
}
