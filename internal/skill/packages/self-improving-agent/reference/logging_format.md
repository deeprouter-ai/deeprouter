# Self-Improving Agent — Logging Format Guide

## Directory Structure

```
.learnings/
├── LEARNINGS.md        General lessons learned
├── ERRORS.md           Mistakes and failures  
└── FEATURE_REQUESTS.md Ideas and feature requests
```

Initialize:
```bash
mkdir -p .learnings
touch .learnings/LEARNINGS.md .learnings/ERRORS.md .learnings/FEATURE_REQUESTS.md
```

---

## LEARNINGS.md Format

### Entry format
```markdown
## LRN-YYYYMMDD-001 [STATUS]

**What:** Short description of what was learned
**Context:** Where/when this applies
**Why it matters:** Impact on work quality
**How to apply:** Concrete action for next time

---
```

### STATUS values
- `[active]` — currently relevant
- `[promoted]` — added to CLAUDE.md/AGENTS.md permanently  
- `[superseded]` — replaced by a newer lesson
- `[context-specific]` — only applies to one project

### Example
```markdown
## LRN-20250615-001 [active]

**What:** Always read AGENTS.md before creating new files in Go packages
**Context:** DeepRouter project, any new Go service
**Why it matters:** AGENTS.md has package conventions that prevent CI failures
**How to apply:** First command on any new file task: `cat AGENTS.md | head -100`

---

## LRN-20250615-002 [promoted]

**What:** Use `ON CONFLICT DO NOTHING` for seed SQL instead of plain INSERT
**Context:** PostgreSQL seed scripts  
**Why it matters:** Makes seeds idempotent; safe to re-run in CI
**How to apply:** All seed INSERT statements must include conflict clause

---
```

---

## ERRORS.md Format

### Entry format
```markdown
## ERR-YYYYMMDD-001 [STATUS]

**Error:** What went wrong (1 line)
**Root cause:** Why it happened (not symptoms)
**Impact:** What broke or was wasted
**Fix applied:** What was done to resolve
**Prevention:** How to avoid next time

---
```

### STATUS values
- `[open]` — not yet resolved
- `[resolved]` — fix applied
- `[recurring]` — happened more than once (priority for systemic fix)
- `[wontfix]` — accepted risk

### Example
```markdown
## ERR-20250614-001 [recurring]

**Error:** go:embed directive failed with "no matching files found"
**Root cause:** embed cannot traverse upward with ../; must be at same package level
**Impact:** 45 min debugging; CI build failed
**Fix applied:** Moved embed declaration to parent package (packages.go at skill/ level)
**Prevention:** Any new embed directive — verify package path alignment before running go build

---
```

---

## FEATURE_REQUESTS.md Format

### Entry format
```markdown
## FEAT-YYYYMMDD-001 [STATUS]

**Feature:** Short title
**Request:** Detailed description
**Why:** Business or UX justification
**Priority:** high|medium|low
**Effort estimate:** S|M|L|XL

---
```

### STATUS values
- `[backlog]` — noted, not scheduled
- `[planned]` — in sprint planning
- `[in-progress]` — being implemented
- `[done]` — shipped
- `[rejected]` — decided not to build (include reason)

---

## Analysis Script Usage

```bash
# Full analysis (all files)
python analyze_learnings.py

# Errors only (for incident review)
python analyze_learnings.py --focus errors

# Feature requests (for sprint planning)
python analyze_learnings.py --focus features

# Find what to promote to CLAUDE.md
python analyze_learnings.py --action promote

# Different project directory
python analyze_learnings.py --dir /path/to/project/.learnings
```

---

## Integration with Claude Code Hooks

Add to `.claude/settings.json` to auto-analyze at end of session:
```json
{
  "hooks": {
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "python /path/to/analyze_learnings.py --focus errors 2>&1 | head -50"
      }]
    }]
  }
}
```

---

## Promotion Workflow

When `--action promote` identifies entries to add to CLAUDE.md:

1. Copy the suggested Markdown blocks to `CLAUDE.md`
2. Mark promoted entries: change `[active]` → `[promoted]` in LEARNINGS.md
3. Commit both files together

This ensures CLAUDE.md stays current and LEARNINGS.md stays clean.
