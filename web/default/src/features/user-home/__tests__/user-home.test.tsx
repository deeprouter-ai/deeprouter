/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { UserHome } from '../index'
import type { UserHomeData } from '../types'

const mockGetUserHome = vi.hoisted(() => vi.fn<() => Promise<UserHomeData>>())

vi.mock('../api', () => ({
  getUserHome: mockGetUserHome,
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (!opts) return key
      return Object.entries(opts).reduce(
        (acc, [name, value]) => acc.replace(`{{${name}}}`, String(value)),
        key
      )
    },
  }),
}))

vi.mock('@/components/layout', () => {
  const Layout = ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  )
  Layout.Title = ({ children }: { children: ReactNode }) => <h1>{children}</h1>
  Layout.Description = ({ children }: { children: ReactNode }) => (
    <p>{children}</p>
  )
  Layout.Content = ({ children }: { children: ReactNode }) => <>{children}</>
  return { SectionPageLayout: Layout }
})

function renderUserHome() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return render(
    <QueryClientProvider client={client}>
      <UserHome />
    </QueryClientProvider>
  )
}

const HOME_DATA: UserHomeData = {
  account: {
    balance_quota: 6000000,
    used_quota: 1000,
    topups_count: 2,
  },
  active_plan: {
    title: 'PLUS',
    status: 'active',
    end_time: 1782766400,
  },
}

describe('User Home dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetUserHome.mockResolvedValue(HOME_DATA)
  })

  it('renders the Balance and Plan cards from user home data', async () => {
    renderUserHome()

    expect(
      await screen.findByRole('heading', { name: 'Home' })
    ).toBeInTheDocument()
    expect(await screen.findByText('Balance')).toBeInTheDocument()
    expect(screen.getByText('Plan')).toBeInTheDocument()
    expect(screen.getByText('PLUS')).toBeInTheDocument()
    expect(screen.getByText('2 top-ups total')).toBeInTheDocument()
  })

  it('shows "No active plan" when the user has no active subscription', async () => {
    mockGetUserHome.mockResolvedValue({
      ...HOME_DATA,
      active_plan: null,
    })
    renderUserHome()

    expect(await screen.findByText('No active plan')).toBeInTheDocument()
  })
})
