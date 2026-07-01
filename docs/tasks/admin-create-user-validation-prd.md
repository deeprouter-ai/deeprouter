# Admin Create User Validation PRD

Status: eval

Date: 2026-07-01

## Context

Admin Users can open the create-user drawer and submit a new user. The backend
requires `username`, `password`, and a role lower than the caller. The password
must satisfy the existing model validation of 8-20 characters.

The current frontend schema marks `password` as optional for the shared
create/update form. That is correct for update, but create mode can submit an
empty or invalid password and then only show a generic create failure after the
backend rejects it. This makes the system look like it cannot add users.

## Goals

- In create mode, require a password before calling `POST /api/user/`.
- Match backend password constraints in the create drawer: 8-20 characters.
- Keep update mode behavior unchanged: empty password means keep the current
  password.
- Verify root/super-admin can create common and admin users, while creating
  another root user remains blocked.

## Non-Goals

- Do not allow creating root users from the Admin UI.
- Do not change existing role hierarchy rules.
- Do not redesign the Users page.

## Acceptance Criteria

- Creating a user without a password shows a field-level password validation
  error and does not call the create-user API.
- Creating a user with a password shorter than 8 or longer than 20 characters
  shows a field-level password validation error and does not call the API.
- Creating a common/admin user with a valid password succeeds under a root
  caller in backend coverage.
- Backend coverage confirms creating a root user is rejected.

## Evaluation Notes

- Backend focused and controller/router tests passed.
- Full Go suite passed after generating `web/default/dist` with the frontend
  production build.
- Focused Users create drawer tests passed with coverage.
- Frontend typecheck, build, i18n sync, touched-file lint, and full Vitest suite
  passed.
- Full-project frontend lint still fails on pre-existing unrelated lint debt
  outside the touched files.
