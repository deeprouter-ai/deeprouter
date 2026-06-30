# DR-98 Referral Skill Purchase Grant Isolation PRD

Status: eval

## Context

PR #134 added referral rewards for top-up, PLUS subscription, and one-time Skill purchase conversions. Top-up and subscription reward grants are best-effort: reward failures are logged and do not fail the primary payment flow.

The Skill purchase path currently calls referral reward granting inside the purchase transaction and returns the grant error. A transient referral reward failure can therefore roll back the successful Skill purchase path, leaving the user without the Skill after completing payment.

## Scope

- Make one-time Skill purchase referral rewards best-effort, matching top-up and subscription behavior.
- Keep Skill order creation, wallet debit, durable entitlement, enablement, and purchase event emission authoritative even if referral reward granting fails.
- Log referral reward failures with user, source, and order reference context.
- Preserve idempotency: repeated successful purchase calls may retry the referral conversion grant, but rewards must still be granted at most once by the referral service.
- Add a regression test that forces referral reward failure and verifies the Skill purchase still succeeds.

## Non-Goals

- No retry worker, dead-letter queue, or admin repair UI.
- No changes to referral reward ledger schema or fraud rules.
- No changes to top-up or subscription referral grant behavior.

## Acceptance

- If referral reward granting errors during a successful one-time Skill purchase, the API still returns purchase success.
- The order is saved as succeeded, the buyer receives the entitlement, and the wallet debit remains committed.
- The referral record is not partially rewarded when reward grant fails.
- Duplicate successful purchase calls do not double-charge or double-award referral rewards.
