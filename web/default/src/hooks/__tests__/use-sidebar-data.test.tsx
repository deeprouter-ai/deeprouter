// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { membershipOf } from '@/features/org/__tests__/fixtures'
import type { OrgMembership } from '@/features/org/types'
import { useSidebarData } from '../use-sidebar-data'

// Enterprise Org P3 to P9: the "Organization" group of the sidebar holds the
// pages a member's role shows them — the two about people for member.read, the
// keys page for key.read, the reports page for any of usage.read, alert.read
// and audit.read. Everyone else — personal accounts above all — must see the
// sidebar exactly as it was.

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
  it('is there, right after General, with all four pages for the preset roles that read everything in their reach', async () => {
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
        expect.objectContaining({
          title: 'Organization keys',
          url: '/org/keys',
        }),
        expect.objectContaining({
          title: 'Reports & alerts',
          url: '/org/reports',
        }),
      ])
    }
  })

  it('offers each page by its own permission', async () => {
    // A key desk sees keys and no people; HR sees people and no keys. Both
    // have the reports page, like every member.
    const keyDesk = await sidebarGroupsFor(
      membershipOf('staff', {
        role: 'Key Desk',
        role_scope: 'org',
        permissions: ['key.read', 'key.freeze'],
      })
    )
    expect(keyDesk.map((group) => group.id)).toEqual([
      'general',
      'org',
      'personal',
      'admin',
    ])
    expect(keyDesk[1].items.map((item) => item.title)).toEqual([
      'Organization keys',
      'Reports & alerts',
    ])

    const hr = await sidebarGroupsFor(
      membershipOf('staff', {
        role: 'HR',
        role_scope: 'org',
        permissions: ['member.read', 'member.invite'],
      })
    )
    expect(hr[1].items.map((item) => item.title)).toEqual([
      'Members & departments',
      'Roles & permissions',
      'Reports & alerts',
    ])
  })

  it('offers the reports page to every member, whatever their role reads', async () => {
    // The page shows each reader their part — and every member has one: the
    // alerts on their own keys (PRD D47).
    for (const [role, permissions] of [
      ['Finance Ops', ['usage.read']],
      ['On call', ['alert.read']],
      ['Auditor', ['audit.read']],
      ['Staff', []],
    ] as const) {
      const groups = await sidebarGroupsFor(
        membershipOf('staff', {
          role,
          role_scope: 'org',
          permissions: [...permissions],
        })
      )
      expect(
        groups.map((group) => group.id),
        role
      ).toEqual(['general', 'org', 'personal', 'admin'])
      expect(groups[1].items, role).toEqual([
        expect.objectContaining({
          title: 'Reports & alerts',
          url: '/org/reports',
        }),
      ])
    }
  })

  it('holds the reports page alone for staff', async () => {
    const groups = await sidebarGroupsFor(membershipOf('staff'))
    expect(groups.map((group) => group.id)).toEqual([
      'general',
      'org',
      'personal',
      'admin',
    ])
    expect(groups[1].items.map((item) => item.title)).toEqual([
      'Reports & alerts',
    ])
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

// Enterprise Org P7: the wallet is the account holder's own. The keys of an
// organization's members spend the company wallet — the owner's balance — so
// only a personal account and the owner are shown the way to it.
describe('sidebar: the Wallet entry', () => {
  /** The titles of the Personal group for one viewer. */
  async function personalEntriesFor(answer: OrgMembership | null) {
    const groups = await sidebarGroupsFor(answer)
    const group = groups.find((candidate) => candidate.id === 'personal')
    return group?.items.map((item) => item.title)
  }

  it('is there for a personal account and for the owner', async () => {
    for (const viewer of [null, membershipOf('owner')]) {
      expect(await personalEntriesFor(viewer)).toEqual([
        'Home',
        'Wallet',
        'My Skills',
        'Profile',
      ])
    }
  })

  it('is not there for any other member', async () => {
    for (const role of ['admin', 'manager', 'staff', 'readonly'] as const) {
      expect(await personalEntriesFor(membershipOf(role)), role).toEqual([
        'Home',
        'My Skills',
        'Profile',
      ])
    }
  })
})
