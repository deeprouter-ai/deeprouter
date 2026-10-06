// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { OrgMembership } from '@/features/org/types'
import { useSidebarData } from '../use-sidebar-data'

// Enterprise Org P3: the "Organization" group of the sidebar exists only for
// members who run their organization. Everyone else — personal accounts above
// all — must see the sidebar exactly as it was.

const mockFetchOrgMembership = vi.hoisted(() => vi.fn())

vi.mock('@/features/org/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/features/org/api')>()),
  fetchOrgMembership: mockFetchOrgMembership,
}))
vi.mock('@/features/marketplace/api', () => ({
  marketplaceQueryKeys: { myPurchases: () => ['marketplace', 'my-purchases'] },
  fetchMyPurchases: vi.fn().mockResolvedValue({ total: 0, purchases: [] }),
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: { id: 1 } } }),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

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

/** Renders the hook and waits until the membership probe has answered. */
async function sidebarGroupsFor(answer: OrgMembership | null) {
  mockFetchOrgMembership.mockResolvedValue(answer)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const { result } = renderHook(() => useSidebarData(), { wrapper })
  await waitFor(() => expect(mockFetchOrgMembership).toHaveBeenCalled())
  await waitFor(() => expect(queryClient.isFetching()).toBe(0))
  return result.current.navGroups
}

beforeEach(() => {
  mockFetchOrgMembership.mockReset()
})

describe('sidebar: the Organization group', () => {
  it('is there for the owner and for an admin, right after General', async () => {
    for (const manager of [
      membership({ is_owner: true, role: 'owner' }),
      membership({ is_admin: true, role: 'admin' }),
    ]) {
      const groups = await sidebarGroupsFor(manager)
      expect(groups.map((group) => group.id)).toEqual([
        'general',
        'org',
        'personal',
        'admin',
      ])
      const org = groups[1]
      expect(org.title).toBe('Organization')
      expect(org.items).toEqual([
        expect.objectContaining({
          title: 'Members & departments',
          url: '/org/members',
        }),
      ])
    }
  })

  it('is absent for members who do not run the organization', async () => {
    for (const role of ['manager', 'staff', 'readonly']) {
      const groups = await sidebarGroupsFor(membership({ role }))
      expect(groups.map((group) => group.id)).toEqual([
        'general',
        'personal',
        'admin',
      ])
    }
  })

  it('is absent for a personal account', async () => {
    const groups = await sidebarGroupsFor(null)
    expect(groups.map((group) => group.id)).toEqual([
      'general',
      'personal',
      'admin',
    ])
  })

  it('stays away when the probe fails', async () => {
    mockFetchOrgMembership.mockRejectedValue(new Error('network'))
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
    const { result } = renderHook(() => useSidebarData(), { wrapper })
    await waitFor(() => expect(queryClient.isFetching()).toBe(0))
    expect(result.current.navGroups.map((group) => group.id)).toEqual([
      'general',
      'personal',
      'admin',
    ])
  })
})
