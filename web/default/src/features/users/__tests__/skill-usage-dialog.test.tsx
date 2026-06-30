import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DataTableRowActions } from '../components/data-table-row-actions'
import { UserSkillUsageDialog } from '../components/dialogs/user-skill-usage-dialog'
import { canViewUserSkillUsageAction } from '../constants'
import type { User, UserSkillUsageResponse } from '../types'

const { mockGetUserSkillUsage, mockUseUsers, authState } = vi.hoisted(() => ({
  mockGetUserSkillUsage: vi.fn(),
  mockUseUsers: vi.fn(),
  authState: {
    role: 100,
  },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, string>) =>
      values
        ? key.replace(/\{\{(\w+)\}\}/g, (_, name) => values[name] ?? '')
        : key,
  }),
}))

vi.mock('../api', () => ({
  getUserSkillUsage: mockGetUserSkillUsage,
  manageUser: vi.fn(),
  resetUserPasskey: vi.fn(),
  resetUserTwoFA: vi.fn(),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: <T,>(selector: (state: unknown) => T) =>
    selector({
      auth: {
        user: { id: 99, username: 'root', role: authState.role },
      },
    }),
}))

vi.mock('../components/users-provider', () => ({
  useUsers: () => mockUseUsers(),
}))

vi.mock('../components/dialogs/user-binding-dialog', () => ({
  UserBindingDialog: () => null,
}))

vi.mock(
  '@/features/subscriptions/components/dialogs/user-subscriptions-dialog',
  () => ({
    UserSubscriptionsDialog: () => null,
  })
)

const baseUser: User = {
  id: 42,
  username: 'demo-user',
  display_name: 'Demo User',
  quota: 1000,
  used_quota: 0,
  request_count: 0,
  group: 'default',
  status: 1,
  role: 1,
}

const consentedUsage: UserSkillUsageResponse = {
  user_id: 42,
  consent_granted: true,
  kids_protected: false,
  downloads: [
    {
      skill_id: 'skill-1',
      skill_slug: 'polished-writer',
      skill_name: 'Polished Writer',
      enabled: true,
      enabled_at: '2026-06-30T01:00:00Z',
      source: 'skill_package',
      last_update_time: '2026-06-30T02:00:00Z',
      input_tokens: 1200,
      output_tokens: 450,
      total_tokens: 1650,
      cost_usd: 0.0123,
    },
  ],
  usage_timeline: [
    {
      event_id: 'event-1',
      event_type: 'skill_used',
      occurred_at: '2026-06-30T02:00:00Z',
      skill_id: 'skill-1',
      skill_slug: 'polished-writer',
      skill_name: 'Polished Writer',
      model: 'smart-tier',
      input_tokens: 1200,
      output_tokens: 450,
      total_tokens: 1650,
      cost_usd: 0.0123,
      success: true,
    },
  ],
}

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}

beforeEach(() => {
  vi.clearAllMocks()
  authState.role = 100
  mockUseUsers.mockReturnValue({
    setOpen: vi.fn(),
    setCurrentRow: vi.fn(),
    triggerRefresh: vi.fn(),
  })
})

describe('DR-108 user Skill usage dialog', () => {
  it('renders consented per-Skill totals and usage timeline', async () => {
    mockGetUserSkillUsage.mockResolvedValue({
      success: true,
      data: consentedUsage,
    })

    renderWithQuery(
      <UserSkillUsageDialog open onOpenChange={vi.fn()} user={baseUser} />
    )

    expect((await screen.findAllByText('Polished Writer')).length).toBe(2)
    expect(screen.getAllByText('polished-writer').length).toBe(2)
    expect(screen.getAllByText('1,650').length).toBeGreaterThan(0)
    expect(screen.getAllByText('$0.0123').length).toBeGreaterThan(0)
    expect(screen.getByText('smart-tier')).toBeInTheDocument()
    expect(screen.getByText('Success')).toBeInTheDocument()
  })

  it('shows a privacy state when telemetry consent is missing', async () => {
    mockGetUserSkillUsage.mockResolvedValue({
      success: true,
      data: {
        ...consentedUsage,
        consent_granted: false,
        downloads: [],
        usage_timeline: [],
      },
    })

    renderWithQuery(
      <UserSkillUsageDialog open onOpenChange={vi.fn()} user={baseUser} />
    )

    expect(
      await screen.findByText('Skill usage unavailable')
    ).toBeInTheDocument()
    expect(screen.queryByText('Polished Writer')).not.toBeInTheDocument()
  })

  it('shows an API error state', async () => {
    mockGetUserSkillUsage.mockRejectedValue(new Error('Forbidden'))

    renderWithQuery(
      <UserSkillUsageDialog open onOpenChange={vi.fn()} user={baseUser} />
    )

    expect(
      await screen.findByText('Failed to load Skill usage')
    ).toBeInTheDocument()
    expect(
      screen.getByText('Please try again or check admin access.')
    ).toBeInTheDocument()
  })
})

describe('DR-108 user row Skill usage action', () => {
  it('allows root users to see the Skill usage action', () => {
    expect(canViewUserSkillUsageAction(100)).toBe(true)
  })

  it('hides the Skill usage action for non-root admins', () => {
    expect(canViewUserSkillUsageAction(10)).toBe(false)
  })

  it('renders the Skill usage menu item for root users', async () => {
    authState.role = 100

    render(<DataTableRowActions row={{ original: baseUser } as never} />)
    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))

    expect(await screen.findByText('Skill usage')).toBeInTheDocument()
  })

  it('does not render the Skill usage menu item for non-root admins', async () => {
    authState.role = 10

    render(<DataTableRowActions row={{ original: baseUser } as never} />)
    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))

    expect(await screen.findByText('Edit')).toBeInTheDocument()
    expect(screen.queryByText('Skill usage')).not.toBeInTheDocument()
  })
})
