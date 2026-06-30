# DR-105 Pro Paid Skill Seeds PRD

Status: eval
Owner: DeepRouter
Ticket: DR-105
Source of truth: `/Users/kl/Downloads/Deep_Router/Jira_V2/paid_skills_seed.md`

## Problem

DR-51 seeds four free demo Skills. The marketplace also needs four Plus/Pro-gated demo Skills that prove the paid-wall and entitlement loop: free users can discover the Skills but cannot download or run them, active Plus and Enterprise users can download and run them with their own DeepRouter key, and expired Plus users are blocked at use time without billing.

## Scope

- Seed four additional published, active, packaged Skills:
  - `research-synthesizer-pro`
  - `legal-clause-reviewer-pro`
  - `pr-architecture-reviewer-pro`
  - `financial-modeler-pro`
- All four use `required_plan=pro`, `monetization_type=plan_included`, `price_markup=0`, `free_quota_per_month=null`, `model_whitelist=["smart-tier"]`.
- Reuse the existing DR-46/47/48/79 seed path that creates draft metadata, active versions, publish state, and downloadable packages.
- Preserve D-09 safeguards: DeepRouter routing work step, tier aliases instead of provider models, separated structured inputs, server-authoritative `skill_version_id`, and runner own-key billing.
- Add regression coverage that seeded Pro Skills are published/versioned/packaged and that download entitlement gates reject free users while allowing active Plus and Enterprise users.

## Non-Goals

- No new entitlement model, checkout flow, subscription implementation, or UI surface.
- No raw demo prompt/output analytics storage.
- No provider credentials in packages.

## Acceptance

- Seeder creates eight total demo Skills: four DR-51 free Skills and four DR-105 Pro Skills.
- The four Pro Skills are published, have active versions, valid `smart-tier` whitelist snapshots, populated example I/O with synthetic data, and package work steps that call `/v1/routing/chat/completions`.
- Free users downloading seeded Pro Skills receive `SKILL_PLAN_REQUIRED`.
- Active Plus and Enterprise users can download seeded Pro Skills.
- Existing pricing/availability regression continues to cover expired Plus `SKILL_SUBSCRIPTION_INACTIVE` and `renew` CTA.
