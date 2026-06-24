-- Seed: 3 DeepSeek-powered Skills for the DeepRouter Marketplace
-- Target: deeprouter-ai/deeprouter (all three databases: PG / MySQL / SQLite)
-- Run:
--   psql $DATABASE_URL -f seed.sql
--   mysql -u root -p new_api < seed.sql
--   sqlite3 ./data/new_api.db < seed.sql
--
-- Idempotent: ON CONFLICT (slug) DO NOTHING / INSERT IGNORE / INSERT OR IGNORE
-- All three skills are "legacy" type (no active_version_id) — customers supply
-- their own DeepSeek API key and run the bundled Python scripts locally.
--
-- UUIDs are hardcoded for referential stability across environments.

-- ─── PostgreSQL ──────────────────────────────────────────────────────────────
-- (Wrap in a DO block so MySQL/SQLite parsers can skip it.)
-- For MySQL / SQLite, use the INSERT IGNORE / INSERT OR IGNORE variants below.

-- ── Skill 1: Academic Paper Polish ──────────────────────────────────────────

INSERT INTO skills (
    id, slug, status, category, tags, default_locale,
    name, short_description, description,
    input_hints, example_inputs, example_outputs,
    required_plan, monetization_type, price_markup,
    model_whitelist, timeout_seconds, timeout_risk,
    is_kids_safe, is_kids_exclusive, kids_approval_status,
    ai_disclosure_required, featured_flag, featured_rank,
    active_version_id, created_by, created_at, updated_at, published_at
)
VALUES (
    'a1b2c3d4-0001-0001-0001-000000000001',
    'academic-polish',
    'published',
    'writing',
    '["academic","writing","editing","research","English","Chinese"]',
    'en',
    'Academic Paper Polish',
    'AI-powered academic editing that polishes grammar, clarity, and register for journal submission.',
    E'Elevate your research manuscripts with expert AI editing powered by DeepSeek Chat.\n\n'
    E'This skill polishes academic text while strictly preserving your original argument, data,\n'
    E'and citations. It fixes grammatical errors, eliminates redundancy, strengthens sentence\n'
    E'clarity, and ensures consistent academic register — producing publication-ready prose for\n'
    E'journals including Nature, Science, IEEE, and ACM.\n\n'
    E'**Supports:** English and Simplified Chinese manuscripts.\n\n'
    E'## Setup\n\n'
    E'1. Install Python 3.9+ and run: `pip install openai`\n'
    E'2. Get your DeepRouter API key at <https://deeprouter.co> → Dashboard → API Keys\n'
    E'3. Set your key: `export DEEPROUTER_API_KEY=sk-...`\n\n'
    E'## Quick Start\n\n'
    E'```bash\n'
    E'# Polish an English manuscript\n'
    E'python scripts/polish.py paper.txt\n\n'
    E'# Polish a Chinese manuscript\n'
    E'python scripts/polish.py paper.txt --lang zh\n\n'
    E'# Save polished output\n'
    E'python scripts/polish.py paper.txt --out polished.txt\n\n'
    E'# Interactive mode (paste text directly)\n'
    E'python scripts/polish.py --interactive\n'
    E'```\n\n'
    E'## What Gets Edited\n\n'
    E'- Grammar and awkward phrasing\n'
    E'- Overly verbose or repetitive sentences\n'
    E'- Informal language replaced with precise academic terms\n'
    E'- Inconsistent verb tense within sections\n'
    E'- Weak paragraph transitions\n\n'
    E'## What Is Preserved\n\n'
    E'- All original data, claims, and argument structure\n'
    E'- Citation keys and reference numbers\n'
    E'- Technical terms, model names, and variable names\n'
    E'- Heading levels and paragraph structure\n\n'
    E'See `reference/style_guide.md` for the full editing ruleset the AI applies.',
    '["Polish an English academic paper for journal submission","Improve grammar and clarity of a research abstract","Fix verb tense consistency throughout a manuscript","Remove informal language from a thesis chapter","Polish a Chinese academic paper (--lang zh)"]',
    '["The proposed algorithm utilize a hierarchical approach to reduces the computational complexity, which result in more efficient performance comparing to existing methods."]',
    '["The proposed algorithm employs a hierarchical approach to reduce computational complexity, resulting in superior efficiency compared to existing methods."]',
    'free',
    'free',
    0.0000,
    '["deepseek-chat"]',
    90,
    false,
    true,
    false,
    'not_required',
    true,
    true,
    1,
    NULL,
    1,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO NOTHING;

-- ── Skill 2: Code Review Expert ──────────────────────────────────────────────

INSERT INTO skills (
    id, slug, status, category, tags, default_locale,
    name, short_description, description,
    input_hints, example_inputs, example_outputs,
    required_plan, monetization_type, price_markup,
    model_whitelist, timeout_seconds, timeout_risk,
    is_kids_safe, is_kids_exclusive, kids_approval_status,
    ai_disclosure_required, featured_flag, featured_rank,
    active_version_id, created_by, created_at, updated_at, published_at
)
VALUES (
    'a1b2c3d4-0002-0002-0002-000000000002',
    'code-review-ds',
    'published',
    'coding',
    '["code review","security","bugs","DeepSeek Coder","Python","Go","TypeScript"]',
    'en',
    'Code Review Expert',
    'Deep multi-dimensional code review: correctness, security, performance, and maintainability.',
    E'Catch bugs, security vulnerabilities, and performance issues before they reach production —\n'
    E'powered by DeepSeek Coder, a model fine-tuned on hundreds of millions of lines of real-world code.\n\n'
    E'The review covers five dimensions in a single pass:\n\n'
    E'| Dimension | What it finds |\n'
    E'|-----------|---------------|\n'
    E'| Correctness | Logic errors, null deref, resource leaks, off-by-one |\n'
    E'| Security | Injection, hard-coded secrets, path traversal, insecure crypto |\n'
    E'| Performance | N+1 queries, O(n²) loops, unnecessary allocations |\n'
    E'| Maintainability | Duplicated logic, magic numbers, missing error types |\n'
    E'| Style | Dead code, unused imports, formatting inconsistency |\n\n'
    E'**Supports 30+ languages:** Python, Go, TypeScript, Java, Rust, C/C++, Ruby, PHP, and more.\n\n'
    E'## Setup\n\n'
    E'1. Install Python 3.9+ and run: `pip install openai`\n'
    E'2. Get your DeepRouter API key at <https://deeprouter.co> → Dashboard → API Keys\n'
    E'3. Set your key: `export DEEPROUTER_API_KEY=sk-...`\n\n'
    E'## Quick Start\n\n'
    E'```bash\n'
    E'# Review a single file\n'
    E'python scripts/review.py main.go\n\n'
    E'# Review an entire directory (recursive)\n'
    E'python scripts/review.py ./src/\n\n'
    E'# Security-only review\n'
    E'python scripts/review.py ./src/ --focus security\n\n'
    E'# Save report to Markdown\n'
    E'python scripts/review.py ./src/ --out review_report.md\n\n'
    E'# Strict mode: exit 1 if CRITICAL or HIGH issues found (great for CI)\n'
    E'python scripts/review.py ./src/ --strict\n'
    E'```\n\n'
    E'## Output Format\n\n'
    E'The report is Markdown with a **Summary** paragraph, then per-issue sections:\n\n'
    E'```\n'
    E'### [HIGH] SQL query built by string concatenation\n'
    E'- File/Line: `service/user.go:42`\n'
    E'- Category: Security\n'
    E'- Description: User-supplied input is concatenated directly into the query string...\n'
    E'- Suggested Fix: (parameterised query snippet)\n'
    E'```\n\n'
    E'See `reference/review_checklist.md` for the full security checklist the reviewer applies.',
    '["Find security vulnerabilities in a Python web service","Review a Go microservice for correctness and performance","Check TypeScript code for type safety and null handling","Run in CI to block CRITICAL issues from merging","Get a security-only review of authentication code"]',
    '["func GetUser(db *sql.DB, username string) (*User, error) { query := \"SELECT * FROM users WHERE username = ''\" + username + \"''\"; ... }"]',
    '["### [CRITICAL] SQL injection via string concatenation\n- File/Line: `handler.go:12`\n- Category: Security\n- Description: The username parameter is concatenated directly into the SQL query...\n- Suggested Fix: Use db.QueryRow(\"SELECT * FROM users WHERE username = ?\", username)"]',
    'free',
    'free',
    0.0000,
    '["deepseek-coder"]',
    120,
    true,
    false,
    false,
    'not_required',
    true,
    true,
    2,
    NULL,
    1,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO NOTHING;

-- ── Skill 3: Smart Translation Engine ────────────────────────────────────────

INSERT INTO skills (
    id, slug, status, category, tags, default_locale,
    name, short_description, description,
    input_hints, example_inputs, example_outputs,
    required_plan, monetization_type, price_markup,
    model_whitelist, timeout_seconds, timeout_risk,
    is_kids_safe, is_kids_exclusive, kids_approval_status,
    ai_disclosure_required, featured_flag, featured_rank,
    active_version_id, created_by, created_at, updated_at, published_at
)
VALUES (
    'a1b2c3d4-0003-0003-0003-000000000003',
    'smart-translate',
    'published',
    'translation',
    '["translation","multilingual","document","glossary","Markdown","batch"]',
    'en',
    'Smart Translation Engine',
    'Professional translation across 28 languages with domain glossary support and Markdown formatting preservation.',
    E'Professional-grade document translation powered by DeepSeek Chat. Unlike generic translation\n'
    E'tools, this skill:\n\n'
    E'- Preserves your document''s formatting (Markdown, HTML, LaTeX) exactly\n'
    E'- Applies custom domain glossaries for consistent terminology (legal, medical, tech, finance)\n'
    E'- Translates entire directories in batch mode with automatic output naming\n'
    E'- Handles long documents by splitting at paragraph boundaries without breaking context\n\n'
    E'**Supported languages:** English, Simplified Chinese, Traditional Chinese, Japanese, Korean,\n'
    E'French, German, Spanish, Portuguese, Italian, Russian, Arabic, Hindi, Vietnamese, Thai,\n'
    E'Indonesian, Dutch, Polish, Turkish, Swedish, Norwegian, Danish, Finnish, Czech, Romanian,\n'
    E'Hungarian, Ukrainian, and Malay.\n\n'
    E'## Setup\n\n'
    E'1. Install Python 3.9+ and run: `pip install openai`\n'
    E'2. Get your DeepRouter API key at <https://deeprouter.co> → Dashboard → API Keys\n'
    E'3. Set your key: `export DEEPROUTER_API_KEY=sk-...`\n\n'
    E'## Quick Start\n\n'
    E'```bash\n'
    E'# Translate an English Markdown file to Chinese\n'
    E'python scripts/translate.py README.md --from en --to zh\n\n'
    E'# Translate a legal contract with domain glossary\n'
    E'python scripts/translate.py contract.txt --from en --to zh \\\n'
    E'  --glossary reference/glossary_domains.txt --format plain\n\n'
    E'# Batch translate all .md files in docs/ to Japanese\n'
    E'python scripts/translate.py ./docs/ --from en --to ja --ext md --outdir ./docs-ja/\n\n'
    E'# Translate to Traditional Chinese\n'
    E'python scripts/translate.py paper.txt --from en --to zh-tw\n'
    E'```\n\n'
    E'## Included Glossaries\n\n'
    E'| File | Domain | Terms |\n'
    E'|------|--------|-------|\n'
    E'| `reference/glossary_tech.txt` | Software, AI/ML, Networking | 120+ terms |\n'
    E'| `reference/glossary_domains.txt` | Legal, Medical, Finance | 100+ terms |\n\n'
    E'Add your own glossary with the format `source_term = target_term` (one per line).\n\n'
    E'## Format Modes\n\n'
    E'Use `--format` to tell the translator what to preserve:\n\n'
    E'- `markdown` (default) — preserves headers, bold, tables, code blocks\n'
    E'- `html` — translates only text nodes, never attributes\n'
    E'- `latex` — preserves commands and math environments\n'
    E'- `plain` — plain text with paragraph breaks',
    '["Translate an English README to Chinese for a Chinese open-source project","Translate a legal contract from English to Chinese with domain glossary","Batch translate all documentation from English to Japanese","Translate a medical research paper with consistent terminology","Convert a Markdown blog post to French preserving all formatting"]',
    '["# Getting Started\n\nThis guide explains how to **install** and configure the SDK.\n\n## Prerequisites\n\n- Python 3.9+\n- An API key from [our dashboard](https://example.com)"]',
    '["# 快速入门\n\n本指南介绍如何**安装**和配置 SDK。\n\n## 前提条件\n\n- Python 3.9+\n- 从[我们的控制台](https://example.com)获取的 API 密钥"]',
    'free',
    'free',
    0.0000,
    '["deepseek-chat"]',
    90,
    false,
    true,
    false,
    'not_required',
    true,
    true,
    3,
    NULL,
    1,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO NOTHING;

-- ── Skill 4: Prompt Optimizer ────────────────────────────────────────────────

INSERT INTO skills (
    id, slug, status, category, tags, default_locale,
    name, short_description, description,
    input_hints, example_inputs, example_outputs,
    required_plan, monetization_type, price_markup,
    model_whitelist, timeout_seconds, timeout_risk,
    is_kids_safe, is_kids_exclusive, kids_approval_status,
    ai_disclosure_required, featured_flag, featured_rank,
    active_version_id, created_by, created_at, updated_at, published_at
)
VALUES (
    'a1b2c3d4-0004-0004-0004-000000000004',
    'prompt-optimizer',
    'published',
    'productivity',
    '["prompt engineering","AI","LLM","optimization","ChatGPT","Claude","DeepSeek"]',
    'en',
    'Prompt Optimizer',
    'Transform vague AI prompts into precise, high-performance instructions using expert prompt engineering.',
    E'Stop guessing why your AI prompts underperform. This skill analyzes your prompt and rewrites\n'
    E'it using proven prompt engineering principles — powered by DeepRouter.\n\n'
    E'**What it does:**\n\n'
    E'- Removes ambiguity and vague instructions\n'
    E'- Adds role assignment, output format specs, and constraints\n'
    E'- Applies Chain-of-Thought for reasoning tasks\n'
    E'- Suggests Few-Shot examples when needed\n'
    E'- Explains every change so you learn the principles\n\n'
    E'Works with any AI model: ChatGPT, Claude, Gemini, DeepSeek, or any DeepRouter-routed model.\n\n'
    E'## Setup\n\n'
    E'1. Install Python 3.9+ and run: `pip install openai`\n'
    E'2. Get your DeepRouter API key at <https://deeprouter.co> → Dashboard → API Keys\n'
    E'3. Set your key: `export DEEPROUTER_API_KEY=sk-...`\n\n'
    E'## Quick Start\n\n'
    E'```bash\n'
    E'# Optimize a prompt inline\n'
    E'python scripts/optimize.py "Write me a story about AI"\n\n'
    E'# Optimize a prompt from file\n'
    E'python scripts/optimize.py my_prompt.txt\n\n'
    E'# Save the optimized prompt\n'
    E'python scripts/optimize.py my_prompt.txt --out optimized.txt\n\n'
    E'# From stdin\n'
    E'cat my_prompt.txt | python scripts/optimize.py --stdin\n'
    E'```\n\n'
    E'## Output\n\n'
    E'Each optimization returns:\n\n'
    E'- **Optimized Prompt** — ready to copy-paste\n'
    E'- **What Changed** — bullet list of every improvement\n'
    E'- **Why These Changes Work** — the reasoning behind each change\n'
    E'- **Tip** — one advanced technique to add next\n\n'
    E'See `reference/techniques.md` for the full prompt engineering reference guide.',
    '["Optimize a vague ChatGPT prompt for a blog post","Improve a system prompt for a customer service bot","Add output format and constraints to an ambiguous instruction","Rewrite a prompt that keeps giving the wrong answer","Turn a one-liner into a structured, high-quality prompt"]',
    '["Write me something good about machine learning for my blog"]',
    '["## Optimized Prompt\nYou are a technology writer for software developers. Write a 700-word blog post explaining how transformer attention mechanisms work. Use concrete analogies. Structure: intro → explanation → example code → conclusion. Tone: technical but accessible. Do not use marketing language."]',
    'free',
    'free',
    0.0000,
    '["deepseek-chat"]',
    60,
    false,
    true,
    false,
    'not_required',
    true,
    true,
    4,
    NULL,
    1,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO NOTHING;

-- ── Skill 5: YouTube Analyzer ────────────────────────────────────────────────

INSERT INTO skills (
    id, slug, status, category, tags, default_locale,
    name, short_description, description,
    input_hints, example_inputs, example_outputs,
    required_plan, monetization_type, price_markup,
    model_whitelist, timeout_seconds, timeout_risk,
    is_kids_safe, is_kids_exclusive, kids_approval_status,
    ai_disclosure_required, featured_flag, featured_rank,
    active_version_id, created_by, created_at, updated_at, published_at
)
VALUES (
    'a1b2c3d4-0005-0005-0005-000000000005',
    'youtube-analyzer',
    'published',
    'research',
    '["YouTube","video","content strategy","SEO","sentiment","analytics"]',
    'en',
    'YouTube Analyzer',
    'AI-powered YouTube research: analyze videos, comments, channels, and trends — no YouTube API key needed.',
    E'Research YouTube content at scale using only your DeepRouter API key. This skill uses yt-dlp\n'
    E'(the community-maintained YouTube data library) to fetch public data, then sends it to\n'
    E'DeepRouter AI for analysis. No YouTube API key, no OAuth, no Google account required.\n\n'
    E'**Commands:**\n\n'
    E'| Command | What it does |\n'
    E'|---|---|\n'
    E'| `video <URL or ID>` | Summarize + analyze performance of a video |\n'
    E'| `comments <URL or ID>` | Sentiment analysis of top 50 comments |\n'
    E'| `channel <URL or @handle>` | Full channel health + growth strategy report |\n'
    E'| `search <query>` | Trend analysis of top 10 search results |\n'
    E'| `script <URL or ID>` | Generate script outline from a video |\n\n'
    E'## Setup\n\n'
    E'1. Install Python 3.9+ and run: `pip install openai yt-dlp`\n'
    E'2. Get your DeepRouter API key at <https://deeprouter.co> → Dashboard → API Keys\n'
    E'3. Set your key: `export DEEPROUTER_API_KEY=sk-...`\n\n'
    E'## Quick Start\n\n'
    E'```bash\n'
    E'# Analyze a video (full URL or bare ID both work)\n'
    E'python scripts/analyze.py video dQw4w9WgXcQ\n'
    E'python scripts/analyze.py video https://www.youtube.com/watch?v=dQw4w9WgXcQ\n\n'
    E'# Check comment sentiment\n'
    E'python scripts/analyze.py comments dQw4w9WgXcQ\n\n'
    E'# Research a topic\n'
    E'python scripts/analyze.py search "Python tutorial 2025"\n\n'
    E'# Analyze a channel\n'
    E'python scripts/analyze.py channel @mkbhd\n\n'
    E'# Get script outline\n'
    E'python scripts/analyze.py script dQw4w9WgXcQ\n'
    E'```\n\n'
    E'See `reference/api_reference.md` for full command reference and troubleshooting.',
    '["Analyze the performance and audience appeal of a YouTube video","Get comment sentiment and viewer feedback analysis","Research trending content in a niche","Generate a script outline based on a top-performing video","Find content gaps in search results for a topic"]',
    '["video dQw4w9WgXcQ"]',
    '["## Summary\nThis music video by Rick Astley became a cultural phenomenon known as Rickrolling...\n## Key Topics\n- 80s pop music\n- Viral internet culture\n## Audience Appeal\nHigh nostalgia factor drives shares..."]',
    'free',
    'free',
    0.0000,
    '["deepseek-chat"]',
    60,
    false,
    true,
    false,
    'not_required',
    true,
    true,
    5,
    NULL,
    1,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO NOTHING;

-- ── Skill 6: Agent Memory ────────────────────────────────────────────────────

INSERT INTO skills (
    id, slug, status, category, tags, default_locale,
    name, short_description, description,
    input_hints, example_inputs, example_outputs,
    required_plan, monetization_type, price_markup,
    model_whitelist, timeout_seconds, timeout_risk,
    is_kids_safe, is_kids_exclusive, kids_approval_status,
    ai_disclosure_required, featured_flag, featured_rank,
    active_version_id, created_by, created_at, updated_at, published_at
)
VALUES (
    'a1b2c3d4-0006-0006-0006-000000000006',
    'agent-memory',
    'published',
    'productivity',
    '["memory","AI agent","SQLite","knowledge management","lessons learned","productivity"]',
    'en',
    'Agent Memory',
    'Persistent memory for AI agents: store facts, learn from experience, and synthesize knowledge with DeepRouter AI.',
    E'Give your AI agent a long-term memory. Store facts, record lessons, and let DeepRouter AI\n'
    E'synthesize patterns from your accumulated knowledge base.\n\n'
    E'**Core features:**\n\n'
    E'- **`remember`** — store any fact with tags and optional expiry\n'
    E'- **`recall`** — instant full-text keyword search (SQLite FTS5)\n'
    E'- **`smart-recall`** — AI-powered semantic search via DeepRouter\n'
    E'- **`learn`** — record positive/negative lessons with context and insight\n'
    E'- **`synthesize`** — AI summary of all memories: key facts, patterns, suggested actions\n'
    E'- **`suggest`** — AI analysis of failures → systemic fixes and CLAUDE.md rules\n'
    E'- **`forget-stale`** — auto-prune unused facts\n\n'
    E'Data is stored locally in SQLite (`~/.agent-memory/memory.db`) — nothing is sent to any server\n'
    E'except the AI analysis calls which use your DeepRouter API key.\n\n'
    E'## Setup\n\n'
    E'1. Install Python 3.9+ and run: `pip install openai`\n'
    E'2. Get your DeepRouter API key at <https://deeprouter.co> → Dashboard → API Keys\n'
    E'3. Set your key: `export DEEPROUTER_API_KEY=sk-...`\n\n'
    E'## Quick Start\n\n'
    E'```bash\n'
    E'# Store a fact\n'
    E'python scripts/memory.py remember "Team standup is at 9am UTC" --tags team,schedule\n\n'
    E'# Search memories\n'
    E'python scripts/memory.py recall "standup time"\n\n'
    E'# AI-powered semantic search\n'
    E'python scripts/memory.py smart-recall "when is the daily meeting"\n\n'
    E'# Record a lesson learned\n'
    E'python scripts/memory.py learn "Deployed without running tests" \\\n'
    E'  --context "backend release" --outcome negative \\\n'
    E'  --insight "Always run go test ./... before any production deploy"\n\n'
    E'# AI synthesis of everything you know\n'
    E'python scripts/memory.py synthesize\n'
    E'```\n\n'
    E'See `reference/usage.md` for integration with Claude Code hooks and CLAUDE.md.',
    '["Store a project decision and recall it later","Record a lesson from a bug and get AI improvement suggestions","Synthesize all memories to find patterns and priorities","Smart-recall semantically related facts","Clean up unused facts older than 30 days"]',
    '["remember \"Go embed cannot traverse upward with ../\" --tags go,gotcha,embed"]',
    '["Remembered: 3f7a2c1d8e4b"]',
    'free',
    'free',
    0.0000,
    '["deepseek-chat"]',
    60,
    false,
    true,
    false,
    'not_required',
    true,
    true,
    6,
    NULL,
    1,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO NOTHING;

-- ── Skill 7: Self-Improving Agent ────────────────────────────────────────────

INSERT INTO skills (
    id, slug, status, category, tags, default_locale,
    name, short_description, description,
    input_hints, example_inputs, example_outputs,
    required_plan, monetization_type, price_markup,
    model_whitelist, timeout_seconds, timeout_risk,
    is_kids_safe, is_kids_exclusive, kids_approval_status,
    ai_disclosure_required, featured_flag, featured_rank,
    active_version_id, created_by, created_at, updated_at, published_at
)
VALUES (
    'a1b2c3d4-0007-0007-0007-000000000007',
    'self-improving-agent',
    'published',
    'productivity',
    '["self-improvement","retrospective","learning","AI agent","CLAUDE.md","developer"]',
    'en',
    'Self-Improving Agent',
    'Log AI agent learnings and errors, then use DeepRouter to analyze patterns and generate improvement plans.',
    E'Help your AI agent learn from experience. Maintain a structured `.learnings/` directory of\n'
    E'observations, errors, and feature requests, then use DeepRouter to analyze patterns and\n'
    E'generate prioritized improvement plans.\n\n'
    E'**What you get:**\n\n'
    E'- Structured logging format for `LEARNINGS.md`, `ERRORS.md`, `FEATURE_REQUESTS.md`\n'
    E'- AI-powered pattern analysis: identify root causes across entries\n'
    E'- Priority ranking: what needs fixing first\n'
    E'- Quick wins: things fixable in under 30 minutes\n'
    E'- Systemic improvements: structural fixes that prevent whole categories of issues\n'
    E'- Promotion candidates: which learnings belong permanently in `CLAUDE.md`/`AGENTS.md`\n\n'
    E'## Setup\n\n'
    E'1. Install Python 3.9+ and run: `pip install openai`\n'
    E'2. Get your DeepRouter API key at <https://deeprouter.co> → Dashboard → API Keys\n'
    E'3. Set your key: `export DEEPROUTER_API_KEY=sk-...`\n'
    E'4. Initialize your learnings directory:\n'
    E'   ```bash\n'
    E'   mkdir -p .learnings\n'
    E'   touch .learnings/LEARNINGS.md .learnings/ERRORS.md .learnings/FEATURE_REQUESTS.md\n'
    E'   ```\n\n'
    E'## Quick Start\n\n'
    E'```bash\n'
    E'# Analyze all learnings\n'
    E'python scripts/analyze_learnings.py\n\n'
    E'# Focus on errors only\n'
    E'python scripts/analyze_learnings.py --focus errors\n\n'
    E'# Find what to promote to CLAUDE.md\n'
    E'python scripts/analyze_learnings.py --action promote\n'
    E'```\n\n'
    E'See `reference/logging_format.md` for the full entry format specification and Claude Code hook integration.',
    '["Analyze all learnings to find recurring error patterns","Get a prioritized list of the top 5 issues to fix","Find quick wins that can be fixed in under 30 minutes","Identify which learnings should be promoted to CLAUDE.md","Focus analysis on errors or feature requests only"]',
    '["--focus errors"]',
    '["## Pattern Analysis\n1. Missing validation at API boundaries — appears in 4 of 7 error entries\n2. State mutations without locking — 2 concurrent-access bugs this sprint\n\n## Quick Wins\n1. Add input validation middleware (30 min)\n2. Add mutex to session cache (20 min)"]',
    'free',
    'free',
    0.0000,
    '["deepseek-chat"]',
    90,
    false,
    true,
    false,
    'not_required',
    true,
    true,
    7,
    NULL,
    1,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO NOTHING;

-- ── Skill 8: Self-Reflection ─────────────────────────────────────────────────

INSERT INTO skills (
    id, slug, status, category, tags, default_locale,
    name, short_description, description,
    input_hints, example_inputs, example_outputs,
    required_plan, monetization_type, price_markup,
    model_whitelist, timeout_seconds, timeout_risk,
    is_kids_safe, is_kids_exclusive, kids_approval_status,
    ai_disclosure_required, featured_flag, featured_rank,
    active_version_id, created_by, created_at, updated_at, published_at
)
VALUES (
    'a1b2c3d4-0008-0008-0008-000000000008',
    'self-reflection',
    'published',
    'productivity',
    '["reflection","retrospective","journaling","developer","AI agent","learning"]',
    'en',
    'Self-Reflection',
    'AI-powered structured retrospectives for developers and AI agents: input session notes, get deep insights and action items.',
    E'Turn raw session notes into structured retrospectives with DeepRouter AI.\n\n'
    E'Three reflection styles for different use cases:\n\n'
    E'| Style | Best for |\n'
    E'|---|---|\n'
    E'| `--style dev` | Daily coding sessions, bug postmortems, sprint reviews |\n'
    E'| `--style agent` | AI agent performance evaluation |\n'
    E'| `--style learning` | Courses, tutorials, documentation review sessions |\n\n'
    E'**Output includes:**\n\n'
    E'- What went well (specific, not generic)\n'
    E'- What was challenging + root cause analysis\n'
    E'- Key learnings numbered for easy reference\n'
    E'- Concrete action items for tomorrow\n'
    E'- One key insight to remember\n\n'
    E'## Setup\n\n'
    E'1. Install Python 3.9+ and run: `pip install openai`\n'
    E'2. Get your DeepRouter API key at <https://deeprouter.co> → Dashboard → API Keys\n'
    E'3. Set your key: `export DEEPROUTER_API_KEY=sk-...`\n\n'
    E'## Quick Start\n\n'
    E'```bash\n'
    E'# Developer retrospective from notes file\n'
    E'python scripts/reflect.py session_notes.txt\n\n'
    E'# AI agent performance review\n'
    E'python scripts/reflect.py agent_log.txt --style agent\n\n'
    E'# Learning session recap\n'
    E'python scripts/reflect.py study_notes.txt --style learning\n\n'
    E'# Interactive mode (type notes, press Ctrl+D)\n'
    E'python scripts/reflect.py\n\n'
    E'# Save to file\n'
    E'python scripts/reflect.py notes.txt --out reflection_2026-06-23.md\n'
    E'```\n\n'
    E'See `reference/reflection_guide.md` for tips on what to include in your session notes for best results.',
    '["Reflect on today''s coding session and get action items","Evaluate an AI agent''s task performance","Consolidate a learning session with practice exercises","Do a weekly developer retrospective","Create a blameless postmortem from incident notes"]',
    '["Worked on DB migration. Hit foreign key issue for 2h. Fixed with deferred constraints. Tests pass."]',
    '["## What Went Well\n- Correctly identified foreign key constraint as root cause without external help\n- Tests cover the migration path\n\n## Root Causes\n- Schema dependency order not documented\n\n## For Tomorrow\n1. Add migration order comments to schema file\n2. Check for similar constraint issues in user table migration"]',
    'free',
    'free',
    0.0000,
    '["deepseek-chat"]',
    60,
    false,
    true,
    false,
    'not_required',
    true,
    true,
    8,
    NULL,
    1,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO NOTHING;

-- ─────────────────────────────────────────────────────────────────────────────
-- MySQL variant (replace ON CONFLICT with INSERT IGNORE):
-- Replace the eight INSERT INTO ... ON CONFLICT (slug) DO NOTHING
-- with INSERT IGNORE INTO ... (remove the ON CONFLICT clause).
--
-- SQLite variant (replace ON CONFLICT with INSERT OR IGNORE):
-- Replace INSERT INTO with INSERT OR IGNORE INTO (remove ON CONFLICT clause).
-- Replace NOW() with CURRENT_TIMESTAMP.
-- Remove the E'' string escaping; use regular '' single quotes.
-- ─────────────────────────────────────────────────────────────────────────────
