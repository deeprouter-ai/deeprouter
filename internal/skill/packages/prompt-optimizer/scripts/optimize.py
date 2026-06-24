#!/usr/bin/env python3
"""Prompt Optimizer — powered by DeepRouter.

Analyzes and rewrites AI prompts to be clearer, more specific, and more
effective. Based on Anthropic prompt engineering best practices.

Requirements:
    pip install openai

Usage:
    python optimize.py "Write me a story"
    python optimize.py prompt.txt
    cat prompt.txt | python optimize.py --stdin
    python optimize.py "your prompt" --model deepseek-chat
    python optimize.py prompt.txt --out optimized.txt

Environment:
    DEEPROUTER_API_KEY   Your DeepRouter API key (required)
    DEEPROUTER_MODEL     Model override (default: deepseek-chat)
"""

import argparse
import os
import sys

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






SYSTEM = """You are a world-class prompt engineering expert specialising in large language models.

Your task: analyse the submitted prompt and rewrite it to be maximally effective.

Apply these principles:
1. CLARITY — remove ambiguity; every instruction must be unambiguous
2. SPECIFICITY — replace vague terms with concrete, measurable instructions
3. STRUCTURE — use logical sections, numbered steps, or XML tags where helpful
4. CONTEXT — add only context that changes the output; strip irrelevant preamble
5. ROLE — assign a clear persona/expertise when it improves the response
6. OUTPUT FORMAT — specify format, length, tone, and audience explicitly
7. ANTI-PATTERNS — eliminate: over-specification, contradictions, jailbreak surfaces, hallucination bait

Techniques to apply as appropriate:
- Chain-of-Thought: add "Think step by step" for reasoning tasks
- Few-shot: suggest 1-2 examples when the task needs a specific format
- XML tags: use <task>, <context>, <format> for complex system prompts
- Prefilling: suggest response starters when format matters
- Constraints: add "Do not..." for known failure modes

Output format (Markdown):
## Optimized Prompt
(the rewritten prompt — ready to copy-paste)

## What Changed
(bullet list: what was wrong in the original and what was fixed)

## Why These Changes Work
(2-3 sentences connecting changes to improved model behaviour)

## Tip
(one advanced technique the user could add next to push quality further)"""


def get_client() -> OpenAI:
    api_key = os.environ.get("DEEPROUTER_API_KEY")
    if not api_key:
        print("ERROR: DEEPROUTER_API_KEY not set.", file=sys.stderr)
        print("  Get your key at https://deeprouter.co → Dashboard → API Keys", file=sys.stderr)
        print("  export DEEPROUTER_API_KEY=sk-...", file=sys.stderr)
        sys.exit(1)




    base_url = os.environ.get("DEEPROUTER_BASE_URL", "https://api.deeprouter.co/v1")
    return OpenAI(api_key=api_key, base_url=base_url)


def optimize(prompt_text: str, model: str) -> str:
    client = get_client()
    resp = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": SYSTEM},
            {"role": "user", "content": f"Optimize this prompt:\n\n{prompt_text}"},
        ],
        temperature=0.3,
        max_tokens=2048,
    )
    return resp.choices[0].message.content or ""


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Optimize AI prompts using DeepRouter.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument("input", nargs="?", help="Prompt text or path to .txt file")
    parser.add_argument("--stdin", action="store_true", help="Read prompt from stdin")
    parser.add_argument("--model", default=None, help="Model override (default: deepseek-chat)")
    parser.add_argument("--out", help="Save output to file")
    args = parser.parse_args()

    model = args.model or os.environ.get("DEEPROUTER_MODEL", "deepseek-chat")

    if args.stdin or (not args.input and not sys.stdin.isatty()):
        text = sys.stdin.read().strip()
    elif args.input and os.path.isfile(args.input):
        text = open(args.input, encoding="utf-8").read().strip()
    elif args.input:
        text = args.input.strip()
    else:
        parser.print_help()
        sys.exit(0)

    if not text:
        print("ERROR: Empty prompt.", file=sys.stderr)
        sys.exit(1)

    print(f"Optimizing prompt via DeepRouter ({model})...", file=sys.stderr)
    result = optimize(text, model)

    if args.out:
        open(args.out, "w", encoding="utf-8").write(result)
        print(f"Written to {args.out}", file=sys.stderr)
    else:
        print(result)


if __name__ == "__main__":
    main()
