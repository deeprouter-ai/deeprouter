-- Step 0 (Phase 1): Retire V1 skill tables by renaming them.
-- Run this on production BEFORE deploying the V2 P1 code.
-- These renames are fast and reversible. The _v1_bak_* tables remain until P1 is
-- verified in production, at which point run 2026-09-01-cleanup-v1-bak-tables.sql.

ALTER TABLE skills                      RENAME TO _v1_bak_skills;
ALTER TABLE skill_versions              RENAME TO _v1_bak_skill_versions;
ALTER TABLE user_enabled_skills         RENAME TO _v1_bak_user_enabled_skills;
ALTER TABLE skill_audit_log             RENAME TO _v1_bak_skill_audit_log;
ALTER TABLE skill_purchase_orders       RENAME TO _v1_bak_skill_purchase_orders;
ALTER TABLE skill_entitlements          RENAME TO _v1_bak_skill_entitlements;
ALTER TABLE skill_telemetry_quarantines RENAME TO _v1_bak_skill_telemetry_quarantines;
ALTER TABLE skill_usage_events          RENAME TO _v1_bak_skill_usage_events;
ALTER TABLE user_saved_skills           RENAME TO _v1_bak_user_saved_skills;

-- Drop V1 columns added to the shared users table
ALTER TABLE users DROP COLUMN IF EXISTS tier2_telemetry_consent;
ALTER TABLE users DROP COLUMN IF EXISTS tier2_telemetry_consented_at;
