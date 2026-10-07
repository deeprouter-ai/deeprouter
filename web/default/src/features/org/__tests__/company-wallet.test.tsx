/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: Enterprise Org P7 (meta-repo docs/enterprise-org-prd.md §4). The
// keys of an organization's members spend the company wallet — the owner's
// balance. So everywhere the console shows an account its balance, offers a
// top-up or nudges about credit, a member who is not the owner gets none of
// it, and is told once, where the balance would be, that the organization
// pays. A personal account and the owner keep every one of those things.
import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { formatQuota } from '@/lib/format'
import { BalanceWidget } from '@/components/balance-widget'
import { MobileDrawer } from '@/components/layout/components/mobile-drawer'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { OnboardingStatusBanner } from '@/features/dashboard/components/overview/onboarding-status-banner'
import { SummaryCards } from '@/features/dashboard/components/overview/summary-cards'
import { TopupNudge } from '@/features/dashboard/components/topup-nudge'
import { ProfileHeader } from '@/features/profile/components/profile-header'
import type { UserProfile } from '@/features/profile/types'
import { SimpleHome } from '@/features/simple/pages/home'
import { SimpleMe } from '@/features/simple/pages/me'
import { UserHome } from '@/features/user-home'
import { Wallet } from '@/features/wallet'
import type { OrgMembership } from '../types'
import { membershipOf } from './fixtures'

/** A balance small enough for every "running low" nudge to fire. */
const QUOTA = 31415
const BALANCE = formatQuota(QUOTA)

type Account = {
  user: Record<string, unknown>
  /** `undefined`: the question has not been answered yet. */
  membership: OrgMembership | null | undefined
}

const mocks = vi.hoisted(() => ({
  account: { user: {}, membership: null } as {
    user: Record<string, unknown>
    membership: unknown
  },
  getSelf: vi.fn(),
  navigate: vi.fn(),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: mocks.account.user, setUser: vi.fn() } }),
}))
vi.mock('@/features/org/hooks/use-org-membership', () => ({
  useOrgMembership: () => ({ data: mocks.account.membership }),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, vars?: Record<string, unknown>) =>
      vars
        ? key.replace(/{{(\w+)}}/g, (_, name) => String(vars[name] ?? ''))
        : key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children }: { to?: string; children?: ReactNode }) => (
    <a href={to}>{children}</a>
  ),
  useNavigate: () => mocks.navigate,
}))
vi.mock('@tanstack/react-query', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-query')>()),
  // The personal home page asks for the account's balance and plan; the
  // dashboard asks for a day of usage. Neither is what is under test.
  useQuery: (options: { queryKey: unknown[] }) =>
    options.queryKey[0] === 'user-home'
      ? {
          isError: false,
          isLoading: false,
          data: {
            account: { balance_quota: 31415, used_quota: 400, topups_count: 2 },
            active_plan: null,
          },
        }
      : { data: undefined },
}))
vi.mock('@/lib/api', () => ({ getSelf: mocks.getSelf }))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: {}, loading: false }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/components/sign-out-dialog', () => ({ SignOutDialog: () => null }))
vi.mock('@/features/profile/lib/persist-persona', () => ({
  persistPersona: vi.fn(),
}))
vi.mock('@/features/profile/api', () => ({ updateUserLanguage: vi.fn() }))
vi.mock('@/features/simple/components/topup-sheet', () => ({
  TopupSheet: (props: { open: boolean }) => (
    <div data-testid='topup-sheet'>{props.open ? 'open' : 'closed'}</div>
  ),
}))
vi.mock('@/features/dashboard/components/ui/stat-card', () => ({
  StatCard: (props: { title: string; value: string }) => (
    <div data-testid='stat-card'>
      {props.title}: {props.value}
    </div>
  ),
}))
vi.mock('@/components/layout', () => {
  const Part = ({ children }: { children?: ReactNode }) => <div>{children}</div>
  return {
    SectionPageLayout: Object.assign(Part, {
      Title: ({ children }: { children?: ReactNode }) => <h1>{children}</h1>,
      Description: Part,
      Content: Part,
      Actions: Part,
    }),
  }
})
// The wallet page proper: its parts are stood in for, so that what is asked
// is only whether the page is the wallet or the notice.
vi.mock('@/features/wallet/hooks', () => ({
  useTopupInfo: () => ({ topupInfo: null }),
  useAffiliate: () => ({
    affiliateLink: '',
    loading: false,
    transferQuota: vi.fn(),
    transferring: false,
  }),
}))
vi.mock('@/features/wallet/components/wallet-stats-card', () => ({
  WalletStatsCard: () => <div>wallet balance and stats</div>,
}))
vi.mock('@/features/wallet/components/recharge-panel', () => ({
  RechargePanel: () => <div>recharge panel</div>,
}))
vi.mock('@/features/wallet/components/subscription-plans-card', () => ({
  SubscriptionPlansCard: () => null,
}))
vi.mock('@/features/wallet/components/affiliate-rewards-card', () => ({
  AffiliateRewardsCard: () => null,
}))
vi.mock('@/features/wallet/components/auto-topup-card', () => ({
  AutoTopupCard: () => null,
}))
vi.mock(
  '@/features/wallet/components/dialogs/auto-topup-prompt-dialog',
  () => ({ AutoTopupPromptDialog: () => null })
)
vi.mock('@/features/wallet/components/dialogs/billing-history-dialog', () => ({
  BillingHistoryDialog: () => null,
}))
vi.mock('@/features/wallet/components/dialogs/transfer-dialog', () => ({
  TransferDialog: () => null,
}))

/** The signed-in user every account below is a variation of. */
const USER = {
  id: 7,
  username: 'someone',
  display_name: 'Someone',
  role: 1,
  quota: QUOTA,
  used_quota: 400,
  request_count: 3,
  setting: { persona: 'dev' },
}

const personal: Account = { user: USER, membership: null }
const owner: Account = {
  user: { ...USER, org_id: 1 },
  membership: membershipOf('owner'),
}
/** Every kind of member who is not the owner. */
const members: [string, Account][] = (
  ['admin', 'manager', 'staff', 'readonly'] as const
).map((role) => [
  role,
  { user: { ...USER, org_id: 1 }, membership: membershipOf(role) },
])
/** A member whose place in the organization has not been answered yet. */
const unanswered: Account = {
  user: { ...USER, org_id: 1 },
  membership: undefined,
}
/** The accounts that spend their own balance. */
const payers: [string, Account][] = [
  ['a personal account', personal],
  ['the owner', owner],
]

/** Signs an account in for what is rendered next. */
function signIn(account: Account) {
  mocks.account = account
}

const NOTICE = 'Paid by your organization'

beforeEach(() => {
  vi.clearAllMocks()
  mocks.getSelf.mockResolvedValue({ success: true, data: null })
  window.localStorage.clear()
  signIn(personal)
})

describe('the balance in the top bar', () => {
  it.each(payers)('is there for %s, and leads to the wallet', (_, account) => {
    signIn(account)
    render(<BalanceWidget />)
    expect(screen.getByRole('link', { name: BALANCE })).toHaveAttribute(
      'href',
      '/wallet'
    )
  })

  it.each([...members, ['a member not yet placed', unanswered] as const])(
    'is not there for %s',
    (_, account) => {
      signIn(account)
      const { container } = render(<BalanceWidget />)
      expect(container).toBeEmptyDOMElement()
    }
  )
})

describe('the nudges about credit on the dashboard', () => {
  it.each(payers)('tell %s that the balance is low', (_, account) => {
    signIn(account)
    render(
      <>
        <TopupNudge />
        <OnboardingStatusBanner />
      </>
    )
    expect(
      screen.getByText('Your balance is low — top up to get started')
    ).toBeInTheDocument()
    expect(screen.getByText('Trial credit running low.')).toBeInTheDocument()
  })

  it('tell a personal account that has not called yet about its unused credit', () => {
    signIn({ ...personal, user: { ...USER, request_count: 0 } })
    render(<OnboardingStatusBanner />)
    expect(
      screen.getByText("You haven't called the API yet.")
    ).toBeInTheDocument()
  })

  it.each(members)('have nothing to tell a member (%s)', (_, account) => {
    for (const requests of [0, 3]) {
      signIn({ ...account, user: { ...account.user, request_count: requests } })
      const { container, unmount } = render(
        <>
          <TopupNudge />
          <OnboardingStatusBanner />
        </>
      )
      expect(container).toBeEmptyDOMElement()
      unmount()
    }
  })
})

describe('the Simple console', () => {
  it.each(payers)(
    'opens on the balance and a way to add credit for %s',
    (_, account) => {
      signIn(account)
      render(<SimpleHome />)
      expect(screen.getByText('Balance')).toBeInTheDocument()
      expect(screen.getByText(BALANCE)).toBeInTheDocument()
      expect(
        screen.getByRole('button', { name: 'Add credit' })
      ).toBeInTheDocument()
      expect(screen.getByTestId('topup-sheet')).toHaveTextContent('closed')
      expect(screen.queryByText(NOTICE)).toBeNull()
    }
  )

  it.each(members)(
    'opens on "paid by your organization" for a member (%s)',
    (_, account) => {
      signIn(account)
      render(<SimpleHome />)
      expect(screen.getByText(NOTICE)).toBeInTheDocument()
      expect(screen.queryByText('Balance')).toBeNull()
      expect(screen.queryByText(BALANCE)).toBeNull()
      expect(screen.queryByRole('button', { name: 'Add credit' })).toBeNull()
      // What the console is for is still all there.
      expect(screen.getByText('What do you want to do?')).toBeInTheDocument()
    }
  )

  it('does not open the top-up sheet for a member who follows a link to it', () => {
    signIn(members[2][1])
    render(<SimpleHome openTopup />)
    expect(screen.queryByTestId('topup-sheet')).toBeNull()

    signIn(personal)
    render(<SimpleHome openTopup />)
    expect(screen.getByTestId('topup-sheet')).toHaveTextContent('open')
  })

  it('shows neither a balance nor the notice until a member’s place is known', () => {
    signIn(unanswered)
    render(<SimpleHome />)
    expect(screen.queryByText(BALANCE)).toBeNull()
    expect(screen.queryByText(NOTICE)).toBeNull()
    expect(screen.queryByRole('button', { name: 'Add credit' })).toBeNull()
    expect(screen.getByText('What do you want to do?')).toBeInTheDocument()
  })

  it('offers "Add credit" under Me to whoever pays, and not to a member', () => {
    for (const [, account] of payers) {
      signIn(account)
      const { unmount } = render(<SimpleMe />)
      expect(
        screen.getByRole('button', { name: 'Add credit' })
      ).toBeInTheDocument()
      unmount()
    }
    for (const [, account] of members) {
      signIn(account)
      const { unmount } = render(<SimpleMe />)
      expect(screen.queryByRole('button', { name: 'Add credit' })).toBeNull()
      expect(
        screen.getByRole('button', { name: /Switch to professional mode/ })
      ).toBeInTheDocument()
      unmount()
    }
  })
})

describe('the ways to the wallet', () => {
  /** Opens the profile menu and returns the names of its entries. */
  async function profileMenu() {
    render(<ProfileDropdown />)
    await userEvent.click(screen.getByRole('button'))
    return (await screen.findAllByRole('menuitem')).map(
      (item) => item.textContent
    )
  }

  it('include the profile menu for whoever pays', async () => {
    signIn(owner)
    expect(await profileMenu()).toContain('Wallet')
  })

  it('do not include the profile menu for a member', async () => {
    signIn(members[0][1])
    const entries = await profileMenu()
    expect(entries).not.toContain('Wallet')
    expect(entries).toContain('Profile')
  })

  /** Renders the mobile drawer open for the signed-in user. */
  function drawer() {
    return render(
      <MobileDrawer
        isOpen
        onClose={vi.fn()}
        homeUrl='/'
        displayLogo={null}
        displaySiteName='DeepRouter'
        loading={false}
        logoLoaded
        mobileLinksList={[]}
        showAuthButtons
        user={mocks.account.user as never}
      />
    )
  }

  it('include the mobile drawer for whoever pays, and not for a member', () => {
    signIn(personal)
    const { unmount } = drawer()
    expect(screen.getByRole('link', { name: 'Wallet' })).toHaveAttribute(
      'href',
      '/wallet'
    )
    unmount()

    signIn(members[2][1])
    drawer()
    expect(screen.queryByRole('link', { name: 'Wallet' })).toBeNull()
    expect(screen.getAllByRole('link', { name: 'Profile' })).not.toHaveLength(0)
  })
})

describe('the wallet page, where every "top up" link ends', () => {
  it.each(payers)('is the wallet for %s', (_, account) => {
    signIn(account)
    render(<Wallet />)
    expect(screen.getByText('wallet balance and stats')).toBeInTheDocument()
    expect(screen.queryByText(NOTICE)).toBeNull()
    expect(mocks.getSelf).toHaveBeenCalled()
  })

  it.each(members)(
    'tells a member that the organization pays (%s)',
    (_, account) => {
      signIn(account)
      render(<Wallet />)
      expect(screen.getByText(NOTICE)).toBeInTheDocument()
      expect(screen.queryByText('wallet balance and stats')).toBeNull()
      expect(screen.queryByText('recharge panel')).toBeNull()
      // Nothing of the wallet was even loaded.
      expect(mocks.getSelf).not.toHaveBeenCalled()
    }
  )

  it('is blank until a member’s place is known', () => {
    signIn(unanswered)
    const { container } = render(<Wallet />)
    expect(container).toBeEmptyDOMElement()
    expect(mocks.getSelf).not.toHaveBeenCalled()
  })
})

describe('the dashboard’s usage summary', () => {
  it.each(payers)(
    'shows %s what they have, with a way to recharge',
    (_, account) => {
      signIn(account)
      render(<SummaryCards />)
      expect(
        screen.getAllByTestId('stat-card').map((card) => card.textContent)
      ).toEqual([
        `Current Balance: ${BALANCE}`,
        expect.stringContaining('Historical Usage'),
        expect.stringContaining('Request Count'),
      ])
      expect(screen.getByText('Credit remaining')).toBeInTheDocument()
      expect(screen.getByRole('link', { name: 'Recharge' })).toHaveAttribute(
        'href',
        '/wallet'
      )
      expect(screen.queryByText(NOTICE)).toBeNull()
    }
  )

  it.each(members)(
    'shows a member what they used, and that the organization pays (%s)',
    (_, account) => {
      signIn(account)
      render(<SummaryCards />)
      expect(
        screen.getAllByTestId('stat-card').map((card) => card.textContent)
      ).toEqual([
        expect.stringContaining('Historical Usage'),
        expect.stringContaining('Request Count'),
      ])
      expect(screen.queryByText(BALANCE)).toBeNull()
      expect(screen.queryByText('Credit remaining')).toBeNull()
      expect(screen.queryByRole('link', { name: 'Recharge' })).toBeNull()
      expect(screen.getByText(NOTICE)).toBeInTheDocument()
      expect(
        screen.getByText('What you used and how many calls you made.')
      ).toBeInTheDocument()
    }
  )
})

describe('the profile header', () => {
  const profile: UserProfile = {
    ...USER,
    setting: '{"persona":"dev"}',
    group: 'default',
    status: 1,
    aff_count: 0,
    aff_quota: 0,
    aff_history_quota: 0,
    created_time: 1700000000,
  }

  it.each(payers)('shows %s their balance', (_, account) => {
    signIn(account)
    render(<ProfileHeader profile={profile} loading={false} />)
    expect(screen.getByText('Current Balance')).toBeInTheDocument()
    expect(screen.getByText(BALANCE)).toBeInTheDocument()
    expect(screen.getByText('Total Usage')).toBeInTheDocument()
  })

  it.each(members)(
    'shows a member their usage without it (%s)',
    (_, account) => {
      signIn(account)
      render(<ProfileHeader profile={profile} loading={false} />)
      expect(screen.queryByText('Current Balance')).toBeNull()
      expect(screen.queryByText(BALANCE)).toBeNull()
      expect(screen.getByText('Total Usage')).toBeInTheDocument()
      expect(screen.getByText('API Requests')).toBeInTheDocument()
    }
  )
})

describe('the personal home page', () => {
  it.each(payers)('shows %s their balance and plan', (_, account) => {
    signIn(account)
    render(<UserHome />)
    expect(screen.getByText('Balance')).toBeInTheDocument()
    expect(screen.getByText(BALANCE)).toBeInTheDocument()
    expect(screen.getByText('Plan')).toBeInTheDocument()
  })

  it.each(members)(
    'tells a member that the organization pays (%s)',
    (_, account) => {
      signIn(account)
      render(<UserHome />)
      expect(screen.getByText(NOTICE)).toBeInTheDocument()
      expect(screen.queryByText('Balance')).toBeNull()
      expect(screen.queryByText(BALANCE)).toBeNull()
    }
  )

  it('is blank until a member’s place is known', () => {
    signIn(unanswered)
    const { container } = render(<UserHome />)
    expect(container).toBeEmptyDOMElement()
  })
})
