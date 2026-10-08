// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { QueryClient } from '@tanstack/react-query'
import { isRedirect } from '@tanstack/react-router'
import { Route as KeysRoute } from '@/routes/_authenticated/org/keys'
import { Route as MembersRoute } from '@/routes/_authenticated/org/members'
import { Route as RolesRoute } from '@/routes/_authenticated/org/roles'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { OrgMembership } from '../types'
import { membershipOf } from './fixtures'

// Enterprise Org P3 to P5: who may open the organization pages. The backend
// checks every call again; this gate only keeps people off pages that have
// nothing for them — and must not turn anyone away because the question
// failed. The pages about people take member.read, the keys page key.read.

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

type GatedRoute = { options: { beforeLoad?: unknown } }

/** Runs a route's gate; resolves to the redirect target, or null when let in. */
async function gate(route: GatedRoute): Promise<string | null> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const beforeLoad = route.options.beforeLoad as (args: {
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

describe.each([
  ['/org/members', MembersRoute as GatedRoute],
  ['/org/roles', RolesRoute as GatedRoute],
])('%s gate', (_path, route) => {
  it('lets in everyone whose role shows them members', async () => {
    const allowed: OrgMembership[] = [
      membershipOf('owner'),
      membershipOf('admin'),
      membershipOf('manager'),
      membershipOf('readonly'),
      // A custom role: HR Ops holds member.read like the presets above.
      membershipOf('staff', {
        role: 'HR Ops',
        role_scope: 'org',
        permissions: ['member.read', 'member.invite'],
      }),
    ]
    for (const answer of allowed) {
      mockFetchOrgMembership.mockResolvedValue(answer)
      expect(await gate(route), answer.role).toBeNull()
    }
  })

  it('sends staff, roles that do not show members and personal accounts to the 403 page', async () => {
    for (const answer of [
      membershipOf('staff'),
      membershipOf('staff', {
        role: 'Finance Ops',
        role_scope: 'org',
        permissions: ['usage.read'],
      }),
      // Seeing keys is not seeing people.
      membershipOf('staff', {
        role: 'Key Desk',
        role_scope: 'org',
        permissions: ['key.read', 'key.freeze'],
      }),
      null,
    ]) {
      mockFetchOrgMembership.mockResolvedValue(answer)
      expect(await gate(route), answer?.role ?? 'personal').toBe('/403')
    }
  })

  it('does not call a failed lookup a refusal', async () => {
    // Offline or rate-limited: the page loads and shows its own retry.
    mockFetchOrgMembership.mockRejectedValue(new Error('429'))
    expect(await gate(route)).toBeNull()
  })
})

describe('/org/keys gate', () => {
  const route = KeysRoute as GatedRoute

  it('lets in everyone whose role shows them keys', async () => {
    const allowed: OrgMembership[] = [
      membershipOf('owner'),
      membershipOf('admin'),
      membershipOf('manager'),
      membershipOf('readonly'),
      // A custom role that sees keys and no people.
      membershipOf('staff', {
        role: 'Key Desk',
        role_scope: 'org',
        permissions: ['key.read', 'key.freeze'],
      }),
    ]
    for (const answer of allowed) {
      mockFetchOrgMembership.mockResolvedValue(answer)
      expect(await gate(route), answer.role).toBeNull()
    }
  })

  it('sends staff, roles that do not show keys and personal accounts to the 403 page', async () => {
    for (const answer of [
      membershipOf('staff'),
      // Seeing people is not seeing keys.
      membershipOf('staff', {
        role: 'HR',
        role_scope: 'org',
        permissions: ['member.read', 'member.invite'],
      }),
      membershipOf('staff', {
        role: 'Finance Ops',
        role_scope: 'org',
        permissions: ['usage.read'],
      }),
      null,
    ]) {
      mockFetchOrgMembership.mockResolvedValue(answer)
      expect(await gate(route), answer?.role ?? 'personal').toBe('/403')
    }
  })

  it('does not call a failed lookup a refusal', async () => {
    mockFetchOrgMembership.mockRejectedValue(new Error('429'))
    expect(await gate(route)).toBeNull()
  })
})
