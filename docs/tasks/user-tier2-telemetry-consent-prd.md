# User Tier 2 Telemetry Consent PRD

Status: eval
Ticket: unassigned

## Context

DR-94 and DR-108 protect per-user Skill usage drill-down behind `users.tier2_telemetry_consent`. When the flag is false, Admin -> Users -> Skill usage shows only a privacy state and no rows. The database field and read-side guards exist, but users currently have no product surface to grant or revoke the consent themselves.

## Goal

Add a caller-scoped Profile / Privacy control that lets the signed-in user enable or disable Tier 2 telemetry consent. Admins may view consent state through existing usage surfaces, but must not be able to enable consent on behalf of a user.

## Scope

- Add authenticated APIs for the current user to read and update their own Tier 2 telemetry consent.
- Include consent state in `/api/user/self` for Profile hydration.
- Add a Profile privacy card with a clear switch, consent timestamp, and privacy copy.
- Persist `tier2_telemetry_consent=true` and `tier2_telemetry_consented_at=now` when enabled.
- Persist `tier2_telemetry_consent=false` when disabled; retain the last consent timestamp for audit context.
- Keep Admin user edit paths unable to mutate the consent flag.
- Add frontend i18n for new user-visible strings in the currently registered locales.

## Out Of Scope

- Changing Admin Skill usage authorization.
- Storing or displaying raw prompts, raw input/output, provider payloads, metadata, package contents, or instruction templates.
- Adding an Admin override for consent.
- Backfilling consent for existing users.

## Privacy And Permissions

- Only the current authenticated user can update their own Tier 2 telemetry consent.
- Disabling consent must immediately cause downstream per-user Skill usage drill-down to remain hidden under existing DR-94/DR-108 checks.
- Kids privacy rules remain unchanged.

## Acceptance

- A signed-in user can view Tier 2 telemetry consent state from Profile / Privacy.
- A signed-in user can enable consent and sees the state update.
- A signed-in user can disable consent and sees the state update.
- `/api/user/telemetry-consent` does not accept a target `user_id` and cannot update another user.
- Admin user edit APIs do not expose or mutate the flag.
- Profile UI explains that aggregate Skill usage metadata can be used while raw content and provider payloads are not stored or shown.
- Focused backend and frontend tests cover read, enable, disable, failure, and UI state updates.

## Evaluation Notes

- 2026-07-01: PRD created and moved to build.
- 2026-07-01: Implemented caller-scoped consent APIs and Profile / Privacy card, then moved to eval after focused backend/frontend tests, full Go suite, full frontend test, typecheck, build, i18n sync, and touched-file lint passed.
