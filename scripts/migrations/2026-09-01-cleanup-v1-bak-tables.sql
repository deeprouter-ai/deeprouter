-- Step 0 (Phase 2): Drop the V1 backup tables.
-- Run this ONLY after P1 has been verified in production and there is no need
-- to roll back to V1. This is irreversible.

DROP TABLE IF EXISTS _v1_bak_skills;
DROP TABLE IF EXISTS _v1_bak_skill_versions;
DROP TABLE IF EXISTS _v1_bak_user_enabled_skills;
DROP TABLE IF EXISTS _v1_bak_skill_audit_log;
DROP TABLE IF EXISTS _v1_bak_skill_purchase_orders;
DROP TABLE IF EXISTS _v1_bak_skill_entitlements;
DROP TABLE IF EXISTS _v1_bak_skill_telemetry_quarantines;
DROP TABLE IF EXISTS _v1_bak_skill_usage_events;
DROP TABLE IF EXISTS _v1_bak_user_saved_skills;
