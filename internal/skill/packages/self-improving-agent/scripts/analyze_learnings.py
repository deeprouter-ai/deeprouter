#!/usr/bin/env python3
"""Self-Improving Agent Analyzer — powered by DeepRouter.

Reads your .learnings/ directory and uses DeepRouter AI to identify patterns,
generate a prioritized improvement plan, and suggest what to promote to
CLAUDE.md / AGENTS.md.

Requirements:
    pip install openai

Usage:
    # Analyze all learnings in current directory
    python analyze_learnings.py

    # Analyze a specific learnings directory
    python analyze_learnings.py --dir /path/to/project/.learnings

    # Focus on errors only
    python analyze_learnings.py --focus errors

    # Focus on feature requests
    python analyze_learnings.py --focus features

    # Generate CLAUDE.md promotion candidates
    python analyze_learnings.py --action promote

Environment:
    DEEPROUTER_API_KEY   Your DeepRouter API key (required)
    DEEPROUTER_MODEL     Model override (default: deepseek-chat)
"""

import argparse
import os
import sys
from pathlib import Path

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






LEARNINGS_FILE = "LEARNINGS.md"
ERRORS_FILE = "ERRORS.md"
FEATURES_FILE = "FEATURE_REQUESTS.md"


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
        temperature=0.3, max_tokens=3000,
    )
    return resp.choices[0].message.content or ""


def read_file(path: Path) -> str:
    if path.exists():
        return path.read_text(encoding="utf-8")
    return ""


def truncate(text: str, max_chars: int = 8000) -> str:
    return text[:max_chars] + "\n[...truncated...]" if len(text) > max_chars else text


def cmd_analyze(learnings_dir: Path, focus: str, model: str) -> None:
    learnings = read_file(learnings_dir / LEARNINGS_FILE)
    errors = read_file(learnings_dir / ERRORS_FILE)
    features = read_file(learnings_dir / FEATURES_FILE)

    if not any([learnings, errors, features]):
        print(f"ERROR: No .learnings/ files found in {learnings_dir}", file=sys.stderr)
        print("  Run: mkdir .learnings && touch .learnings/LEARNINGS.md .learnings/ERRORS.md .learnings/FEATURE_REQUESTS.md")
        sys.exit(1)

    if focus == "errors":
        context = f"ERRORS:\n{truncate(errors)}"
    elif focus == "features":
        context = f"FEATURE REQUESTS:\n{truncate(features)}"
    else:
        context = f"LEARNINGS:\n{truncate(learnings)}\n\nERRORS:\n{truncate(errors, 3000)}\n\nFEATURE REQUESTS:\n{truncate(features, 2000)}"

    system = """You are a software engineering coach performing a retrospective analysis.
Given an agent's learning logs, provide:

## Pattern Analysis
Identify the top 3-5 recurring themes or root causes (not individual incidents)

## Priority Issues (top 5)
Most critical pending items that need immediate attention

## Quick Wins
3 things that could be fixed in under 30 minutes

## Systemic Improvements
2-3 structural changes that would prevent whole categories of issues

## What's Going Well
Positive patterns worth reinforcing

## Action Plan for Next 48 Hours
Numbered list of 5 concrete tasks, most impactful first"""

    total = sum(len(x) for x in [learnings, errors, features] if x)
    print(f"Analyzing {total} chars of learnings via DeepRouter ({model})...", file=sys.stderr)
    print(ai_call(system, context, model))


def cmd_promote(learnings_dir: Path, model: str) -> None:
    learnings = read_file(learnings_dir / LEARNINGS_FILE)
    if not learnings:
        print("No LEARNINGS.md found."); return

    system = """You are a project documentation curator.
Review these learning entries and identify which deserve promotion to permanent memory.

Output format:

## CLAUDE.md Additions
(project facts, gotchas, conventions — concise rules)

```markdown
## [Section Title]
- rule 1
- rule 2
```

## AGENTS.md Additions
(workflow rules, automation patterns, tool usage)

```markdown
## [Workflow Name]
1. step
2. step
```

## Entries to Mark 'promoted'
List the entry IDs (LRN-YYYYMMDD-XXX) that should be marked as promoted after adding above."""

    print(f"Finding promotion candidates via DeepRouter ({model})...", file=sys.stderr)
    print(ai_call(system, truncate(learnings, 6000), model))


def main() -> None:
    parser = argparse.ArgumentParser(
        description="AI-powered learning analysis using DeepRouter.",
        formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    parser.add_argument("--dir", default=".learnings",
                        help="Path to .learnings directory (default: ./.learnings)")
    parser.add_argument("--focus", choices=["all", "errors", "features"], default="all",
                        help="Which logs to analyze (default: all)")
    parser.add_argument("--action", choices=["analyze", "promote"], default="analyze",
                        help="analyze (default) or promote (find CLAUDE.md candidates)")
    parser.add_argument("--model", default=None)
    args = parser.parse_args()

    model = args.model or os.environ.get("DEEPROUTER_MODEL", "deepseek-chat")
    learnings_dir = Path(args.dir)

    if args.action == "promote":
        cmd_promote(learnings_dir, model)
    else:
        cmd_analyze(learnings_dir, args.focus, model)


if __name__ == "__main__":
    main()
