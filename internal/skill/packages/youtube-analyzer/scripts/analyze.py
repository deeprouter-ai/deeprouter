#!/usr/bin/env python3
"""YouTube Analyzer — powered by DeepRouter.

Fetches YouTube video/channel/comment data (no API key required — uses yt-dlp)
and runs AI analysis via DeepRouter.

Requirements:
    pip install openai yt-dlp

Usage:
    python analyze.py video <URL_or_ID>        Summarize + analyze a video
    python analyze.py comments <URL_or_ID>     Analyze comment sentiment
    python analyze.py channel <URL_or_ID>      Channel performance analysis
    python analyze.py search <QUERY>           Search + trend analysis
    python analyze.py script <URL_or_ID>       Generate script outline

Environment:
    DEEPROUTER_API_KEY   Your DeepRouter API key (required)
    DEEPROUTER_MODEL     Model override (default: deepseek-chat)

Examples:
    python analyze.py video https://www.youtube.com/watch?v=dQw4w9WgXcQ
    python analyze.py video dQw4w9WgXcQ
    python analyze.py comments dQw4w9WgXcQ
    python analyze.py search "Python tutorial 2025"
    python analyze.py channel https://www.youtube.com/@mkbhd
"""

import argparse
import os
import sys

try:
    import yt_dlp
except ImportError:



    print("ERROR: yt-dlp not found. Run: pip install openai yt-dlp", file=sys.stderr)
    sys.exit(1)

try:
    from openai import OpenAI
except ImportError:
    print("ERROR: openai not found. Run: pip install openai yt-dlp", file=sys.stderr)
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







# ── Helpers ───────────────────────────────────────────────────────────────────

def to_url(id_or_url: str) -> str:
    """Accept a video/channel URL or bare ID and return a full URL."""
    if id_or_url.startswith("http"):
        return id_or_url
    # 11-char alphanumeric → YouTube video ID
    if len(id_or_url) == 11 and id_or_url.replace("-", "").replace("_", "").isalnum():
        return f"https://www.youtube.com/watch?v={id_or_url}"
    # channel handle or ID
    if id_or_url.startswith("@") or id_or_url.startswith("UC"):
        return f"https://www.youtube.com/{id_or_url}"
    return id_or_url


def ydl_extract(url: str, opts: dict) -> dict:
    base = {"quiet": True, "no_warnings": True, "ignoreerrors": True}
    base.update(opts)
    with yt_dlp.YoutubeDL(base) as ydl:
        info = ydl.extract_info(url, download=False)
    if not info:
        print(f"ERROR: Could not fetch data for: {url}", file=sys.stderr)
        sys.exit(1)




    return info


# ── DeepRouter client ─────────────────────────────────────────────────────────

def get_client() -> OpenAI:
    key = os.environ.get("DEEPROUTER_API_KEY")
    if not key:
        print("ERROR: DEEPROUTER_API_KEY not set.", file=sys.stderr)
        print("  Get your key at https://deeprouter.co → Dashboard → API Keys", file=sys.stderr)
        sys.exit(1)
    return OpenAI(api_key=key, base_url=os.environ.get("DEEPROUTER_BASE_URL", "https://api.deeprouter.co/v1"))


def ai(system: str, user: str, model: str) -> str:
    client = get_client()
    resp = client.chat.completions.create(
        model=model,
        messages=[{"role": "system", "content": system}, {"role": "user", "content": user}],
        temperature=0.3,
        max_tokens=2048,
    )
    return resp.choices[0].message.content or ""


# ── Commands ──────────────────────────────────────────────────────────────────

def cmd_video(id_or_url: str, model: str) -> None:
    url = to_url(id_or_url)
    print(f"Fetching video metadata...", file=sys.stderr)
    info = ydl_extract(url, {})

    context = f"""Title: {info.get('title', 'N/A')}
Channel: {info.get('channel', 'N/A')}
Published: {info.get('upload_date', 'N/A')}
Views: {info.get('view_count', 'N/A')}
Likes: {info.get('like_count', 'N/A')}
Comments: {info.get('comment_count', 'N/A')}
Duration: {info.get('duration_string', 'N/A')}
Tags: {', '.join((info.get('tags') or [])[:15])}
Description:
{(info.get('description') or '')[:1500]}"""

    system = """You are a YouTube content strategist. Analyze a video's metadata and provide:
## Summary (2-3 sentences)
## Key Topics (bullet list)
## Audience Appeal — why this performs well or not
## Improvement Suggestions — 3 concrete ideas to improve title, thumbnail, or description
## SEO Tags — 10 suggested tags"""

    print(f"Analyzing via DeepRouter ({model})...", file=sys.stderr)
    print(ai(system, context, model))


def cmd_comments(id_or_url: str, model: str) -> None:
    url = to_url(id_or_url)
    print(f"Fetching comments (this may take 10-20 seconds)...", file=sys.stderr)
    info = ydl_extract(url, {
        "getcomments": True,
        "extractor_args": {"youtube": {"comment_sort": ["top"], "max_comments": ["50"]}},
    })

    comments = info.get("comments") or []
    if not comments:
        print("No comments available for this video.")
        return

    text = "\n".join(
        f'[{c.get("like_count", 0)} likes] {(c.get("text") or "")[:200]}'
        for c in comments[:50]
    )

    system = """You are a social media analyst. Analyse YouTube comments and provide:
## Overall Sentiment (score 1-10, one sentence)
## Top Themes — what viewers talk about most (5 bullet points)
## Positive Signals — what the audience loves
## Pain Points / Criticisms
## Content Opportunities — 3 ideas for follow-up videos based on comment requests"""

    print(f"Analyzing {len(comments)} comments via DeepRouter ({model})...", file=sys.stderr)
    print(ai(system, f"Comments:\n{text}", model))


def cmd_channel(id_or_url: str, model: str) -> None:
    url = to_url(id_or_url)
    print(f"Fetching channel info...", file=sys.stderr)
    info = ydl_extract(url, {
        "playlistend": 10,
        "extract_flat": True,
    })

    title = info.get("channel") or info.get("uploader") or info.get("title", "N/A")
    entries = info.get("entries") or []
    top_videos = "\n".join(
        f"- {e.get('title', 'N/A')} ({e.get('view_count', '?')} views)"
        for e in entries[:10]
    )

    context = f"""Channel: {title}
Subscriber Count: {info.get('channel_follower_count', 'N/A')}
Total Videos: {len(entries)} (fetched)
Description: {(info.get('description') or '')[:500]}
Recent/Top Videos:
{top_videos}"""

    system = """You are a YouTube channel growth strategist. Analyse the channel and provide:
## Channel Health Assessment
## Content Strategy — what topics/formats work best
## Growth Opportunities — 3 specific tactics
## Benchmark Comparison — how this channel likely compares in its niche
## Action Plan — top 5 priorities for the next 30 days"""

    print(f"Analyzing channel via DeepRouter ({model})...", file=sys.stderr)
    print(ai(system, context, model))


def cmd_search(query: str, model: str) -> None:
    print(f"Searching YouTube for: {query}...", file=sys.stderr)
    info = ydl_extract(f"ytsearch10:{query}", {"extract_flat": True})

    entries = info.get("entries") or []
    if not entries:
        print("No results found.")
        return

    items_text = "\n".join(
        f"{i+1}. [{e.get('channel', '?')}] {e.get('title', 'N/A')} — {e.get('view_count', '?')} views"
        for i, e in enumerate(entries)
    )

    system = """You are a content research analyst. Given YouTube search results, provide:
## Trend Analysis — what topics dominate these results
## Content Gap — what's missing that viewers likely want
## Best Video to Watch First — and why
## Content Ideas — 5 original video ideas that could rank for this query"""

    print(f"Analyzing {len(entries)} results via DeepRouter ({model})...", file=sys.stderr)
    print(ai(system, f"Search: {query}\n\nResults:\n{items_text}", model))


def cmd_script(id_or_url: str, model: str) -> None:
    url = to_url(id_or_url)
    print(f"Fetching video for script outline...", file=sys.stderr)
    info = ydl_extract(url, {})

    # Try to get auto-generated subtitle/transcript text for better outline
    subtitles = info.get("automatic_captions") or info.get("subtitles") or {}
    transcript_hint = ""
    if subtitles:
        lang = next(iter(subtitles), None)
        transcript_hint = f"\n(Subtitles available in: {', '.join(list(subtitles.keys())[:5])})"

    context = f"""Title: {info.get('title', 'N/A')}
Channel: {info.get('channel', 'N/A')}
Duration: {info.get('duration_string', 'N/A')}
Tags: {', '.join((info.get('tags') or [])[:10])}{transcript_hint}
Description:
{(info.get('description') or '')[:2000]}"""

    system = """You are a video script writer. Based on the video metadata, create:
## Hook (0-30s) — opening line that stops the scroll
## Intro (30s-2min) — promise + credibility
## Main Sections (outline with 4-6 key points and approximate timestamps)
## Call to Action — what to ask viewers to do
## Thumbnail Concept — visual idea that would drive clicks"""

    print(f"Generating script outline via DeepRouter ({model})...", file=sys.stderr)
    print(ai(system, context, model))


# ── CLI ───────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Analyze YouTube content using DeepRouter AI (no YouTube API key needed).",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    sub = parser.add_subparsers(dest="cmd", required=True)

    for name, help_text in [
        ("video", "Summarize and analyze a video"),
        ("comments", "Analyze comment sentiment"),
        ("channel", "Channel performance analysis"),
        ("search", "Search and analyze results"),
        ("script", "Generate script outline from video"),
    ]:
        p = sub.add_parser(name, help=help_text)
        p.add_argument("id", help="YouTube URL, video ID, channel URL, or search query")
        p.add_argument("--model", default=None)

    args = parser.parse_args()
    model = args.model or os.environ.get("DEEPROUTER_MODEL", "deepseek-chat")

    dispatch = {
        "video": cmd_video,
        "comments": cmd_comments,
        "channel": cmd_channel,
        "search": cmd_search,
        "script": cmd_script,
    }
    dispatch[args.cmd](args.id, model)


if __name__ == "__main__":
    main()
