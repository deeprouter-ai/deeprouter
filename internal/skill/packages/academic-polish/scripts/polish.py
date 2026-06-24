#!/usr/bin/env python3
"""Academic Paper Polish — powered by DeepSeek Chat.

Polishes academic text: fixes grammar, improves clarity, strengthens
argument flow, and ensures consistent academic register. Supports
English and Chinese manuscripts.

Requirements:
    pip install openai

Usage:
    # Polish a text file
    python polish.py paper.txt

    # Polish from stdin
    cat paper.txt | python polish.py --stdin

    # Interactive mode (paste text, press Ctrl+D when done)
    python polish.py --interactive

    # Choose language variant
    python polish.py paper.txt --lang zh

    # Save output to file
    python polish.py paper.txt --out polished.txt

Environment:
    DEEPROUTER_API_KEY   Your DeepRouter API key (required — get it from the DeepRouter dashboard)
    DEEPROUTER_MODEL     Model name (default: deepseek-chat)
"""

import argparse
import os
import sys

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










# ── Prompts ───────────────────────────────────────────────────────────────────

SYSTEM_EN = """You are an expert academic editor with 20+ years of experience
polishing manuscripts for top-tier journals (Nature, Science, Cell, IEEE, ACM).

Your task: polish the submitted text while preserving the author's original
meaning, data, and argument structure.

Editing rules:
1. Fix grammatical errors and awkward phrasing
2. Improve sentence clarity and conciseness — eliminate redundancy
3. Strengthen academic register: replace informal language with precise terms
4. Ensure consistent tense (past tense for completed work, present for theory)
5. Improve paragraph transitions and argument flow
6. Fix punctuation and citation formatting (do not alter citation keys/numbers)
7. Do NOT add new content, claims, or data — only rephrase existing material
8. Preserve all technical terms, model names, variable names exactly

Output format:
- Return ONLY the polished text, no commentary
- Preserve original paragraph structure and heading levels
- Mark significant changes with [EDITED] tag on that line (optional; remove if clean)"""

SYSTEM_ZH = """你是一位拥有20年以上经验的学术论文编辑专家，专注于中文顶级期刊（Nature中文版、
Science、IEEE、ACM等）的稿件打磨。

你的任务：在不改变作者原意、数据和论点结构的前提下，润色提交的文本。

编辑规则：
1. 修正语法错误和表达不当之处
2. 提高句子清晰度和简洁性，消除冗余
3. 强化学术语体：用精准术语替换口语化表达
4. 确保时态一致（已完成工作用过去时，理论陈述用一般现在时）
5. 改善段落过渡和论证流畅性
6. 修正标点符号和引用格式（不得改变引用编号/键值）
7. 不得添加新内容、主张或数据——仅对现有内容改写
8. 保留所有专业术语、模型名称、变量名称原样不变

输出格式：
- 只返回润色后的文本，不附任何说明
- 保留原有段落结构和标题层级"""


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


def polish(text: str, lang: str = "en", model: str | None = None) -> str:
    """Send text to DeepSeek and return the polished version."""
    client = get_client()
    system = SYSTEM_ZH if lang == "zh" else SYSTEM_EN
    chosen_model = model or os.environ.get("DEEPROUTER_MODEL", "deepseek-chat")

    response = client.chat.completions.create(
        model=chosen_model,
        messages=[
            {"role": "system", "content": system},
            {"role": "user", "content": text},
        ],
        temperature=0.3,   # low temperature for consistent, conservative edits
        max_tokens=8192,
    )
    return response.choices[0].message.content or ""


def chunk_text(text: str, max_chars: int = 6000) -> list[str]:
    """Split text at paragraph boundaries to stay within token limits."""
    paragraphs = text.split("\n\n")
    chunks: list[str] = []
    current = ""
    for para in paragraphs:
        if len(current) + len(para) + 2 > max_chars and current:
            chunks.append(current.strip())
            current = para
        else:
            current = (current + "\n\n" + para).lstrip()
    if current:
        chunks.append(current.strip())
    return chunks


def polish_long(text: str, lang: str = "en", model: str | None = None) -> str:
    """Polish long documents by splitting into chunks and reassembling."""
    chunks = chunk_text(text)
    if len(chunks) == 1:
        return polish(text, lang, model)

    polished_chunks = []
    for i, chunk in enumerate(chunks, 1):
        print(f"  Polishing chunk {i}/{len(chunks)}...", file=sys.stderr)
        polished_chunks.append(polish(chunk, lang, model))
    return "\n\n".join(polished_chunks)


# ── CLI ───────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Polish academic text using DeepSeek Chat.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument("input", nargs="?", help="Input text file (.txt, .md, .tex)")
    parser.add_argument("--stdin", action="store_true", help="Read from stdin")
    parser.add_argument("--interactive", action="store_true", help="Interactive paste mode")
    parser.add_argument("--lang", choices=["en", "zh"], default="en",
                        help="Manuscript language (default: en)")
    parser.add_argument("--model", help="DeepSeek model override (default: deepseek-chat)")
    parser.add_argument("--out", help="Write output to file instead of stdout")
    args = parser.parse_args()

    # Read input
    if args.interactive:
        print("Paste your text below. Press Ctrl+D (Unix) or Ctrl+Z (Windows) when done:")
        text = sys.stdin.read()
    elif args.stdin or not args.input:
        if sys.stdin.isatty() and not args.stdin:
            parser.print_help()
            sys.exit(0)
        text = sys.stdin.read()
    else:
        try:
            with open(args.input, encoding="utf-8") as f:
                text = f.read()
        except FileNotFoundError:
            print(f"ERROR: File not found: {args.input}", file=sys.stderr)
            sys.exit(1)

    text = text.strip()
    if not text:
        print("ERROR: Input text is empty.", file=sys.stderr)
        sys.exit(1)

    print(f"Polishing {len(text)} characters ({lang_label(args.lang)}) via DeepRouter...", file=sys.stderr)
    result = polish_long(text, lang=args.lang, model=args.model)

    if args.out:
        with open(args.out, "w", encoding="utf-8") as f:
            f.write(result)
        print(f"Written to {args.out}", file=sys.stderr)
    else:
        print(result)


def lang_label(lang: str) -> str:
    return {"en": "English", "zh": "Chinese"}.get(lang, lang)


if __name__ == "__main__":
    main()
