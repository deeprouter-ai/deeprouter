// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { QueryClient } from '@tanstack/react-query'
import { isRedirect } from '@tanstack/react-router'
import { Route as ReportsRoute } from '@/routes/_authenticated/org/reports'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reportSections, seesOwnAlertsOnly } from '../lib/reports'
import type { OrgMembership } from '../types'
import { membershipOf } from './fixtures'

// Enterprise Org P9: who may open "Reports & alerts", and which of its three
// sections each of them gets. Usage and the audit log take a permission —
// usage.read, audit.read. Alerts are every member's (PRD D47): a role that
// holds alert.read sees what it reaches, everyone else the warnings on their
// own keys. So the page is open to every member of an organization and shows
// each their part. The backend checks every call again; this only keeps a
// personal account off a page with nothing on it.

const mockFetchOrgMembership = vi.hoisted(() => vi.fn())

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  fetchOrgMembership: mockFetchOrgMembership,
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: Object.assign(
    (selector: (state: unknown) => unknown) =>
      selector({ auth: { user: { id: 1 } } }),
    { getState: () => ({ auth: { user: { id: 1 } } }) }
  ),
}))
// The route brings the page along, and the page its chart. Nothing is drawn
// here, and the real chart library looks for a canvas as soon as it loads.
vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))

/** A custom role that holds exactly the given primitives across the organization. */
function customRole(role: string, permissions: string[]): OrgMembership {
  return membershipOf('staff', { role, role_scope: 'org', permissions })
}

/** Runs the route's gate; resolves to the redirect target, or null when let in. */
async function gate(): Promise<string | null> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const beforeLoad = ReportsRoute.options.beforeLoad as unknown as (args: {
    context: { queryClient: QueryClient }
  }) => Promise<void>
  try {
    await beforeLoad({ context: { queryClient } })
    return null
  } catch (thrown) {
    if (isRedirect(thrown)) return String(thrown.options.to)
    throw thrown
  }
}

beforeEach(() => {
  mockFetchOrgMembership.mockReset()
})

describe('the sections of the page a member may read', () => {
  it('follow the read permissions, in tab order', () => {
    expect(reportSections(membershipOf('owner'))).toEqual([
      'usage',
      'alerts',
      'audit',
    ])
    expect(reportSections(membershipOf('admin'))).toEqual([
      'usage',
      'alerts',
      'audit',
    ])
    // A read-only member reads everything; a manager everything but the audit
    // log, which belongs to the organization as a whole.
    expect(reportSections(membershipOf('readonly'))).toEqual([
      'usage',
      'alerts',
      'audit',
    ])
    expect(reportSections(membershipOf('manager'))).toEqual(['usage', 'alerts'])
  })

  it('give a role that reads usage or the audit log that section, beside the alerts every member has', () => {
    expect(reportSections(customRole('Finance Ops', ['usage.read']))).toEqual([
      'usage',
      'alerts',
    ])
    expect(reportSections(customRole('On call', ['alert.read']))).toEqual([
      'alerts',
    ])
    expect(reportSections(customRole('Auditor', ['audit.read']))).toEqual([
      'alerts',
      'audit',
    ])
  })

  it('are the alerts alone for staff and for a role about keys and people, and none for a personal account', () => {
    expect(reportSections(membershipOf('staff'))).toEqual(['alerts'])
    expect(
      reportSections(
        customRole('HR Ops', ['member.read', 'member.invite', 'key.read'])
      )
    ).toEqual(['alerts'])
    expect(reportSections(null)).toEqual([])
    expect(reportSections(undefined)).toEqual([])
  })

  it('show the alerts of their own keys alone to whoever does not read alerts', () => {
    for (const reader of [
      membershipOf('owner'),
      membershipOf('admin'),
      membershipOf('manager'),
      membershipOf('readonly'),
      customRole('On call', ['alert.read']),
    ]) {
      expect(seesOwnAlertsOnly(reader), reader.role).toBe(false)
    }
    for (const member of [
      membershipOf('staff'),
      customRole('Finance Ops', ['usage.read']),
      customRole('Auditor', ['audit.read']),
      customRole('Key Desk', ['key.read', 'key.freeze']),
    ]) {
      expect(seesOwnAlertsOnly(member), member.role).toBe(true)
    }
  })
})

describe('/org/reports gate', () => {
  it('lets in every member of an organization, whatever their role reads', async () => {
    const members: OrgMembership[] = [
      membershipOf('owner'),
      membershipOf('admin'),
      membershipOf('manager'),
      membershipOf('readonly'),
      membershipOf('staff'),
      customRole('Finance Ops', ['usage.read']),
      customRole('On call', ['alert.read']),
      customRole('Auditor', ['audit.read']),
      customRole('HR Ops', ['member.read', 'member.invite', 'key.read']),
      customRole('Key Desk', ['key.read', 'key.freeze']),
    ]
    for (const answer of members) {
      mockFetchOrgMembership.mockResolvedValue(answer)
      expect(await gate(), answer.role).toBeNull()
    }
  })

  it('sends a personal account to the 403 page', async () => {
    mockFetchOrgMembership.mockResolvedValue(null)
    expect(await gate()).toBe('/403')
  })

  it('does not call a failed lookup a refusal', async () => {
    // Offline or rate-limited: the page loads and shows its own retry.
    mockFetchOrgMembership.mockRejectedValue(new Error('429'))
    expect(await gate()).toBeNull()
  })
})

describe('the section in the address', () => {
  /** What the route makes of a search string's parameters. */
  const read = (search: Record<string, unknown>) =>
    (
      ReportsRoute.options.validateSearch as unknown as {
        parse: (input: unknown) => { section?: string }
      }
    ).parse(search)

  it('is one of the three, so that a notification can link to the alerts', () => {
    for (const section of ['usage', 'alerts', 'audit']) {
      expect(read({ section })).toEqual({ section })
    }
  })

  it('falls back to the first section for anything else, instead of failing the page', () => {
    expect(read({})).toEqual({})
    expect(read({ section: 'payroll' })).toEqual({})
    expect(read({ section: 7 })).toEqual({})
  })
})
