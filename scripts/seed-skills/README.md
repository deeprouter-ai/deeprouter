# Skill Seed — DeepSeek Starter Pack

Seeds 3 DeepSeek-powered Skills into the marketplace database.

## Skills

| Slug | Name | Model | Category |
|------|------|-------|----------|
| `academic-polish` | Academic Paper Polish | deepseek-chat | writing |
| `code-review-ds` | Code Review Expert | deepseek-coder | coding |
| `smart-translate` | Smart Translation Engine | deepseek-chat | translation |

## What customers download

When a user downloads a skill from the marketplace, the ZIP contains:

```
<slug>.zip
├── manifest.json         ← auto-generated from DB
├── SKILL.md              ← auto-generated from DB description
├── scripts/
│   └── polish.py         ← runnable Python 3 script
└── reference/
    └── *.md / *.txt      ← style guides, checklists, glossaries
```

The `scripts/` and `reference/` files are embedded in the Go binary at
`internal/skill/packages/<slug>/` and injected into the ZIP at download time.

## Prerequisites

```bash
pip install openai   # for customers running the downloaded scripts
```

## Running the seed

### PostgreSQL (production / dev compose)

```bash
# Using psql
psql $DATABASE_URL -f scripts/seed-skills/seed.sql

# Using the dev compose database
docker compose exec postgres psql -U postgres -d new_api -f /dev/stdin < scripts/seed-skills/seed.sql
```

### MySQL

Replace the three `ON CONFLICT (slug) DO NOTHING` clauses with no clause,
and change `INSERT INTO` to `INSERT IGNORE INTO` for each statement.
Replace `NOW()` stays the same.

```bash
mysql -u root -p new_api < scripts/seed-skills/seed_mysql.sql
```

### SQLite (local dev without Docker)

```bash
sqlite3 ./data/new_api.db < scripts/seed-skills/seed_sqlite.sql
```

## Verify

After seeding, check the marketplace API:

```bash
curl http://localhost:3300/api/v1/marketplace/skills | jq '.data[] | .slug'
# Expected:
# "academic-polish"
# "code-review-ds"
# "smart-translate"
```

## Re-running

The seed is idempotent. Running it twice does not create duplicates.
The `ON CONFLICT (slug) DO NOTHING` clause silently skips existing rows.

## Admin note

The seed sets `created_by = 1` (platform admin user). Change this value
if your admin user has a different ID.
