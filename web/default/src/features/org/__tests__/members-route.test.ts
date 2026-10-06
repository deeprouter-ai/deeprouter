// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { QueryClient } from '@tanstack/react-query'
import { isRedirect } from '@tanstack/react-router'
import { Route } from '@/routes/_authenticated/org/members'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { OrgMembership } from '../types'

// Enterprise Org P3: who may open /org/members. The backend checks every
// call again; this gate only keeps people off a page that has nothing for
// them — and must not turn a manager away because the question failed.

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

/** A staff membership with the given fields changed. */
function membership(over: Partial<OrgMembership>): OrgMembership {
  return {
    org_id: 1,
    org_name: 'Acme',
    is_owner: false,
    is_admin: false,
    role: 'staff',
    role_scope: 'self',
    permissions: [],
    department_id: 1,
    ...over,
  }
}

/** Runs the route's gate; resolves to the redirect target, or null when let in. */
async function gate(): Promise<string | null> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const beforeLoad = Route.options.beforeLoad as (args: {
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

describe('/org/members gate', () => {
  it('lets the owner and an admin in', async () => {
    mockFetchOrgMembership.mockResolvedValue(membership({ is_owner: true }))
    expect(await gate()).toBeNull()
    mockFetchOrgMembership.mockResolvedValue(membership({ is_admin: true }))
    expect(await gate()).toBeNull()
  })

  it('sends other members and personal accounts to the 403 page', async () => {
    for (const answer of [
      membership({ role: 'staff' }),
      membership({ role: 'manager' }),
      membership({ role: 'readonly' }),
      null,
    ]) {
      mockFetchOrgMembership.mockResolvedValue(answer)
      expect(await gate()).toBe('/403')
    }
  })

  it('does not call a failed lookup a refusal', async () => {
    // Offline or rate-limited: the page loads and shows its own retry.
    mockFetchOrgMembership.mockRejectedValue(new Error('429'))
    expect(await gate()).toBeNull()
  })
})
