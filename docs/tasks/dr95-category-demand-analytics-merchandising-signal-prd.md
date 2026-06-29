# DR-95 Category Demand Analytics and Merchandising Signal PRD

Status: eval

Ticket: DR-95
Reference: NEW-21 gap review 2026-06-28; dependencies DR-73, DR-75, NEW-12, NEW-13

## Problem

Skill Marketplace analytics rank individual Skills, but operators cannot see category-level demand or use that demand to merchandise newly published Skills. When a category such as video is hot, DeepRouter should help Admin decide what to build next and help customers discover fresh Skills in that category.

## Goals

- Aggregate Skill downloads and successful usage by category over 7-day and 30-day windows.
- Expose an Admin "Category demand" ranking with trend signals for hot/rising categories.
- Boost newly published Skills in hot categories on public Featured / New-this-week rails.
- Mark category-demand surfaced interactions with `entry_point=category_demand`.
- Keep the signal aggregate-only; do not expose per-user rows.

## Non-Goals

- No per-user category drill-down.
- No personalized category ranking changes.
- No raw prompt, provider payload, or event metadata exposure.
- No paid promotion configuration UI.

## Product Requirements

- Admin Skill Analytics shows a Category Demand panel with 7d and 30d downloads, successful runs, demand score, and trend percentage.
- Public marketplace rail responses can identify Skills boosted because their category is hot.
- Skill cards display a compact badge for hot-category boosted Skills.
- Opening a hot-category boosted Skill records discovery attribution as `category_demand`.

## Data and Privacy

- Category demand is computed from `skills.category` joined to aggregate `skill_usage_events`.
- Download demand counts `skill_enabled` and `skill_purchased` events from package, detail, and purchase flows already used for download leaderboards.
- Usage demand counts successful `skill_used` events.
- Analytics excludes `admin_preview` and Kids sessions by default, matching DR-75 semantics.
- API responses include aggregate category rows only.

## Acceptance

- Admin sees category demand ranking and trend over 7d/30d windows.
- Newly published Skills in hot categories are sorted ahead of otherwise equivalent New-this-week entries and carry a hot-category badge.
- Detail clicks from those badges/boosted rails are recorded with `entry_point=category_demand`.
- Tests cover backend aggregation, rail boosting, frontend badge rendering, and attribution.

## Implementation Notes

- Added aggregate-only `GET /api/v1/ops/skill-analytics/category-demand`.
- Added `category_demand` entry point to enum/model/migration checks.
- New-this-week rail applies hot-category ordering and returns badge/attribution metadata.
- Skill Analytics renders a Category Demand panel; Marketplace cards show a Hot category badge and record category-demand attribution.

## Verification

See `docs/test-results/dr95-category-demand-analytics-merchandising-signal.txt`.
