// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import type { OrgAlertState, OrgAuditLog, OrgWorkHours } from '../types'
import { alertStateLabel } from './alerts'
import { keyStateLabel, keyTemplateLabel, type OrgKeyState } from './keys'
import { permissionLabel, scopeLabel } from './permissions'
import { orgRoleLabel } from './roles'

/**
 * The kinds of thing an audit record can be about, in the order the filter
 * offers them (the backend's `AuditTarget…` names).
 */
export const AUDIT_TARGET_TYPES = [
  'key',
  'member',
  'role',
  'department',
  'invite',
  'alert',
  'organization',
]

/** What kind of thing a record is about, in words. */
export function auditTargetLabel(t: TFunction, targetType: string): string {
  switch (targetType) {
    case 'key':
      return t('Key')
    case 'member':
      return t('Member')
    case 'role':
      return t('Role')
    case 'department':
      return t('Department')
    case 'invite':
      return t('Invite link')
    case 'alert':
      return t('Alert')
    case 'organization':
      return t('Alert settings')
    default:
      return targetType
  }
}

/**
 * What was done, in words. An action the page has no words for yet — one
 * added on the backend first — shows as its code.
 */
export function auditActionLabel(t: TFunction, action: string): string {
  switch (action) {
    case 'department.create':
      return t('Created a department')
    case 'department.rename':
      return t('Renamed a department')
    case 'department.delete':
      return t('Deleted a department')
    case 'role.create':
      return t('Created a role')
    case 'role.update':
      return t('Changed a role')
    case 'role.delete':
      return t('Deleted a role')
    case 'role.assign':
      return t('Gave a member another role')
    case 'member.move':
      return t('Moved a member to another department')
    case 'member.manages':
      return t('Changed which departments a member manages')
    case 'member.invite':
      return t('Created an invite link')
    case 'invite.revoke':
      return t('Revoked an invite link')
    case 'member.join':
      return t('Joined through an invite link')
    case 'member.remove':
      return t('Removed a member')
    case 'service_account.create':
      return t('Added a service account')
    case 'key.create':
      return t('Created a key')
    case 'key.update':
      return t('Changed a key')
    case 'key.rotate':
      return t('Gave a key a new value')
    case 'key.freeze':
      return t('Froze a key')
    case 'key.unfreeze':
      return t('Unfroze a key')
    case 'key.delete':
      return t('Deleted a key')
    case 'key.assign':
      return t('Assigned a key')
    case 'key.reclaim':
      return t('Took a key back')
    case 'key.deliver':
      return t('Installed their key with one-click setup')
    case 'alert.handle':
      return t('Marked an alert')
    case 'org.settings':
      return t('Changed the alert settings')
    default:
      return action
  }
}

/**
 * One thing a record says about its target: what it is called, and what it was
 * and what it became. A creation has only `after`, a deletion only `before`.
 */
export type AuditChange = {
  label: string
  before?: string
  after?: string
}

/**
 * Fields a record carries for machines: ids whose names are next to them, and
 * what a role or member view happens to include.
 */
const UNSAID = new Set([
  'id',
  'role_id',
  'department_id',
  'holder_id',
  'invite_id',
  'is_preset',
  'is_owner',
  'powers',
  'email',
  'key_count',
  'managed_department_ids',
])

/** Fields that say which thing a record is about, shown even when unchanged. */
const ALWAYS_SAID = new Set(['rule'])

/** Lists of ids, which a record shows as how many there are. */
const COUNTED = new Set(['department_ids', 'member_ids', 'key_ids'])

/** Whether a value is a list with something in it. */
function isList(value: unknown): value is unknown[] {
  return Array.isArray(value) && value.length > 0
}

/** The gateway's key statuses, as the states the keys page shows. */
const KEY_STATES: Record<number, OrgKeyState> = {
  1: 'active',
  2: 'frozen',
  3: 'expired',
  4: 'exhausted',
}

/** A list of things in a sentence, or a word for none. */
function listed(t: TFunction, items: string[], none: string): string {
  return items.length > 0 ? items.join(t(', ')) : none
}

/** Minutes after midnight as a clock time: 540 is "09:00". */
function clock(minutes: number): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(minutes / 60) % 24)}:${pad(minutes % 60)}`
}

/** A day of the week by its number, 0 being Sunday. */
export function weekdayLabel(t: TFunction, day: number): string {
  return (
    [t('Sun'), t('Mon'), t('Tue'), t('Wed'), t('Thu'), t('Fri'), t('Sat')][
      day
    ] ?? String(day)
  )
}

/** An alert rule by name, without the level a warning was raised at. */
function ruleLabel(t: TFunction, rule: string): string {
  switch (rule) {
    case 'quota':
      return t('Quota warning')
    case 'monthly':
      return t('Monthly requests warning')
    case 'spike':
      return t('Sudden rise in spending')
    case 'offhours':
      return t('Spending outside working hours')
    case 'new_ip':
      return t('Unfamiliar IP address')
    default:
      return rule
  }
}

/** What a field of a record is called. A field the page does not know keeps its own name. */
function fieldLabel(t: TFunction, field: string): string {
  switch (field) {
    case 'name':
    case 'display_name':
      return t('Name')
    case 'username':
      return t('Username')
    case 'role':
      return t('Role')
    case 'department':
      return t('Department')
    case 'holder':
      return t('Held by')
    case 'status':
    case 'state':
      return t('Status')
    case 'key':
      return t('Key')
    case 'value_shown':
      return t('Full value shown')
    case 'policy_template':
      return t('Policy template')
    case 'model_limits':
      return t('May call')
    case 'remain_quota':
      return t('Quota left')
    case 'unlimited_quota':
      return t('No quota limit')
    case 'expired_time':
      return t('Valid until')
    case 'rpm_limit':
      return t('Requests per minute')
    case 'tpm_limit':
      return t('Tokens per minute')
    case 'monthly_limit':
      return t('Requests per month')
    case 'scope':
      return t('Reach')
    case 'permissions':
      return t('Permissions')
    case 'department_ids':
      return t('Departments managed')
    case 'member_ids':
      return t('Members affected')
    case 'key_ids':
      return t('Keys taken back')
    case 'is_service':
      return t('Service account')
    case 'tools':
      return t('Installed into')
    case 'expires_time':
      return t('Link expires')
    case 'rule':
      return t('Alert')
    case 'warn_at':
      return t('Warning levels')
    case 'spike_multiple':
      return t('Sudden rise')
    case 'off_hours_percent':
      return t('Outside working hours')
    case 'min_spend':
      return t('Smallest amount reported')
    case 'work_hours':
      return t('Working hours')
    default:
      return field
  }
}

/** What a field held, in words. */
function fieldText(t: TFunction, field: string, value: unknown): string {
  if (typeof value === 'boolean') return value ? t('Yes') : t('No')
  if (Array.isArray(value)) {
    switch (field) {
      case 'permissions':
        return listed(
          t,
          value.map((name) => permissionLabel(t, String(name))),
          t('None')
        )
      case 'model_limits':
        return listed(t, value.map(String), t('Every model'))
      case 'warn_at':
        return listed(
          t,
          value.map((level) => `${level}%`),
          t('None')
        )
      default:
        if (COUNTED.has(field)) return String(value.length)
        return listed(t, value.map(String), t('None'))
    }
  }
  if (field === 'work_hours') {
    if (value === null || typeof value !== 'object') return t('Not set')
    const hours = value as OrgWorkHours
    return [
      hours.timezone,
      hours.days.map((day) => weekdayLabel(t, day)).join(t(', ')),
      `${clock(hours.start)}–${clock(hours.end)}`,
    ].join(' · ')
  }
  if (typeof value === 'number') {
    switch (field) {
      case 'status':
        return KEY_STATES[value]
          ? keyStateLabel(t, KEY_STATES[value])
          : String(value)
      case 'remain_quota':
      case 'min_spend':
        return formatQuota(value)
      case 'expired_time':
        return value === -1 ? t('Never expires') : formatTimestampToDate(value)
      case 'expires_time':
        return formatTimestampToDate(value)
      case 'rpm_limit':
      case 'tpm_limit':
      case 'monthly_limit':
        return value === 0 ? t('No limit') : String(value)
      case 'spike_multiple':
        return t('{{times}} times the daily average', { times: value })
      case 'off_hours_percent':
        return t('{{percent}}% of the daily average', { percent: value })
      default:
        return String(value)
    }
  }
  const text = value === null || value === undefined ? '' : String(value)
  switch (field) {
    case 'role':
      return orgRoleLabel(t, text)
    case 'scope':
      return scopeLabel(t, text)
    case 'policy_template':
      return text ? keyTemplateLabel(t, text) : t('None')
    case 'key':
      // The masked form the keys page shows; a record never holds a value.
      return /\*/.test(text) ? `sk-${text}` : text
    case 'state':
      return alertStateLabel(t, text as OrgAlertState)
    case 'rule':
      return ruleLabel(t, text)
    default:
      return text
  }
}

/**
 * Whether a value says nothing in the record of something made or removed:
 * a limit that was not set, a switch left off, an empty list.
 */
function saysNothing(value: unknown): boolean {
  return (
    value === null ||
    value === undefined ||
    value === '' ||
    value === false ||
    value === 0 ||
    value === -1 ||
    (Array.isArray(value) && value.length === 0)
  )
}

/**
 * What a record says happened to its target, one line per thing. Where the
 * record has a before and an after, only what differs between them is listed;
 * where it has one of the two — something was made, or removed — what it was
 * made with, leaving out what was left unset and the name the row already
 * shows.
 */
export function auditChanges(t: TFunction, log: OrgAuditLog): AuditChange[] {
  const before = log.detail?.before
  const after = log.detail?.after
  const fields = [
    ...new Set([...Object.keys(after ?? {}), ...Object.keys(before ?? {})]),
  ].filter((field) => !UNSAID.has(field))

  const changes: AuditChange[] = []
  for (const field of fields) {
    const was = before && field in before ? before[field] : undefined
    const is = after && field in after ? after[field] : undefined
    const wasText = was === undefined ? undefined : fieldText(t, field, was)
    const isText = is === undefined ? undefined : fieldText(t, field, is)
    if (ALWAYS_SAID.has(field)) {
      changes.push({ label: fieldLabel(t, field), after: isText ?? wasText })
      continue
    }
    if (before && after) {
      if (wasText === isText) continue
      // Two long lists side by side hide the one entry that moved: a list
      // that changed says what was added to it and what was taken out.
      if (isList(was) && isList(is) && !COUNTED.has(field)) {
        const added = is.filter((item) => !was.includes(item))
        const removed = was.filter((item) => !is.includes(item))
        changes.push({
          label: fieldLabel(t, field),
          after: [
            added.length > 0 &&
              t('added {{items}}', { items: fieldText(t, field, added) }),
            removed.length > 0 &&
              t('removed {{items}}', { items: fieldText(t, field, removed) }),
          ]
            .filter(Boolean)
            .join(t('; ')),
        })
        continue
      }
    } else {
      const value = after ? is : was
      if (saysNothing(value)) continue
      if (
        (field === 'name' || field === 'display_name') &&
        value === log.target
      ) {
        continue
      }
    }
    changes.push({
      label: fieldLabel(t, field),
      before: wasText,
      after: isText,
    })
  }
  return changes
}

/** A change in one line: "Quota left: $10 → $20", or "Role: Staff". */
export function auditChangeText(t: TFunction, change: AuditChange): string {
  const value =
    change.before !== undefined && change.after !== undefined
      ? `${change.before} → ${change.after}`
      : (change.after ?? change.before ?? '')
  return t('{{label}}: {{value}}', { label: change.label, value })
}
