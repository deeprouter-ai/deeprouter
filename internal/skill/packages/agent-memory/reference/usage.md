# Agent Memory — Usage Guide

## Installation

```bash
pip install openai
export DEEPROUTER_API_KEY=sk-...      # Required for AI features
export MEMORY_DB_PATH=~/.agent-memory/memory.db  # Optional, this is the default
```

---

## Command Reference

### `remember` — Store a fact
```bash
python memory.py remember "Content goes here" --tags tag1,tag2 --expires 30
```
- `--tags` comma-separated (optional)
- `--expires N` auto-delete after N days (optional)
- Returns: fact ID (12-char hex)

### `recall` — Keyword search
```bash
python memory.py recall "boss communication style" --limit 10
```
- Uses SQLite FTS5 full-text search with Porter stemming
- Returns matching facts sorted by relevance

### `smart-recall` — AI-powered semantic search
```bash
python memory.py smart-recall "how should I communicate with my manager"
```
- First does keyword recall (top 20 candidates)
- Then sends to DeepRouter to rank by semantic relevance
- Returns top 3-5 with relevance explanations
- Uses `DEEPROUTER_API_KEY`

### `learn` — Record an experience
```bash
python memory.py learn "Sent long Slack message to CEO" \
  --context "company communication" \
  --outcome negative \
  --insight "CEO prefers 3-bullet summaries, not paragraphs"
```
- `--outcome` must be: `positive`, `negative`, or `neutral`

### `synthesize` — AI summary of all memories
```bash
python memory.py synthesize --model deepseek-chat
```
- Sends all facts + lessons to DeepRouter
- Returns: key facts, patterns, lessons, suggested actions
- Best run weekly or after a large batch of `remember` calls

### `suggest` — AI improvement suggestions
```bash
python memory.py suggest
```
- Analyzes negative lessons only
- Returns: systemic fixes, CLAUDE.md rules, automation candidates

### `stats` — Quick statistics
```bash
python memory.py stats
# Output: Active facts: 47  |  Lessons: 12
# DB: /home/user/.agent-memory/memory.db
```

### `forget-stale` — Clean up unused facts
```bash
python memory.py forget-stale --days 30
```
- Deletes facts not accessed in N days AND with access_count = 1
- Facts recalled even once are kept (they proved useful)

---

## Integration with Claude Code

Add to your CLAUDE.md:
```markdown
## Memory
Before starting work on a complex task, run:
  python /path/to/memory.py recall "<topic>"
  
After completing a task, record lessons:
  python /path/to/memory.py learn "<what I did>" \
    --context "<project>" --outcome <outcome> --insight "<what I learned>"
```

Or add as a hook in `.claude/settings.json`:
```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": "Bash",
      "hooks": [{"type": "command",
        "command": "python ~/.agent-memory/memory.py recall \"recent context\" 2>/dev/null || true"}]
    }]
  }
}
```

---

## Data Format

### Facts database (SQLite)
```
facts table:
  id           TEXT  12-char hex hash
  content      TEXT  The stored fact
  tags         TEXT  JSON array ["tag1","tag2"]
  source       TEXT  "conversation" | custom
  confidence   REAL  0.0-1.0
  created_at   TEXT  ISO 8601 UTC
  last_accessed TEXT ISO 8601 UTC
  access_count INTEGER
  expires_at   TEXT  ISO 8601 UTC or NULL
  superseded_by TEXT fact ID or NULL
```

### Lessons database
```
lessons table:
  id           TEXT  12-char hex hash
  action       TEXT  What was done
  context      TEXT  In what situation
  outcome      TEXT  "positive"|"negative"|"neutral"
  insight      TEXT  What was learned
  created_at   TEXT  ISO 8601 UTC
  applied_count INTEGER
```

---

## Tips

**Tag consistently** — tags power both filtering and AI synthesis:
```bash
python memory.py remember "..." --tags work,manager,communication
python memory.py remember "..." --tags code,python,async
```

**Record both good and bad lessons** — `synthesize` and `suggest` need contrast:
```bash
# Negative
python memory.py learn "Used global mutable state" --context "Go service" \
  --outcome negative --insight "Caused race condition under load"

# Positive  
python memory.py learn "Added integration test before refactor" \
  --context "payment module" --outcome positive \
  --insight "Caught 3 regressions before they reached prod"
```

**Use `--expires` for time-sensitive facts:**
```bash
python memory.py remember "Sprint ends Friday, no new features after Wednesday" \
  --tags sprint,deadline --expires 7
```
