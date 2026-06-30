# DR-108 Admin Users Skill Usage UI PRD

Status: eval
Ticket: DR-108

## Context

DR-94 shipped the Super Admin-only backend endpoint for per-user Skill usage:

`GET /api/v1/admin/users/{user_id}/skill-usage`

The endpoint is consent-gated, audit-logged, and intentionally excludes raw prompts, raw inputs, raw outputs, provider payloads, metadata, instruction templates, and package contents. The Admin UI does not yet expose this support/compliance view.

## Goal

Add a root-only Skill Usage drill-down from Admin -> Users so operators can answer which Skills a selected user downloaded or used, and how many input/output/total tokens and estimated USD cost each Skill consumed.

## Scope

- Add a row action in Admin -> Users for Super Admin/root users only.
- Open a dialog for the selected user.
- Call `GET /api/v1/admin/users/{user_id}/skill-usage`.
- Show consent status and Kids protection status.
- Show downloaded Skills with enabled state, timestamps, token totals, and estimated USD cost.
- Show a compact bounded usage timeline with event type, Skill, model, tokens, cost, and success.
- Add loading, error, empty, and non-consent states.
- Add translations for all new user-visible strings across the currently registered default frontend locales, en and zh.

## Out of Scope

- Backend API changes.
- CSV/export support.
- Prompt/raw content inspection.
- Support-role access.
- Adding token columns directly to the Users table.

## Privacy And Permissions

- Only Super Admin/root users may see the UI affordance.
- Non-consented users must show an empty privacy state and no usage rows.
- The UI must not display raw prompts, raw input/output, provider payloads, metadata, instruction templates, or package contents.
- Kids-protected sessions remain pseudonymized; the UI only renders the endpoint response.

## Acceptance

- Root users can open Skill Usage from a user row in Admin -> Users.
- Non-root users do not see the Skill Usage row action.
- Consented responses render downloaded Skills with per-Skill token totals and cost.
- Consented responses render the bounded usage timeline.
- Non-consented responses render a privacy/consent empty state.
- API error state is handled.
- New strings are translated for all currently registered default frontend locales.
- Focused frontend tests cover root visibility, non-root hidden action, consented rendering, non-consent state, and API error state.

## Evaluation Notes

- 2026-07-01: Implemented Admin -> Users row action and Skill Usage dialog, moved to eval. Test evidence recorded in `docs/test-results/dr108-admin-users-skill-usage-ui.txt`.
