# YouTube Analyzer — Reference Guide

## How it works

This skill uses **yt-dlp** to fetch YouTube public data — no YouTube API key or
Google account required. yt-dlp is the community-maintained successor to
youtube-dl and is used by millions of developers worldwide.

DeepRouter AI then analyzes the fetched data.

---

## Installation

```bash
pip install openai yt-dlp
export DEEPROUTER_API_KEY=sk-...
```

That's all. No YouTube API key. No OAuth. No credit card.

---

## Accepted Input Formats

All commands accept a full URL **or** a bare video/channel ID:

```bash
# Full URL
python analyze.py video https://www.youtube.com/watch?v=dQw4w9WgXcQ

# Short URL
python analyze.py video https://youtu.be/dQw4w9WgXcQ

# Bare video ID (11 characters)
python analyze.py video dQw4w9WgXcQ

# Channel by handle
python analyze.py channel https://www.youtube.com/@mkbhd
python analyze.py channel @mkbhd

# Channel by ID
python analyze.py channel UCBcRF18a7Qf58cCRy5xuWwQ
```

---

## Command Reference

### `video` — Analyze a video
```bash
python analyze.py video dQw4w9WgXcQ
```
Fetches: title, channel, views, likes, comment count, duration, tags, description.
Returns: summary, key topics, audience appeal, improvement suggestions, SEO tags.

### `comments` — Sentiment analysis
```bash
python analyze.py comments dQw4w9WgXcQ
```
Fetches top 50 comments (sorted by likes).
Returns: overall sentiment score, top themes, positives, pain points, content ideas.

Note: comment fetching takes 10-20 seconds. Some videos have comments disabled.

### `channel` — Channel analysis
```bash
python analyze.py channel @mkbhd
```
Fetches channel name, subscriber count, top 10 videos.
Returns: health assessment, content strategy, growth opportunities, 30-day action plan.

### `search` — Trend research
```bash
python analyze.py search "Python tutorial 2025"
```
Fetches top 10 YouTube search results.
Returns: trend analysis, content gaps, best video recommendation, 5 content ideas.

### `script` — Script outline
```bash
python analyze.py script dQw4w9WgXcQ
```
Fetches title, description, duration, tags.
Returns: hook, intro, main sections with timestamps, CTA, thumbnail concept.

---

## Model Override

```bash
# Use a different model
python analyze.py video dQw4w9WgXcQ --model deepseek-reasoner

# Via environment variable
export DEEPROUTER_MODEL=deepseek-reasoner
python analyze.py video dQw4w9WgXcQ
```

---

## Limitations of yt-dlp (no API key) vs. YouTube Data API

| Feature | yt-dlp (this skill) | YouTube Data API |
|---|---|---|
| Video metadata | ✓ Full | ✓ Full |
| View/like counts | ✓ | ✓ |
| Comments | ✓ (top 50, slower) | ✓ (faster) |
| Channel info | ✓ | ✓ |
| Search | ✓ | ✓ |
| Subtitles/transcripts | ✓ (bonus!) | ✗ |
| API Key required | **No** | Yes (free) |
| Rate limits | Soft (respectful) | 10,000 units/day |

---

## Troubleshooting

**`ERROR: Could not fetch data`**
- Check the URL is a public video (not private or age-restricted without login)
- Try the full URL format: `https://www.youtube.com/watch?v=VIDEO_ID`

**Comments take a long time**
- Normal — yt-dlp fetches comment pages sequentially. Top 50 comments takes ~15s.

**Channel shows few videos**
- The script fetches 10 videos. For channels with playlists, use the playlist URL instead.

**yt-dlp needs updating**
```bash
pip install -U yt-dlp
```
YouTube changes its internals frequently; keeping yt-dlp updated fixes most issues.
