# Skill Content — DeepSeek Starter Pack (3 Skills) — PRD

**Status:** In Progress
**Priority:** P0
**Author:** DeepRouter Engineering
**Created:** 2026-06-22
**Branch:** feat/skill-content-deepseek

---

## 1. Background

The Skill Marketplace infrastructure is complete but the catalog is empty.
This task seeds the first 3 published Skills, all powered by DeepSeek API,
giving operators and users a working marketplace on launch day.

All three skills are "download and run locally" type: the user downloads a ZIP
containing `SKILL.md`, `scripts/`, and `reference/` directories, sets their own
DeepSeek API key, and runs the bundled Python script.

---

## 2. Skills

| # | Slug | Name | Model | Category |
|---|------|------|-------|----------|
| 1 | `academic-polish` | Academic Paper Polish | deepseek-chat | writing |
| 2 | `code-review-ds` | Code Review Expert | deepseek-coder | coding |
| 3 | `smart-translate` | Smart Translation Engine | deepseek-chat | translation |

---

## 3. Package Format

Each downloaded ZIP contains:

```
<slug>.zip
├── manifest.json          # auto-generated from DB
├── SKILL.md               # auto-generated + instructions
├── scripts/
│   └── <main>.py          # runnable Python 3 script
└── reference/
    └── *.md / *.txt       # style guides, checklists, glossaries
```

The extra files (`scripts/`, `reference/`) are stored as Go-embedded assets
in `internal/skill/packages/<slug>/` and injected into the ZIP at download
time by `buildSkillPackage`.

---

## 4. Technical Changes

### 4.1 Go — `internal/skill/packages/`
Add `//go:embed` directory. `buildSkillPackage` walks the embedded FS for the
matching slug and appends all files to the ZIP.

### 4.2 SQL Seed — `scripts/seed-skills/seed.sql`
Direct INSERT for skills + skill_versions tables. Run once against the target DB.
Admin user ID 1 is assumed (configurable via sed/env).

---

## 5. Acceptance Criteria

- [ ] `GET /api/v1/marketplace/skills` returns 3 published skills
- [ ] `GET /api/v1/marketplace/skills/academic-polish/download` returns a valid ZIP
- [ ] ZIP contains `manifest.json`, `SKILL.md`, `scripts/polish.py`,
      `reference/style_guide.md`
- [ ] Same structure for other two skills
- [ ] Python scripts run cleanly: `python scripts/polish.py --help` exits 0
- [ ] `DEEPSEEK_API_KEY=xxx python scripts/polish.py sample.txt` calls DeepSeek API
- [ ] `bun run build` passes (no Go compile errors)

---

## 6. Dependencies

| Dependency | Status |
|------------|--------|
| Skill marketplace DB tables (DR-41/DR-55) | Merged |
| Download endpoint (`/api/v1/marketplace/skills/:id/download`) | Merged |
| DR-75 analytics API | To Do (no blocker here) |
