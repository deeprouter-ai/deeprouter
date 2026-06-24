#!/usr/bin/env python3
"""Agent Memory — powered by DeepRouter.

Persistent memory for AI agents: remember facts, learn from experience,
track entities. Includes AI-powered synthesis and smart recall via DeepRouter.

Requirements:
    pip install openai

Usage:
    python memory.py remember "Boss prefers short updates" --tags work,comm
    python memory.py learn "Used X" --context "project" --outcome negative --insight "Y"
    python memory.py recall "how does boss like updates"
    python memory.py synthesize              AI summary of all memories
    python memory.py smart-recall "query"    AI-powered semantic recall
    python memory.py suggest                 AI improvement suggestions
    python memory.py stats
    python memory.py forget-stale --days 30

Environment:
    DEEPROUTER_API_KEY   Your DeepRouter API key (required for AI features)
    DEEPROUTER_MODEL     Model override (default: deepseek-chat)
    MEMORY_DB_PATH       SQLite path (default: ~/.agent-memory/memory.db)
"""

import argparse
import hashlib
import json
import os
import sqlite3
import sys
from dataclasses import asdict, dataclass
from datetime import datetime, timedelta
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    from openai import OpenAI
except ImportError:



    print("ERROR: openai not found. Run: pip install openai", file=sys.stderr)
    sys.exit(1)

def _load_dotenv() -> None:
    """Load .env from the skill root directory so DEEPROUTER_API_KEY is set automatically."""
    import pathlib as _pl, os as _os
    env_path = _pl.Path(__file__).parent.parent / ".env"
    if not env_path.exists():
        return
    for raw in env_path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, val = line.partition("=")
        key = key.strip()
        val = val.strip()
        if len(val) >= 2 and val[0] == val[-1] and val[0] in ('"', "'"):
            val = val[1:-1]
        if key and key not in _os.environ:
            _os.environ[key] = val

_load_dotenv()











# ── Data models ───────────────────────────────────────────────────────────────

@dataclass
class Fact:
    id: str
    content: str
    tags: List[str]
    source: str
    confidence: float
    created_at: str
    last_accessed: str
    access_count: int
    expires_at: Optional[str] = None
    superseded_by: Optional[str] = None

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class Lesson:
    id: str
    action: str
    context: str
    outcome: str
    insight: str
    created_at: str
    applied_count: int = 0


# ── Storage ───────────────────────────────────────────────────────────────────

class AgentMemory:
    def __init__(self, db_path: str = None):
        if db_path is None:
            db_dir = Path.home() / ".agent-memory"
            db_dir.mkdir(exist_ok=True)
            db_path = str(db_dir / "memory.db")
        self.db_path = db_path
        self._init_db()

    def _init_db(self):
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()
        c.execute("""CREATE TABLE IF NOT EXISTS facts (
            id TEXT PRIMARY KEY, content TEXT NOT NULL, tags TEXT,
            source TEXT DEFAULT 'conversation', confidence REAL DEFAULT 1.0,
            created_at TEXT NOT NULL, last_accessed TEXT NOT NULL,
            access_count INTEGER DEFAULT 1, expires_at TEXT, superseded_by TEXT)""")
        c.execute("""CREATE TABLE IF NOT EXISTS lessons (
            id TEXT PRIMARY KEY, action TEXT NOT NULL, context TEXT NOT NULL,
            outcome TEXT NOT NULL, insight TEXT NOT NULL,
            created_at TEXT NOT NULL, applied_count INTEGER DEFAULT 0)""")
        c.execute("""CREATE VIRTUAL TABLE IF NOT EXISTS facts_fts
            USING fts5(content, tags, tokenize='porter')""")
        conn.commit(); conn.close()

    def _id(self, s: str) -> str:
        return hashlib.sha256(f"{s}{datetime.utcnow().isoformat()}".encode()).hexdigest()[:12]

    def _now(self) -> str:
        return datetime.utcnow().isoformat()

    def remember(self, content: str, tags: List[str] = None,
                 source: str = "conversation", confidence: float = 1.0,
                 expires_in_days: int = None) -> str:
        fid = self._id(content)
        now = self._now()
        exp = (datetime.utcnow() + timedelta(days=expires_in_days)).isoformat() if expires_in_days else None
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()
        c.execute("INSERT INTO facts VALUES (?,?,?,?,?,?,?,1,?)",
                  (fid, content, json.dumps(tags or []), source, confidence, now, now, exp))
        c.execute("INSERT INTO facts_fts SELECT rowid,content,tags FROM facts WHERE id=?", (fid,))
        conn.commit(); conn.close()
        return fid

    def recall(self, query: str, limit: int = 10) -> List[Fact]:
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()
        c.execute("""SELECT f.* FROM facts f JOIN facts_fts fts ON f.rowid=fts.rowid
            WHERE facts_fts MATCH ? AND f.superseded_by IS NULL
            AND (f.expires_at IS NULL OR f.expires_at > ?)
            ORDER BY fts.rank LIMIT ?""", (query, self._now(), limit))
        rows = c.fetchall()
        conn.close()
        return [Fact(r[0], r[1], json.loads(r[2] or "[]"), r[3], r[4],
                     r[5], r[6], r[7], r[8], r[9]) for r in rows]

    def learn(self, action: str, context: str, outcome: str, insight: str) -> str:
        lid = self._id(f"{action}{context}")
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()
        c.execute("INSERT INTO lessons VALUES (?,?,?,?,?,?,0)",
                  (lid, action, context, outcome, insight, self._now()))
        conn.commit(); conn.close()
        return lid

    def all_facts(self, limit: int = 200) -> List[Fact]:
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()
        c.execute("SELECT * FROM facts WHERE superseded_by IS NULL ORDER BY created_at DESC LIMIT ?", (limit,))
        rows = c.fetchall(); conn.close()
        return [Fact(r[0], r[1], json.loads(r[2] or "[]"), r[3], r[4],
                     r[5], r[6], r[7], r[8], r[9]) for r in rows]

    def all_lessons(self, limit: int = 200) -> List[Lesson]:
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()
        c.execute("SELECT * FROM lessons ORDER BY created_at DESC LIMIT ?", (limit,))
        rows = c.fetchall(); conn.close()
        return [Lesson(r[0], r[1], r[2], r[3], r[4], r[5], r[6]) for r in rows]

    def forget_stale(self, days: int = 30) -> int:
        cutoff = (datetime.utcnow() - timedelta(days=days)).isoformat()
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()
        c.execute("DELETE FROM facts WHERE last_accessed < ? AND access_count <= 1", (cutoff,))
        deleted = c.rowcount; conn.commit(); conn.close()
        return deleted

    def stats(self) -> Dict[str, int]:
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()
        c.execute("SELECT COUNT(*) FROM facts WHERE superseded_by IS NULL"); f = c.fetchone()[0]
        c.execute("SELECT COUNT(*) FROM lessons"); l = c.fetchone()[0]
        conn.close()
        return {"active_facts": f, "lessons": l}


# ── AI features ───────────────────────────────────────────────────────────────

def get_client() -> OpenAI:
    key = os.environ.get("DEEPROUTER_API_KEY")
    if not key:
        print("ERROR: DEEPROUTER_API_KEY not set.", file=sys.stderr)
        print("  Get your key at https://deeprouter.co → Dashboard → API Keys", file=sys.stderr)
        sys.exit(1)
    return OpenAI(api_key=key, base_url=os.environ.get("DEEPROUTER_BASE_URL", "https://api.deeprouter.co/v1"))


def ai_call(system: str, user: str, model: str) -> str:
    client = get_client()
    resp = client.chat.completions.create(
        model=model,
        messages=[{"role": "system", "content": system}, {"role": "user", "content": user}],
        temperature=0.3, max_tokens=2048,
    )
    return resp.choices[0].message.content or ""


def cmd_synthesize(mem: AgentMemory, model: str) -> None:
    facts = mem.all_facts(100)
    lessons = mem.all_lessons(50)
    if not facts and not lessons:
        print("Memory is empty."); return

    facts_text = "\n".join(f"- [{','.join(f.tags)}] {f.content}" for f in facts)
    lessons_text = "\n".join(
        f"- [{l.outcome}] {l.action}: {l.insight}" for l in lessons)

    system = """You are a knowledge synthesiser. Given an agent's memory entries, produce:
## Key Facts (top 10 most important, distilled)
## Patterns & Insights (recurring themes across facts and lessons)
## Lessons Learned (top 5 negative lessons to avoid + top 5 positive patterns to repeat)
## Suggested Next Actions (3 concrete things the agent should do based on this knowledge)"""

    user = f"Facts:\n{facts_text}\n\nLessons:\n{lessons_text}"
    print(f"Synthesizing {len(facts)} facts + {len(lessons)} lessons via DeepRouter...", file=sys.stderr)
    print(ai_call(system, user, model))


def cmd_smart_recall(mem: AgentMemory, query: str, model: str) -> None:
    candidates = mem.recall(query, limit=20)
    if not candidates:
        print("No matching memories found."); return

    facts_text = "\n".join(f"[{f.id}] {f.content}" for f in candidates)
    system = """You are a memory assistant. Given a query and candidate memory entries,
return the 3-5 most relevant entries and explain WHY each is relevant to the query.
Format: ranked list with relevance explanation."""

    print(f"Smart-recalling via DeepRouter...", file=sys.stderr)
    print(ai_call(system, f"Query: {query}\n\nCandidates:\n{facts_text}", model))


def cmd_suggest(mem: AgentMemory, model: str) -> None:
    lessons = mem.all_lessons(50)
    neg = [l for l in lessons if l.outcome == "negative"]
    if not neg:
        print("No negative lessons recorded yet."); return

    text = "\n".join(f"- {l.action} → {l.insight}" for l in neg[:20])
    system = """You are a performance coach. Review these negative lessons and suggest:
## Systemic Fixes (root causes + structural solutions)
## Rules to Add to CLAUDE.md (concrete prevention rules)
## What to Automate or Checklist (recurring mistakes that need process, not memory)"""

    print(f"Generating suggestions via DeepRouter...", file=sys.stderr)
    print(ai_call(system, f"Negative lessons:\n{text}", model))


# ── CLI ───────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Persistent AI agent memory with DeepRouter-powered synthesis.",
        formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)

    sub = parser.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("remember", help="Store a fact")
    p.add_argument("content"); p.add_argument("--tags", default=""); p.add_argument("--expires", type=int)

    p = sub.add_parser("recall", help="Search memories (keyword)")
    p.add_argument("query"); p.add_argument("--limit", type=int, default=10)

    p = sub.add_parser("smart-recall", help="AI-powered semantic recall")
    p.add_argument("query"); p.add_argument("--model", default=None)

    p = sub.add_parser("learn", help="Record a lesson")
    p.add_argument("action"); p.add_argument("--context", required=True)
    p.add_argument("--outcome", choices=["positive","negative","neutral"], required=True)
    p.add_argument("--insight", required=True)

    sub.add_parser("synthesize", help="AI synthesis of all memories").add_argument("--model", default=None)
    sub.add_parser("suggest", help="AI improvement suggestions").add_argument("--model", default=None)
    sub.add_parser("stats", help="Memory statistics")

    p = sub.add_parser("forget-stale", help="Delete old unused facts")
    p.add_argument("--days", type=int, default=30)

    args = parser.parse_args()
    model = getattr(args, "model", None) or os.environ.get("DEEPROUTER_MODEL", "deepseek-chat")
    mem = AgentMemory(os.environ.get("MEMORY_DB_PATH"))

    if args.cmd == "remember":
        tags = [t.strip() for t in args.tags.split(",") if t.strip()]
        fid = mem.remember(args.content, tags=tags, expires_in_days=args.expires)
        print(f"Remembered: {fid}")
    elif args.cmd == "recall":
        facts = mem.recall(args.query, args.limit)
        if not facts: print("No matches.")
        for f in facts:
            print(f"[{f.id}] [{','.join(f.tags)}] {f.content}")
    elif args.cmd == "smart-recall":
        cmd_smart_recall(mem, args.query, model)
    elif args.cmd == "learn":
        lid = mem.learn(args.action, args.context, args.outcome, args.insight)
        print(f"Lesson recorded: {lid}")
    elif args.cmd == "synthesize":
        cmd_synthesize(mem, model)
    elif args.cmd == "suggest":
        cmd_suggest(mem, model)
    elif args.cmd == "stats":
        s = mem.stats()
        print(f"Active facts: {s['active_facts']}  |  Lessons: {s['lessons']}")
        print(f"DB: {mem.db_path}")
    elif args.cmd == "forget-stale":
        n = mem.forget_stale(args.days)
        print(f"Deleted {n} stale facts (not accessed in {args.days} days).")


if __name__ == "__main__":
    main()
