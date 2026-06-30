# Skill Version Instruction Migration PRD

Status: eval
Date: 2026-06-30

## Problem

Local smart-router stack startup fails when an existing PostgreSQL `skill_versions`
table has rows from before DR-93. GORM `AutoMigrate(&SkillVersion{})` tries to add
new instruction columns such as `prerequisites` as `NOT NULL` before the custom
backfill migration runs, so PostgreSQL rejects the DDL.

## Scope

- Make `MigrateSkillVersions` safe for existing PostgreSQL databases with
  populated `skill_versions` rows and missing DR-93 instruction columns.
- Preserve existing SQLite and MySQL migration behavior.
- Add focused regression coverage for the legacy PostgreSQL shape.

## Acceptance

- Existing PostgreSQL rows receive default instruction values before strict model
  migration proceeds.
- Fresh database migrations remain unchanged.
- The smart-router compose stack can start without resetting the PostgreSQL data
  volume.
