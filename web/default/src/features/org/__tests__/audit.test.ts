// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'
import {
  AUDIT_TARGET_TYPES,
  auditActionLabel,
  auditChangeText,
  auditChanges,
  auditTargetLabel,
  weekdayLabel,
} from '../lib/audit'
import type { OrgAuditLog } from '../types'

// Enterprise Org P9 (meta-repo docs/enterprise-org-prd.md D17): how a record
// of the audit log reads on the page. A record stores codes and raw values —
// "key.update", remain_quota 5000000 — and the page turns them into words
// without ever dropping a fact: whatever it has no words for shows as it is.

/** A translator that fills in the placeholders and changes nothing else. */
const t = ((key: string, values?: Record<string, unknown>) =>
  key.replace(/{{(\w+)}}/g, (_, name: string) =>
    String(values?.[name] ?? '')
  )) as unknown as TFunction

/** A record as `GET /api/org/audit-logs` lists one. */
function recordOf(over: Partial<OrgAuditLog>): OrgAuditLog {
  return {
    id: 1,
    actor_user_id: 1,
    actor: 'Fiona Founder',
    action: 'key.update',
    target_type: 'key',
    target_id: 100,
    target: 'Design tools',
    detail: null,
    ip: '203.0.113.7',
    created_time: 1790000000,
    ...over,
  }
}

/** What a record says changed, one line each. */
function linesOf(over: Partial<OrgAuditLog>): string[] {
  return auditChanges(t, recordOf(over)).map((change) =>
    auditChangeText(t, change)
  )
}

/**
 * Every action the backend records (internal/org/model/audit.go). A new one
 * belongs here and in auditActionLabel.
 */
const BACKEND_ACTIONS = [
  'department.create',
  'department.rename',
  'department.delete',
  'role.create',
  'role.update',
  'role.delete',
  'role.assign',
  'member.move',
  'member.manages',
  'member.invite',
  'invite.revoke',
  'member.join',
  'member.remove',
  'service_account.create',
  'key.create',
  'key.update',
  'key.rotate',
  'key.freeze',
  'key.unfreeze',
  'key.delete',
  'key.assign',
  'key.reclaim',
  'key.deliver',
  'alert.handle',
  'org.settings',
]

describe('what was done, in words', () => {
  it('has words of its own for every action the backend records', () => {
    const labels = BACKEND_ACTIONS.map((action) => auditActionLabel(t, action))
    for (const [i, action] of BACKEND_ACTIONS.entries()) {
      expect(labels[i], action).not.toBe(action)
      expect(labels[i], action).not.toMatch(/[._]/)
    }
    expect(new Set(labels).size).toBe(BACKEND_ACTIONS.length)
  })

  it('shows an action it has no words for as its code', () => {
    expect(auditActionLabel(t, 'key.teleport')).toBe('key.teleport')
  })

  it('has words for every kind of thing a record can be about', () => {
    expect(AUDIT_TARGET_TYPES).toEqual([
      'key',
      'member',
      'role',
      'department',
      'invite',
      'alert',
      'organization',
    ])
    const labels = AUDIT_TARGET_TYPES.map((type) => auditTargetLabel(t, type))
    for (const [i, type] of AUDIT_TARGET_TYPES.entries()) {
      expect(labels[i], type).not.toBe(type)
    }
    expect(new Set(labels).size).toBe(AUDIT_TARGET_TYPES.length)
    expect(auditTargetLabel(t, 'starship')).toBe('starship')
  })
})

describe('what a record says changed', () => {
  it('lists only what differs between before and after', () => {
    expect(
      linesOf({
        action: 'key.update',
        detail: {
          before: {
            name: 'Design tools',
            holder_id: 3,
            holder: 'sally',
            department_id: 11,
            department: 'Sales',
            policy_template: '',
            model_limits: [],
            remain_quota: 5000000,
            unlimited_quota: false,
            expired_time: -1,
            rpm_limit: 0,
            tpm_limit: 0,
            monthly_limit: 0,
          },
          after: {
            name: 'Design tools',
            holder_id: 3,
            holder: 'sally',
            department_id: 11,
            department: 'Sales',
            policy_template: 'coding',
            model_limits: ['claude-sonnet-5', 'gpt-4o'],
            remain_quota: 10000000,
            unlimited_quota: false,
            expired_time: 1792000000,
            rpm_limit: 60,
            tpm_limit: 0,
            monthly_limit: 1000,
          },
        },
      })
    ).toEqual([
      'Policy template: None → Coding pack',
      'May call: Every model → claude-sonnet-5, gpt-4o',
      'Quota left: $10 → $20',
      expect.stringMatching(/^Valid until: Never expires → 20\d\d-\d\d-\d\d/),
      'Requests per minute: No limit → 60',
      'Requests per month: No limit → 1000',
    ])
  })

  it('says a rename, and nothing when nothing differs', () => {
    expect(
      linesOf({
        action: 'department.rename',
        target_type: 'department',
        target: 'Labs',
        detail: { before: { name: 'Research' }, after: { name: 'Labs' } },
      })
    ).toEqual(['Name: Research → Labs'])
    expect(
      linesOf({ detail: { before: { name: 'Same' }, after: { name: 'Same' } } })
    ).toEqual([])
  })

  it('says what a new thing was made with, leaving out what was left unset', () => {
    expect(
      linesOf({
        action: 'key.create',
        detail: {
          after: {
            name: 'Design tools',
            holder_id: 3,
            holder: 'sally',
            department_id: 11,
            department: 'Sales',
            policy_template: '',
            model_limits: [],
            remain_quota: 0,
            unlimited_quota: true,
            expired_time: -1,
            rpm_limit: 0,
            tpm_limit: 0,
            monthly_limit: 0,
          },
        },
      })
      // The name is the row's own heading; the rest was left at "none".
    ).toEqual(['Held by: sally', 'Department: Sales', 'No quota limit: Yes'])
  })

  it('does not say a switch that was left off, nor a list that was left empty', () => {
    expect(
      linesOf({
        action: 'key.create',
        detail: {
          after: {
            name: 'Design tools',
            holder: 'sally',
            department: 'Sales',
            remain_quota: 5000000,
            unlimited_quota: false,
            value_shown: false,
            tools: [],
            rpm_limit: 0,
          },
        },
      })
    ).toEqual(['Held by: sally', 'Department: Sales', 'Quota left: $10'])
  })

  it('says what a removed thing was', () => {
    expect(
      linesOf({
        action: 'member.remove',
        target_type: 'member',
        target: 'Mia Member',
        detail: {
          before: {
            name: 'Mia Member',
            username: 'mia',
            role_id: 3,
            role: 'manager',
            department_id: 11,
            department: 'Sales',
            key_ids: [100, 101],
          },
        },
      })
    ).toEqual([
      'Username: mia',
      'Role: Manager',
      'Department: Sales',
      'Keys taken back: 2',
    ])
  })

  it('keeps the name of something whose record is all there is to call it by', () => {
    // An account deleted outright: the row has no name to show, so the one
    // in the record is not left out.
    expect(
      linesOf({
        action: 'member.remove',
        target_type: 'member',
        target: '',
        detail: { before: { name: 'Mia Member', key_ids: [] } },
      })
    ).toEqual(['Name: Mia Member'])
  })

  it('says what was added to a list and what was taken out of it', () => {
    const role = {
      id: 9,
      name: 'Key Desk',
      scope: 'org',
      powers: [],
      is_preset: false,
    }
    expect(
      linesOf({
        action: 'role.update',
        target_type: 'role',
        target: 'Key Desk',
        detail: {
          before: { ...role, permissions: ['key.read', 'key.freeze'] },
          after: {
            ...role,
            scope: 'dept',
            permissions: ['key.read', 'key.assign', 'member.read'],
          },
        },
      })
    ).toEqual([
      'Reach: Whole organization → Departments they manage',
      'Permissions: added Assign keys, View members; removed Freeze keys',
    ])
  })

  it('reads a role, a state and a masked key as the pages show them', () => {
    expect(
      linesOf({
        action: 'role.assign',
        target_type: 'member',
        detail: {
          before: { role_id: 4, role: 'staff' },
          after: { role_id: 9, role: 'Key Desk' },
        },
      })
    ).toEqual(['Role: Staff → Key Desk'])
    expect(
      linesOf({
        action: 'key.reclaim',
        detail: {
          before: {
            name: 'Design tools',
            holder_id: 3,
            holder: 'sally',
            department_id: 11,
            department: 'Sales',
            status: 1,
            key: 'abcd**********wxyz',
          },
          after: {
            name: 'Design tools',
            holder_id: 1,
            holder: 'Fiona Founder',
            department_id: 10,
            department: 'General',
            status: 2,
            key: 'efgh**********stuv',
          },
        },
      })
    ).toEqual([
      'Held by: sally → Fiona Founder',
      'Department: Sales → General',
      'Status: Working → Frozen',
      'Key: sk-abcd**********wxyz → sk-efgh**********stuv',
    ])
  })

  it('states a fact that is only recorded when it is true', () => {
    expect(
      linesOf({
        action: 'key.rotate',
        detail: {
          before: { name: 'Nightly build', key: 'ci01**********ci99' },
          after: {
            name: 'Nightly build',
            key: 'ci02**********ci77',
            value_shown: true,
          },
        },
      })
    ).toEqual([
      'Key: sk-ci01**********ci99 → sk-ci02**********ci77',
      'Full value shown: Yes',
    ])
  })

  it('counts the people a deletion moved, without listing ids', () => {
    expect(
      linesOf({
        action: 'department.delete',
        target_type: 'department',
        target: 'Sales',
        detail: { before: { name: 'Sales', member_ids: [3, 5, 8] } },
      })
    ).toEqual(['Members affected: 3'])
    expect(
      linesOf({
        action: 'member.manages',
        target_type: 'member',
        target: 'mona',
        detail: {
          before: { department_ids: [11] },
          after: { department_ids: [11, 12, 13] },
        },
      })
    ).toEqual(['Departments managed: 1 → 3'])
  })

  it('always says which rule an alert was about', () => {
    expect(
      linesOf({
        action: 'alert.handle',
        target_type: 'alert',
        target: 'Design tools',
        detail: {
          before: { rule: 'spike', key: 'Design tools', state: '' },
          after: { rule: 'spike', key: 'Design tools', state: 'false_alarm' },
        },
      })
    ).toEqual([
      'Alert: Sudden rise in spending',
      'Status: Unresolved → False alarm',
    ])
    for (const [rule, words] of [
      ['quota', 'Quota warning'],
      ['monthly', 'Monthly requests warning'],
      ['offhours', 'Spending outside working hours'],
      ['new_ip', 'Unfamiliar IP address'],
      ['from_the_future', 'from_the_future'],
    ]) {
      expect(
        linesOf({
          action: 'alert.handle',
          detail: {
            before: { rule, state: '' },
            after: { rule, state: 'handled' },
          },
        })[0]
      ).toBe(`Alert: ${words}`)
    }
  })

  it('reads a change of the alert settings field by field', () => {
    const before = {
      warn_at: [80, 100],
      spike_multiple: 5,
      off_hours_percent: 50,
      min_spend: 500000,
      work_hours: null,
    }
    expect(
      linesOf({
        action: 'org.settings',
        target_type: 'organization',
        target: '',
        detail: {
          before,
          after: {
            warn_at: [50, 80, 100],
            spike_multiple: 8,
            off_hours_percent: 75,
            min_spend: 2500000,
            work_hours: {
              timezone: 'Asia/Shanghai',
              days: [1, 2, 3, 4, 5],
              start: 540,
              end: 1080,
            },
          },
        },
      })
    ).toEqual([
      'Warning levels: added 50%',
      'Sudden rise: 5 times the daily average → 8 times the daily average',
      'Outside working hours: 50% of the daily average → 75% of the daily average',
      'Smallest amount reported: $1 → $5',
      'Working hours: Not set → Asia/Shanghai · Mon, Tue, Wed, Thu, Fri · 09:00–18:00',
    ])
    // Switching the warnings off altogether is a change to "none", not a list
    // of what was taken out.
    expect(
      linesOf({
        action: 'org.settings',
        detail: { before, after: { ...before, warn_at: [] } },
      })
    ).toEqual(['Warning levels: 80%, 100% → None'])
  })

  it('shows a field it does not know under its own name, as it is', () => {
    expect(
      linesOf({
        detail: {
          before: { colour: 'red', sides: 3 },
          after: { colour: 'blue', sides: 3 },
        },
      })
    ).toEqual(['colour: red → blue'])
  })

  it('has nothing to say about a record without detail', () => {
    expect(linesOf({ detail: null })).toEqual([])
    expect(linesOf({ detail: {} })).toEqual([])
  })
})

describe('the days of the week', () => {
  it('are numbered from Sunday, as the backend stores them', () => {
    expect([0, 1, 2, 3, 4, 5, 6].map((day) => weekdayLabel(t, day))).toEqual([
      'Sun',
      'Mon',
      'Tue',
      'Wed',
      'Thu',
      'Fri',
      'Sat',
    ])
    expect(weekdayLabel(t, 9)).toBe('9')
  })
})
