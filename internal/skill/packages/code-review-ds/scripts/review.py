#!/usr/bin/env python3
"""Code Review Expert — powered by DeepSeek Coder.

Performs multi-dimensional code review: correctness, security, performance,
maintainability, and style. Supports 30+ programming languages.

Requirements:
    pip install openai

Usage:
    # Review a single file
    python review.py main.go

    # Review a directory (recursive)
    python review.py ./src/

    # Review only specific extensions
    python review.py ./src/ --ext py,ts,go

    # Focus on security issues only
    python review.py main.py --focus security

    # Save report to markdown file
    python review.py ./src/ --out review_report.md

    # Strict mode (treat warnings as errors, exit 1 if issues found)
    python review.py ./src/ --strict

Environment:
    DEEPROUTER_API_KEY   Your DeepRouter API key (required — get it from the DeepRouter dashboard)
    DEEPROUTER_MODEL     Model name (default: deepseek-coder)
"""

import argparse
import os
import sys
from dataclasses import dataclass, field
from pathlib import Path

try:
    from openai import OpenAI
except ImportError:



    print("ERROR: openai package not found. Run: pip install openai", file=sys.stderr)
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










# ── Config ────────────────────────────────────────────────────────────────────

DEFAULT_MODEL = "deepseek-coder"

CODE_EXTENSIONS = {
    ".py", ".js", ".ts", ".jsx", ".tsx", ".go", ".java", ".kt", ".swift",
    ".c", ".cpp", ".h", ".hpp", ".cs", ".rb", ".rs", ".php", ".scala",
    ".sh", ".bash", ".zsh", ".sql", ".vue", ".svelte", ".dart", ".r",
    ".m", ".mm", ".lua", ".pl", ".ex", ".exs", ".hs", ".ml", ".fs",
}

MAX_FILE_CHARS = 12_000  # ~3000 tokens — safe for deepseek-coder context

# ── Prompts ───────────────────────────────────────────────────────────────────

SYSTEM_FULL = """You are a senior software engineer performing a comprehensive
code review. Analyse the submitted code across all dimensions:

1. CORRECTNESS — logic errors, off-by-one, null/undefined handling, wrong
   conditions, missing error checks, resource leaks
2. SECURITY — injection vulnerabilities (SQL, command, XSS), hard-coded
   secrets, insecure crypto, path traversal, unsafe deserialization,
   missing auth/authz checks, SSRF, XXE
3. PERFORMANCE — O(n²) or worse in hot paths, redundant I/O, N+1 queries,
   unnecessary allocations, missing indexes, unbounded loops
4. MAINTAINABILITY — duplicated logic, overly complex functions (>30 lines),
   missing error types, magic numbers, poor naming, missing docstrings for
   public APIs
5. STYLE — inconsistent formatting, unused imports, dead code

Output format (Markdown):

## Summary
One paragraph overall assessment. Include severity distribution.

## Issues

For each issue found:

### [SEVERITY] Short title
- **File/Line**: `filename.ext:42` (use "approx." if uncertain)
- **Category**: Correctness | Security | Performance | Maintainability | Style
- **Description**: What is wrong and why it matters
- **Suggested Fix**:
```language
// fixed code snippet
```

Severity levels: CRITICAL | HIGH | MEDIUM | LOW | INFO

## Positive Observations
List 2–5 things done well.

If no issues are found, say so clearly. Do not invent issues."""

SYSTEM_SECURITY = """You are an application security engineer performing a
focused security code review. Your ONLY task is to find security vulnerabilities.

Categories to check (OWASP Top 10 + extras):
- Injection: SQL, NoSQL, command, LDAP, XPath, template injection
- Broken authentication: weak session, missing token validation
- Sensitive data exposure: hard-coded secrets, unencrypted PII in logs
- XXE, SSRF, path traversal, open redirect
- Missing access control (IDOR, privilege escalation)
- Security misconfiguration: debug mode on, permissive CORS
- XSS: reflected, stored, DOM-based
- Insecure direct object references
- Cryptography: weak algorithms (MD5/SHA1 for passwords), no salt, ECB mode
- Race conditions in auth or payment paths
- Supply chain: use of abandoned or typosquatted packages

Output format (Markdown):

## Security Review Summary
Overall risk level: CRITICAL | HIGH | MEDIUM | LOW | CLEAN

## Vulnerabilities Found
For each finding: CVE/CWE reference if applicable, CVSS-like severity,
reproduction steps, and remediation.

## Clean Paths
Explicitly confirm paths/functions that were reviewed and appear clean.

If no vulnerabilities are found, state: "No security vulnerabilities identified."
Do NOT report style or performance issues."""

# ── Data ──────────────────────────────────────────────────────────────────────

@dataclass
class FileReview:
    path: str
    content: str
    review: str = ""
    error: str = ""


@dataclass
class ReviewSession:
    files: list[FileReview] = field(default_factory=list)
    model: str = DEFAULT_MODEL
    focus: str = "full"


# ── Core ──────────────────────────────────────────────────────────────────────

def get_client() -> OpenAI:
    api_key = os.environ.get("DEEPROUTER_API_KEY")
    if not api_key:
        print("ERROR: DEEPROUTER_API_KEY environment variable not set.", file=sys.stderr)
        print("  Get your key at https://deeprouter.co → Dashboard → API Keys", file=sys.stderr)
        print("  export DEEPROUTER_API_KEY=sk-...", file=sys.stderr)
        sys.exit(1)
    return OpenAI(
        api_key=api_key,
        base_url=os.environ.get("DEEPROUTER_BASE_URL", "https://api.deeprouter.co/v1"),
    )


def review_file(client: OpenAI, fr: FileReview, model: str, focus: str) -> None:
    system = SYSTEM_SECURITY if focus == "security" else SYSTEM_FULL
    user_msg = f"File: `{fr.path}`\n\n```\n{fr.content}\n```"
    try:
        resp = client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system},
                {"role": "user", "content": user_msg},
            ],
            temperature=0.1,
            max_tokens=4096,
        )
        fr.review = resp.choices[0].message.content or "(empty response)"
    except Exception as exc:
        fr.error = str(exc)


def collect_files(
    path: Path, extensions: set[str], max_chars: int
) -> list[FileReview]:
    results = []
    targets = [path] if path.is_file() else sorted(path.rglob("*"))
    for target in targets:
        if not target.is_file():
            continue
        if target.suffix.lower() not in extensions:
            continue
        if any(part.startswith(".") or part in ("node_modules", "__pycache__", "vendor", "dist", "build")
               for part in target.parts):
            continue
        try:
            content = target.read_text(encoding="utf-8", errors="replace")
        except OSError as e:
            results.append(FileReview(path=str(target), content="", error=str(e)))
            continue
        if len(content) > max_chars:
            content = content[:max_chars] + f"\n\n[... truncated at {max_chars} chars ...]"
        results.append(FileReview(path=str(target), content=content))
    return results


def render_report(session: ReviewSession, input_path: str) -> str:
    lines = [f"# Code Review Report", f"", f"**Input:** `{input_path}`  ",
             f"**Model:** `{session.model}`  ",
             f"**Focus:** {session.focus}  ",
             f"**Files reviewed:** {len(session.files)}",
             ""]
    for fr in session.files:
        lines.append(f"---")
        lines.append(f"## `{fr.path}`")
        lines.append("")
        if fr.error:
            lines.append(f"> ERROR: {fr.error}")
        else:
            lines.append(fr.review)
        lines.append("")
    return "\n".join(lines)


# ── CLI ───────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(
        description="AI code review using DeepSeek Coder.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument("input", help="File or directory to review")
    parser.add_argument(
        "--ext",
        help=f"Comma-separated extensions to include (default: all code files)",
    )
    parser.add_argument(
        "--focus",
        choices=["full", "security"],
        default="full",
        help="Review focus: full (all dimensions) or security only (default: full)",
    )
    parser.add_argument("--model", help=f"Model override (default: {DEFAULT_MODEL})")
    parser.add_argument("--out", help="Write report to file (default: stdout)")
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Exit 1 if any CRITICAL or HIGH issues are found",
    )
    args = parser.parse_args()

    path = Path(args.input)
    if not path.exists():
        print(f"ERROR: Path not found: {args.input}", file=sys.stderr)
        sys.exit(1)

    extensions = CODE_EXTENSIONS
    if args.ext:
        extensions = {"." + e.lstrip(".").lower() for e in args.ext.split(",")}

    files = collect_files(path, extensions, MAX_FILE_CHARS)
    if not files:
        print(f"No matching files found under {args.input}", file=sys.stderr)
        sys.exit(0)

    model = args.model or os.environ.get("DEEPROUTER_MODEL", DEFAULT_MODEL)
    session = ReviewSession(files=files, model=model, focus=args.focus)
    client = get_client()

    for i, fr in enumerate(session.files, 1):
        print(f"[{i}/{len(session.files)}] Reviewing {fr.path}...", file=sys.stderr)
        if not fr.error:
            review_file(client, fr, model=model, focus=args.focus)

    report = render_report(session, args.input)

    if args.out:
        Path(args.out).write_text(report, encoding="utf-8")
        print(f"Report written to {args.out}", file=sys.stderr)
    else:
        print(report)

    if args.strict:
        combined = "\n".join(fr.review for fr in session.files)
        if "### [CRITICAL]" in combined or "### [HIGH]" in combined:
            print("\nSTRICT MODE: CRITICAL or HIGH issues found. Exiting 1.", file=sys.stderr)
            sys.exit(1)


if __name__ == "__main__":
    main()
