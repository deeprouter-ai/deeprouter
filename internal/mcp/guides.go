package mcp

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// factsHeading names the section of an integration guide written for AI
// assistants: one YAML block with the exact base URL, auth header and model
// for that client.
const factsHeading = "Facts for AI assistants"

// guideShelf is the integration guides this gateway publishes, read once at
// start-up. get_connection_config answers from them directly, so its answer
// cannot drift from the guide a person reads at /docs/integrations/<client>.md.
type guideShelf struct {
	bySlug map[string]guide
	slugs  []string
}

// guide is the machine-readable part of one published guide.
type guide struct {
	slug   string
	facts  string         // the YAML block, verbatim
	fields map[string]any // the same block, parsed
}

// loadGuides reads every English guide in fsys. A guide that cannot be read
// is left out and named in the returned error, so one broken file costs one
// client rather than all of them.
func loadGuides(fsys fs.FS) (*guideShelf, error) {
	shelf := &guideShelf{bySlug: map[string]guide{}}
	if fsys == nil {
		return shelf, errors.New("no guides were given")
	}
	names, err := fs.Glob(fsys, "*.md")
	if err != nil {
		return shelf, err
	}
	var problems []error
	for _, name := range names {
		slug := strings.TrimSuffix(name, ".md")
		// The Chinese guides repeat the same block; GUIDE and README are not
		// about one client.
		if strings.HasSuffix(slug, ".zh") || slug == "GUIDE" || slug == "README" {
			continue
		}
		g, err := readGuide(fsys, name)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		shelf.bySlug[slug] = g
		shelf.slugs = append(shelf.slugs, slug)
	}
	sort.Strings(shelf.slugs)
	if len(shelf.slugs) == 0 && len(problems) == 0 {
		problems = append(problems, errors.New("no integration guides found"))
	}
	return shelf, errors.Join(problems...)
}

// readGuide reads one guide's facts block.
func readGuide(fsys fs.FS, name string) (guide, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return guide{}, err
	}
	facts, ok := factsBlock(string(data))
	if !ok {
		return guide{}, fmt.Errorf("%s has no %q YAML block", name, factsHeading)
	}
	fields := map[string]any{}
	if err := yaml.Unmarshal([]byte(facts), &fields); err != nil {
		return guide{}, fmt.Errorf("%s: %w", name, err)
	}
	return guide{slug: strings.TrimSuffix(name, ".md"), facts: facts, fields: fields}, nil
}

// factsBlock returns the first ```yaml block after the facts heading, without
// its fences.
func factsBlock(markdown string) (string, bool) {
	text := strings.ReplaceAll(markdown, "\r\n", "\n")
	at := strings.Index(text, factsHeading)
	if at < 0 {
		return "", false
	}
	const fence = "```yaml\n"
	rest := text[at:]
	open := strings.Index(rest, fence)
	if open < 0 {
		return "", false
	}
	rest = rest[open+len(fence):]
	end := strings.Index(rest, "```")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

// find returns the guide for a client named the way a person or an AI might
// write it: "Claude Code", "claude_code" and "claude-code" are one client.
func (s *guideShelf) find(client string) (guide, bool) {
	slug := strings.ToLower(strings.TrimSpace(client))
	slug = strings.NewReplacer(" ", "-", "_", "-").Replace(slug)
	slug = strings.TrimSuffix(slug, ".md")
	g, ok := s.bySlug[slug]
	return g, ok
}
