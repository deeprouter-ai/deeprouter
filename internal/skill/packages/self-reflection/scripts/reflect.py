#!/usr/bin/env python3
"""Self-Reflection — powered by DeepRouter.

AI-powered structured reflection for developers and AI agents. Input a
session summary or project description, get back a deep retrospective with
actionable insights.

Requirements:
    pip install openai

Usage:
    # Interactive mode (paste your session notes, press Ctrl+D)
    python reflect.py

    # From a file
    python reflect.py notes.txt

    # From stdin
    cat notes.txt | python reflect.py --stdin

    # Save reflection to file
    python reflect.py notes.txt --out reflection.md

    # Choose reflection style
    python reflect.py notes.txt --style dev       (developer retrospective)
    python reflect.py notes.txt --style agent     (AI agent performance review)
    python reflect.py notes.txt --style learning  (learning session recap)

Environment:
    DEEPROUTER_API_KEY   Your DeepRouter API key (required)
    DEEPROUTER_MODEL     Model override (default: deepseek-chat)
"""

import argparse
import os
import sys
from datetime import datetime

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











SYSTEMS = {
    "dev": """You are an experienced engineering coach conducting a blameless retrospective.
Given a developer's session notes, produce a structured reflection:

## What Went Well ✓
(3-5 specific positives — be concrete, not generic)

## What Was Challenging ✗
(honest assessment of friction points, mistakes, blockers)

## Root Causes
(WHY did the challenges happen — not symptoms, but underlying causes)

## Key Learnings
(3 things worth remembering from this session — numbered)

## For Tomorrow
(3 concrete actions to do differently or build on — specific and actionable)

## One Insight
(the single most important thing from today — one sentence)""",

    "agent": """You are an AI performance analyst reviewing an agent's work session.
Given a session log or summary, evaluate agent performance:

## Task Completion
(what was accomplished vs. what was intended)

## Decision Quality
(good decisions made + questionable decisions + missed opportunities)

## Tool & Resource Usage
(efficient use of tools, unnecessary steps, better alternatives)

## Error Patterns
(mistakes made, whether they were caught, how they were handled)

## Compliance Check
(did the agent follow guidelines, ask when uncertain, avoid overreach)

## Performance Score (1-10)
(brief justification)

## Recommendations
(3 specific improvements for the next session)""",

    "learning": """You are a learning coach conducting a knowledge consolidation session.
Given learning session notes, create a structured recap:

## Core Concepts Covered
(bullet list of the main ideas encountered)

## What I Understood Well
(concepts that clicked — with a brief explanation in your own words)

## What's Still Unclear
(gaps, confusion points, things that need more practice)

## Connections Made
(how this connects to things you already know)

## Questions to Investigate
(specific questions this session raised)

## Practice Tasks
(3 concrete exercises to reinforce today's learning)

## 24h Summary
(explain the core idea in 2 sentences, as if teaching someone else)""",
}


def get_client() -> OpenAI:
    key = os.environ.get("DEEPROUTER_API_KEY")
    if not key:
        print("ERROR: DEEPROUTER_API_KEY not set.", file=sys.stderr)
        print("  Get your key at https://deeprouter.co → Dashboard → API Keys", file=sys.stderr)
        sys.exit(1)
    return OpenAI(api_key=key, base_url=os.environ.get("DEEPROUTER_BASE_URL", "https://api.deeprouter.co/v1"))


def reflect(text: str, style: str, model: str) -> str:
    client = get_client()
    system = SYSTEMS.get(style, SYSTEMS["dev"])
    resp = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": system},
            {"role": "user", "content": f"Session notes:\n\n{text}"},
        ],
        temperature=0.4,
        max_tokens=2000,
    )
    return resp.choices[0].message.content or ""


def main() -> None:
    parser = argparse.ArgumentParser(
        description="AI-powered structured reflection using DeepRouter.",
        formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    parser.add_argument("input", nargs="?", help="Session notes file (.txt, .md)")
    parser.add_argument("--stdin", action="store_true", help="Read from stdin")
    parser.add_argument("--style", choices=["dev", "agent", "learning"], default="dev",
                        help="Reflection style (default: dev)")
    parser.add_argument("--model", default=None)
    parser.add_argument("--out", help="Save reflection to file")
    args = parser.parse_args()

    model = args.model or os.environ.get("DEEPROUTER_MODEL", "deepseek-chat")

    if args.stdin or (not args.input and not sys.stdin.isatty()):
        text = sys.stdin.read().strip()
    elif args.input:
        try:
            text = open(args.input, encoding="utf-8").read().strip()
        except FileNotFoundError:
            print(f"ERROR: File not found: {args.input}", file=sys.stderr)
            sys.exit(1)
    else:
        style_labels = {"dev": "developer", "agent": "agent", "learning": "learning"}
        print(f"Paste your {style_labels[args.style]} session notes below.")
        print("Press Ctrl+D (Unix) or Ctrl+Z (Windows) when done.\n")
        text = sys.stdin.read().strip()

    if not text:
        print("ERROR: No input provided.", file=sys.stderr)
        sys.exit(1)

    date_str = datetime.now().strftime("%Y-%m-%d")
    print(f"Generating {args.style} reflection via DeepRouter ({model})...", file=sys.stderr)
    result = f"# Reflection — {date_str} ({args.style})\n\n{reflect(text, args.style, model)}"

    if args.out:
        open(args.out, "w", encoding="utf-8").write(result)
        print(f"Written to {args.out}", file=sys.stderr)
    else:
        print(result)


if __name__ == "__main__":
    main()
