#!/usr/bin/env python3
"""Smart Translation Engine — powered by DeepSeek Chat.

Professional-grade translation with domain glossary support. Preserves
formatting (Markdown, HTML, LaTeX), handles batch files, and applies
custom terminology consistently.

Requirements:
    pip install openai

Usage:
    # Translate a file to Chinese (default target)
    python translate.py document.md

    # Specify source and target languages
    python translate.py paper.txt --from en --to ja

    # Use a domain glossary
    python translate.py contract.txt --from en --to zh --glossary reference/glossary_legal.txt

    # Translate all .md files in a directory
    python translate.py ./docs/ --from en --to zh --ext md

    # Batch translate with custom output directory
    python translate.py ./docs/ --from en --to zh --outdir ./docs-zh/

    # Preserve formatting mode (Markdown / HTML / LaTeX)
    python translate.py README.md --from en --to zh --format markdown

Environment:
    DEEPROUTER_API_KEY   Your DeepRouter API key (required — get it from the DeepRouter dashboard)
    DEEPROUTER_MODEL     Model name (default: deepseek-chat)
"""

import argparse
import os
import re
import sys
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










# ── Language map ──────────────────────────────────────────────────────────────

LANG_NAMES = {
    "en": "English", "zh": "Simplified Chinese", "zh-tw": "Traditional Chinese",
    "ja": "Japanese", "ko": "Korean", "fr": "French", "de": "German",
    "es": "Spanish", "pt": "Portuguese", "it": "Italian", "ru": "Russian",
    "ar": "Arabic", "hi": "Hindi", "vi": "Vietnamese", "th": "Thai",
    "id": "Indonesian", "ms": "Malay", "nl": "Dutch", "pl": "Polish",
    "tr": "Turkish", "sv": "Swedish", "no": "Norwegian", "da": "Danish",
    "fi": "Finnish", "cs": "Czech", "ro": "Romanian", "hu": "Hungarian",
    "uk": "Ukrainian",
}

DEFAULT_MODEL = "deepseek-chat"
MAX_CHUNK_CHARS = 4000


# ── Prompt builder ────────────────────────────────────────────────────────────

def build_system(from_lang: str, to_lang: str, fmt: str, glossary: dict[str, str]) -> str:
    from_name = LANG_NAMES.get(from_lang, from_lang)
    to_name = LANG_NAMES.get(to_lang, to_lang)

    format_rules = {
        "markdown": (
            "FORMATTING: Preserve all Markdown syntax exactly: "
            "headers (#, ##), bold (**), italic (*), code blocks (```), "
            "inline code (`), links ([text](url)), lists (-, *, 1.), tables. "
            "Translate only the human-readable text; do not translate URLs, "
            "code block contents, or HTML attribute values."
        ),
        "html": (
            "FORMATTING: Preserve all HTML tags, attributes, and structure. "
            "Translate only text node content. Do not translate attribute values "
            "except for alt/title/placeholder/aria-label."
        ),
        "latex": (
            "FORMATTING: Preserve all LaTeX commands, environments, and math. "
            "Do not translate \\command{} names. Translate only text in "
            "\\text{}, section titles, captions, and body prose."
        ),
        "plain": (
            "FORMATTING: Plain text input. Preserve paragraph structure and "
            "line breaks as they appear."
        ),
    }.get(fmt, "FORMATTING: Preserve all formatting exactly as given.")

    glossary_block = ""
    if glossary:
        terms = "\n".join(f"  {src} → {tgt}" for src, tgt in glossary.items())
        glossary_block = f"""
DOMAIN GLOSSARY (use these translations EXACTLY, consistently throughout):
{terms}
"""

    return f"""You are a professional translator specialising in {from_name} to {to_name} translation.

TASK: Translate the submitted text from {from_name} to {to_name}.

RULES:
1. Produce natural, fluent {to_name} — not a word-for-word literal translation
2. Preserve the original meaning, tone, and register (formal/informal)
3. Technical terms and proper nouns: use standard {to_name} equivalents or
   keep in source language with a bracketed translation on first occurrence
4. Do NOT add explanatory notes, translators' comments, or [TN: ...] markers
   unless the source text is fundamentally ambiguous
5. Return ONLY the translated text — no preamble, no "Here is the translation:"
6. Numbers, dates, measurements: convert to {to_name} conventions where appropriate
{format_rules}
{glossary_block}"""


# ── Glossary loader ───────────────────────────────────────────────────────────

def load_glossary(path: str) -> dict[str, str]:
    """Load a glossary file.

    Supported formats:
      source_term = target_term
      source_term -> target_term
      source_term\ttarget_term
    """
    glossary: dict[str, str] = {}
    try:
        text = Path(path).read_text(encoding="utf-8")
    except FileNotFoundError:
        print(f"WARNING: Glossary file not found: {path}", file=sys.stderr)
        return {}

    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        for sep in (" = ", " -> ", " → ", "\t"):
            if sep in line:
                src, _, tgt = line.partition(sep)
                glossary[src.strip()] = tgt.strip()
                break
    return glossary


# ── Text processing ───────────────────────────────────────────────────────────

def chunk_text(text: str, max_chars: int = MAX_CHUNK_CHARS) -> list[str]:
    """Split at paragraph boundaries preserving Markdown structure."""
    # Keep headings with their following paragraph
    paragraphs = re.split(r"\n{2,}", text)
    chunks: list[str] = []
    current_parts: list[str] = []
    current_len = 0

    for para in paragraphs:
        para_len = len(para)
        if current_len + para_len + 2 > max_chars and current_parts:
            chunks.append("\n\n".join(current_parts))
            current_parts = [para]
            current_len = para_len
        else:
            current_parts.append(para)
            current_len += para_len + 2

    if current_parts:
        chunks.append("\n\n".join(current_parts))

    return chunks


# ── Core ──────────────────────────────────────────────────────────────────────

def get_client() -> OpenAI:
    api_key = os.environ.get("DEEPROUTER_API_KEY")
    if not api_key:
        print("ERROR: DEEPROUTER_API_KEY environment variable not set.", file=sys.stderr)
        print("  Get your key at https://deeprouter.co → Dashboard → API Keys", file=sys.stderr)
        print("  export DEEPROUTER_API_KEY=sk-...", file=sys.stderr)
        sys.exit(1)
    base_url = os.environ.get("DEEPROUTER_BASE_URL", "https://api.deeprouter.co/v1")
    return OpenAI(api_key=api_key, base_url=base_url)


def translate_chunk(
    client: OpenAI,
    text: str,
    system: str,
    model: str,
) -> str:
    resp = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": system},
            {"role": "user", "content": text},
        ],
        temperature=0.2,
        max_tokens=4096,
    )
    return resp.choices[0].message.content or ""


def translate_text(
    client: OpenAI,
    text: str,
    from_lang: str,
    to_lang: str,
    fmt: str,
    glossary: dict[str, str],
    model: str,
    verbose: bool = False,
) -> str:
    system = build_system(from_lang, to_lang, fmt, glossary)
    chunks = chunk_text(text)
    if len(chunks) == 1:
        return translate_chunk(client, text, system, model)
    results = []
    for i, chunk in enumerate(chunks, 1):
        if verbose:
            print(f"  Translating chunk {i}/{len(chunks)}...", file=sys.stderr)
        results.append(translate_chunk(client, chunk, system, model))
    return "\n\n".join(results)


def output_path(src: Path, outdir: Path | None, to_lang: str) -> Path:
    stem = src.stem
    suffix = src.suffix
    new_name = f"{stem}.{to_lang}{suffix}"
    base = outdir if outdir else src.parent
    return base / new_name


# ── CLI ───────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Translate documents using DeepSeek Chat.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument("input", help="File or directory to translate")
    parser.add_argument("--from", dest="from_lang", default="en",
                        help="Source language code (default: en)")
    parser.add_argument("--to", dest="to_lang", default="zh",
                        help="Target language code (default: zh)")
    parser.add_argument("--format", dest="fmt",
                        choices=["markdown", "html", "latex", "plain"],
                        default="markdown",
                        help="Document format for formatting preservation (default: markdown)")
    parser.add_argument("--glossary",
                        help="Path to glossary file (source = target, one per line)")
    parser.add_argument("--ext",
                        help="File extensions to include when input is a directory (e.g. md,txt)")
    parser.add_argument("--outdir",
                        help="Output directory for batch translation")
    parser.add_argument("--model",
                        help=f"Model override (default: {DEFAULT_MODEL})")
    parser.add_argument("--out",
                        help="Output file for single-file translation (default: stdout)")
    parser.add_argument("--verbose", action="store_true",
                        help="Show chunk-level progress")
    args = parser.parse_args()

    from_lang: str = args.from_lang
    to_lang: str = args.to_lang
    model = args.model or os.environ.get("DEEPROUTER_MODEL", DEFAULT_MODEL)

    glossary: dict[str, str] = {}
    if args.glossary:
        glossary = load_glossary(args.glossary)
        print(f"Loaded {len(glossary)} glossary terms.", file=sys.stderr)

    client = get_client()
    input_path = Path(args.input)

    if not input_path.exists():
        print(f"ERROR: Path not found: {args.input}", file=sys.stderr)
        sys.exit(1)

    # Single file
    if input_path.is_file():
        text = input_path.read_text(encoding="utf-8")
        print(f"Translating {input_path} ({from_lang} → {to_lang})...", file=sys.stderr)
        result = translate_text(client, text, from_lang, to_lang,
                                args.fmt, glossary, model, verbose=args.verbose)
        if args.out:
            Path(args.out).write_text(result, encoding="utf-8")
            print(f"Written to {args.out}", file=sys.stderr)
        else:
            print(result)
        return

    # Directory batch
    extensions = {"." + e.lstrip(".") for e in args.ext.split(",")} if args.ext else {".md", ".txt", ".rst"}
    outdir = Path(args.outdir) if args.outdir else None
    if outdir:
        outdir.mkdir(parents=True, exist_ok=True)

    files = [f for f in sorted(input_path.rglob("*"))
             if f.is_file() and f.suffix in extensions]

    if not files:
        print(f"No matching files found under {args.input}", file=sys.stderr)
        sys.exit(0)

    for i, src in enumerate(files, 1):
        dest = output_path(src, outdir, to_lang)
        print(f"[{i}/{len(files)}] {src} → {dest}", file=sys.stderr)
        text = src.read_text(encoding="utf-8")
        result = translate_text(client, text, from_lang, to_lang,
                                args.fmt, glossary, model, verbose=args.verbose)
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(result, encoding="utf-8")

    print(f"\nDone. {len(files)} file(s) translated.", file=sys.stderr)


if __name__ == "__main__":
    main()
