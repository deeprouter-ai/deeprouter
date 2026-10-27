package mcp

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// publishedGuides is the directory the web app publishes at
// /docs/integrations/; the gateway embeds its build for get_connection_config.
const publishedGuides = "../../web/default/public/docs/integrations"

// TestGuides_EveryPublishedClientIsServedVerbatim is acceptance B1 (meta-repo
// docs/deeprouter-mcp-prd.md §7): get_connection_config covers every client
// guide and returns its "Facts for AI assistants" block exactly as written.
//
// There is no second table to fall out of step: the tool reads the guides
// themselves. So this pins what can still go wrong - a guide whose block
// cannot be found or parsed, which would drop that client from the tool.
func TestGuides_EveryPublishedClientIsServedVerbatim(t *testing.T) {
	shelf, err := loadGuides(os.DirFS(publishedGuides))
	require.NoError(t, err)

	files, err := filepath.Glob(filepath.Join(publishedGuides, "*.md"))
	require.NoError(t, err)
	var clients []string
	for _, f := range files {
		name := filepath.Base(f)
		if strings.HasSuffix(name, ".zh.md") || name == "GUIDE.md" || name == "README.md" {
			continue
		}
		clients = append(clients, strings.TrimSuffix(name, ".md"))
	}
	// The PRD counts 23 client guides; mcp.md, how to add this server to a
	// client, is the 24th (P2). A guide added or removed changes this on purpose.
	require.Len(t, clients, 24)
	require.Contains(t, clients, "mcp")
	require.Equal(t, clients, shelf.slugs)

	block := regexp.MustCompile("(?s)Facts for AI assistants.*?```yaml\r?\n(.*?)```")
	d := &desk{guides: shelf}
	for _, client := range clients {
		raw, err := os.ReadFile(filepath.Join(publishedGuides, client+".md"))
		require.NoError(t, err)
		m := block.FindSubmatch(raw)
		require.NotNil(t, m, "%s has no Facts for AI assistants block", client)
		want := strings.ReplaceAll(string(m[1]), "\r\n", "\n")

		_, out, err := d.connectionConfig(context.Background(), nil, connectionConfigInput{Client: client})
		require.NoError(t, err)
		require.Equal(t, client, out.Client)
		require.Equal(t, want, out.Facts, client)
		require.NotEmpty(t, out.Fields, client)
		require.Equal(t, guidesURL+client+".md", out.Guide)
		require.Empty(t, out.Clients)
	}
}

// TestGuides_ABrokenGuideCostsOnlyItself checks that one guide without a facts
// block is named in the error and left out, while the others still load; and
// that the Chinese copies, GUIDE and README are not taken for clients.
func TestGuides_ABrokenGuideCostsOnlyItself(t *testing.T) {
	fsys := fstest.MapFS{
		"good.md":    {Data: []byte("# Good\r\n\r\n## Facts for AI assistants\r\n\r\n```yaml\r\ntool: good\r\nbase_url: \"https://api.deeprouter.co/v1\"\r\n```\r\n")},
		"good.zh.md": {Data: []byte("no block, and not a client")},
		"GUIDE.md":   {Data: []byte("not about one client")},
		"broken.md":  {Data: []byte("# Broken\n\nno block here\n")},
	}
	shelf, err := loadGuides(fsys)
	require.ErrorContains(t, err, "broken.md")
	require.Equal(t, []string{"good"}, shelf.slugs)
	g, ok := shelf.find("good")
	require.True(t, ok)
	require.Equal(t, "tool: good\nbase_url: \"https://api.deeprouter.co/v1\"\n", g.facts)
	require.Equal(t, "good", g.fields["tool"])

	_, err = loadGuides(nil)
	require.Error(t, err)
	_, err = loadGuides(fstest.MapFS{})
	require.Error(t, err)
}

// TestGuides_FindAClientByTheNameAPersonTypes checks that "Claude Code",
// "claude_code" and "claude-code.md" all find the claude-code guide.
func TestGuides_FindAClientByTheNameAPersonTypes(t *testing.T) {
	shelf := &guideShelf{bySlug: map[string]guide{"claude-code": {slug: "claude-code"}}}
	for _, name := range []string{"claude-code", "Claude Code", "claude_code", " CLAUDE-CODE.md "} {
		_, ok := shelf.find(name)
		require.True(t, ok, name)
	}
	_, ok := shelf.find("claude")
	require.False(t, ok)
}

// TestConnectionConfig_ListsTheClientsWhenItCannotAnswer checks the answers
// that are not a guide: no client named, an unknown client, no guides at all.
func TestConnectionConfig_ListsTheClientsWhenItCannotAnswer(t *testing.T) {
	shelf, err := loadGuides(os.DirFS(publishedGuides))
	require.NoError(t, err)
	d := &desk{guides: shelf}

	_, out, err := d.connectionConfig(context.Background(), nil, connectionConfigInput{})
	require.NoError(t, err)
	require.Equal(t, shelf.slugs, out.Clients)
	require.Empty(t, out.Facts)

	_, out, err = d.connectionConfig(context.Background(), nil, connectionConfigInput{Client: "no-such-app"})
	require.NoError(t, err)
	require.Contains(t, out.Message, "no-such-app")
	require.Contains(t, out.Clients, "others")

	empty := &desk{guides: &guideShelf{bySlug: map[string]guide{}}}
	_, out, err = empty.connectionConfig(context.Background(), nil, connectionConfigInput{Client: "codex"})
	require.NoError(t, err)
	require.Contains(t, out.Message, "llms.txt")
}
