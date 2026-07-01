# Skill Usage Consent Prompt and Admin Layout PRD

Status: eval

Date: 2026-07-01

## Context

Super Admin can open a user's Skill usage dialog, but details are hidden when
the target user has not granted Tier 2 telemetry consent. Users currently need
to discover Profile / Privacy on their own. The Admin dialog also overflows on
narrow viewports, making the right side of the Skill usage page appear cut off.

Skill package downloads are the natural moment to explain this privacy setting:
the user is about to download/run a Skill, and can choose whether per-user usage
metadata should be stored for support and Admin review.

## Goals

- When Skill usage details are hidden, tell the Admin exactly where the user
  must enable consent: Profile / Privacy.
- Before the user's first Skill package download prompt them to enable Tier 2
  telemetry consent, while still allowing download without enabling.
- Use only the current user's consent API; Admin must not grant consent for a
  different user.
- Fix the Admin Skill usage dialog so the full table is accessible on desktop
  and mobile, with horizontal scrolling instead of clipping.

## Non-Goals

- Do not default-enable consent for all users.
- Do not store raw prompts, raw input/output, provider payloads, package
  contents, or instruction templates.
- Do not change backend telemetry consent semantics.

## Acceptance Criteria

- Admin no-consent state includes a clear instruction to ask the user to enable
  Profile / Privacy -> Tier 2 telemetry consent.
- First Skill download attempt opens a consent explanation dialog when the user
  has not enabled Tier 2 consent.
- The dialog offers enable-and-download and continue-without-enabling choices.
- Enabling calls the current-user consent endpoint and then continues the
  original download.
- Skipping consent still continues the original download and does not call the
  consent update endpoint.
- The Admin Skill usage dialog is not clipped; wide tables are horizontally
  scrollable inside the modal.

## Evaluation Notes

- Skill detail now prompts before first download when Tier 2 telemetry consent is
  disabled.
- User Home and Playground recommendation downloads route through the same
  consent prompt helper.
- Admin Skill usage no-consent state now instructs Super Admin to ask the user
  to enable Profile / Privacy -> Tier 2 telemetry consent.
- Admin Skill usage modal uses viewport-bounded width plus horizontal scrolling
  for wide tables.
- Follow-up fix explicitly overrides the base dialog `sm:max-w-sm` with
  `sm:max-w-[900px]` and keeps the wide tables in horizontal scroll wrappers.
- Focused regression tests with coverage, touched-file lint, i18n sync,
  typecheck, production build, full frontend tests, and full Go suite passed.
- Full-project frontend lint still fails on pre-existing unrelated lint debt.
