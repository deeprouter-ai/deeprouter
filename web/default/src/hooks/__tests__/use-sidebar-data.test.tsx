// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { membershipOf } from '@/features/org/__tests__/fixtures'
import type { OrgMembership } from '@/features/org/types'
import { useSidebarData } from '../use-sidebar-data'

// Enterprise Org P3 and P4: the "Organization" group of the sidebar exists for
// members whose role shows them the organization's members and roles. Everyone
// else — personal accounts above all — must see the sidebar exactly as it was.

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
  it('is there, right after General, for everyone who may see members', async () => {
    for (const viewer of [
      membershipOf('owner'),
      membershipOf('admin'),
      membershipOf('manager'),
      membershipOf('readonly'),
    ]) {
      const groups = await sidebarGroupsFor(viewer)
      expect(
        groups.map((group) => group.id),
        viewer.role
      ).toEqual(['general', 'org', 'personal', 'admin'])
      const org = groups[1]
      expect(org.title).toBe('Organization')
      expect(org.items).toEqual([
        expect.objectContaining({
          title: 'Members & departments',
          url: '/org/members',
        }),
        expect.objectContaining({
          title: 'Roles & permissions',
          url: '/org/roles',
        }),
      ])
    }
  })

  it('is absent for staff, and for a role that only sees usage', async () => {
    for (const viewer of [
      membershipOf('staff'),
      membershipOf('staff', {
        role: 'Finance Ops',
        role_scope: 'org',
        permissions: ['usage.read'],
      }),
    ]) {
      const groups = await sidebarGroupsFor(viewer)
      expect(
        groups.map((group) => group.id),
        viewer.role
      ).toEqual(['general', 'personal', 'admin'])
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
