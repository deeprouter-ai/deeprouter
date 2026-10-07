/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: Enterprise Org P7 (meta-repo docs/enterprise-org-prd.md §4, D40,
// D41), the two places a new member meets before any other. The welcome page
// that follows sign-up tells a personal account about its trial credit, its
// key and how to check it works; none of that is true for someone who signed
// up through an invitation — their organization pays, hands them their keys,
// and a key's value is never shown to them — so a member gets an introduction
// of their own. And the one-tap key check runs on the playground endpoint,
// which is closed to organization accounts: nothing in the console sends them
// to it, and the page sends them back to their keys.
import type { ReactNode } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { formatQuota } from '@/lib/format'
import { ApiKeyIntegrationDialog } from '@/features/keys/components/api-key-integration-dialog'
import { ApiKeysPrimaryButtons } from '@/features/keys/components/api-keys-primary-buttons'
import { KeySelfCheckPage } from '@/features/keys/test'
import { SIMPLE_HOME } from '@/features/simple/lib/mode'
import { Welcome } from '@/features/welcome'
import type { OrgMembership } from '../types'
import { membershipOf } from './fixtures'

type Account = {
  user: Record<string, unknown>
  /** `undefined`: the question has not been answered. */
  membership: OrgMembership | null | undefined
  /** The question is still on its way to the backend. */
  asking?: boolean
}

const mocks = vi.hoisted(() => ({
  account: { user: {}, membership: null } as {
    user: Record<string, unknown>
    membership: unknown
    asking?: boolean
  },
  navigate: vi.fn(),
  updateUserSettings: vi.fn(),
  getUserModels: vi.fn(),
  sendChatCompletion: vi.fn(),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: mocks.account.user, setUser: vi.fn() } }),
}))
vi.mock('@/features/org/hooks/use-org-membership', () => ({
  useOrgMembership: () => ({
    data: mocks.account.membership,
    isLoading: Boolean(mocks.account.asking),
  }),
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
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
// What sign-up hands the welcome page: the trial credit of a new account.
vi.mock('@/features/auth/lib/storage', () => ({
  takeWelcomeHandoff: () => ({
    display_name: 'Someone',
    trial_quota: 250000,
    default_token: '',
  }),
}))
vi.mock('@/features/profile/api', () => ({
  updateUserSettings: mocks.updateUserSettings,
}))
vi.mock('@/features/playground/api', () => ({
  getUserModels: mocks.getUserModels,
  sendChatCompletion: mocks.sendChatCompletion,
}))
vi.mock('@/features/keys/components/api-keys-provider', () => ({
  useApiKeys: () => ({
    setOpen: vi.fn(),
    setCurrentRow: vi.fn(),
    setResolvedKey: vi.fn(),
  }),
}))
vi.mock('@/features/keys/components/key-model-discovery', () => ({
  KeyModelDiscovery: () => null,
}))

/** The signed-in user every account below is a variation of. */
const USER = {
  id: 7,
  username: 'someone',
  display_name: 'Someone',
  role: 1,
  quota: 250000,
  setting: {},
}

const personal: Account = { user: USER, membership: null }
/** Every kind of member who is not the owner. */
const members: [string, Account][] = (
  ['admin', 'manager', 'staff', 'readonly'] as const
).map((role) => [
  role,
  { user: { ...USER, org_id: 1 }, membership: membershipOf(role) },
])
/** Everyone in an organization, its owner included. */
const orgAccounts: [string, Account][] = [
  [
    'the owner',
    { user: { ...USER, org_id: 1 }, membership: membershipOf('owner') },
  ],
  ...members,
]
/** A member whose place in the organization could not be asked. */
const unanswered: Account = {
  user: { ...USER, org_id: 1 },
  membership: undefined,
}
/** The same member while the question is still on its way. */
const asking: Account = { ...unanswered, asking: true }

/** Signs an account in for what is rendered next. */
function signIn(account: Account) {
  mocks.account = account
}

const NOTICE = 'Paid by your organization'
const TRIAL = formatQuota(250000)
const FOOTNOTE =
  'Your free credit is enough to start — top up later when it runs out.'

beforeEach(() => {
  vi.clearAllMocks()
  mocks.updateUserSettings.mockResolvedValue({ success: true })
  mocks.getUserModels.mockResolvedValue([])
  window.sessionStorage.clear()
  signIn(personal)
})

describe('the welcome page after sign-up', () => {
  const KEYS_TITLE = 'Your keys come from your organization'
  const ASK_FOR_ONE =
    'You do not create keys yourself. Once an administrator hands you one, it shows up in your account. If you have none yet, ask an administrator of your organization for one.'
  const MAKE_ONE =
    'Your organization’s keys are created and handed out on the Organization keys page, in professional mode. You are allowed to do that, and can make one out to yourself there.'
  const CHECK_STEP =
    'Come back and check it works — one tap confirms your key and credit are live.'
  /** What the page tells a personal account and must never tell a member. */
  const PERSONAL_ONLY = [
    'Free trial credit',
    TRIAL,
    FOOTNOTE,
    'Your key (API Key)',
    'Ready — view it on the Keys page →',
    'Copy your key above.',
    CHECK_STEP,
    "Everything's set up. Here's what you got — and how to start using it.",
  ]

  /** The numbered steps on the page, top to bottom. */
  const steps = () =>
    screen.getAllByRole('listitem').map((item) => item.textContent)

  /** The three answers to "what will you mostly use it for". */
  const choice = (name: 'Everyday use' | 'Building / coding' | 'Team') =>
    screen.getByRole('button', { name: new RegExp('^' + name) })
  /** Which of them is chosen on the rendered page. */
  const chosen = () =>
    (['Everyday use', 'Building / coding', 'Team'] as const).filter(
      (name) => choice(name).getAttribute('aria-pressed') === 'true'
    )

  /** Picks "I set things up myself" on the rendered page. */
  const byHand = () => userEvent.click(choice('Building / coding'))

  /** Presses the main button, named `button`, and returns where it leads. */
  async function start(button: RegExp) {
    await userEvent.click(screen.getByRole('button', { name: button }))
    await waitFor(() => expect(mocks.navigate).toHaveBeenCalled())
    return mocks.navigate.mock.calls[0][0]
  }

  describe('for a personal account', () => {
    it('says what the account was given and how to add more', () => {
      render(<Welcome />)
      expect(
        screen.getByRole('heading', { name: /Your account is ready, Someone/ })
      ).toBeInTheDocument()
      expect(screen.getByText('Free trial credit')).toBeInTheDocument()
      expect(screen.getByText(TRIAL)).toBeInTheDocument()
      expect(
        screen.getByRole('button', { name: /Add credit/ })
      ).toBeInTheDocument()
      expect(screen.getByText(FOOTNOTE)).toBeInTheDocument()
      // Nothing of the member's page.
      expect(screen.queryByText(NOTICE)).toBeNull()
      expect(screen.queryByText(KEYS_TITLE)).toBeNull()
    })

    it('opens on the Simple console — "Everyday use" chosen, its three steps — and leads there', async () => {
      render(<Welcome />)
      expect(chosen()).toEqual(['Everyday use'])
      expect(screen.getByText('Start using it in 3 steps')).toBeInTheDocument()
      expect(steps()).toHaveLength(3)
      expect(await start(/Get started/)).toEqual({
        to: SIMPLE_HOME,
        search: {},
        replace: true,
      })
      expect(mocks.updateUserSettings).toHaveBeenCalledWith(
        expect.objectContaining({ persona: 'casual' })
      )
    })

    it('tells whoever sets up by hand to copy the key and come back to check it, and leads to the check', async () => {
      render(<Welcome />)
      await byHand()
      expect(screen.getByText('Start using it in 3 steps')).toBeInTheDocument()
      expect(steps()).toEqual([
        expect.stringContaining('Copy your key above.'),
        expect.stringContaining('Paste it into the AI tool you already use'),
        expect.stringContaining(CHECK_STEP),
      ])
      expect(screen.getByText('Your key (API Key)')).toBeInTheDocument()
      expect(await start(/Check it works/)).toEqual({
        to: '/keys/test',
        replace: true,
      })
    })

    it('is what the owner of an organization gets too: the balance is theirs', () => {
      signIn(orgAccounts[0][1])
      render(<Welcome />)
      expect(
        screen.getByRole('button', { name: /Add credit/ })
      ).toBeInTheDocument()
      expect(screen.queryByText(NOTICE)).toBeNull()
    })

    it('does not wait on a question about organizations', () => {
      signIn({ ...personal, membership: undefined, asking: true })
      render(<Welcome />)
      expect(screen.getByText('Free trial credit')).toBeInTheDocument()
    })
  })

  describe('for a member of an organization', () => {
    it.each(members)(
      'welcomes them into the organization and says who pays and where keys come from (%s)',
      (_, account) => {
        signIn(account)
        render(<Welcome />)
        expect(
          screen.getByRole('heading', { name: /You’ve joined Acme, Someone/ })
        ).toBeInTheDocument()
        expect(
          screen.getByText(
            'Your account is ready. Two things to know, then how to start.'
          )
        ).toBeInTheDocument()
        expect(screen.getByText(NOTICE)).toBeInTheDocument()
        expect(screen.getByText(KEYS_TITLE)).toBeInTheDocument()
        // The question every new account is asked is still asked.
        expect(
          screen.getByRole('button', { name: /Everyday use/ })
        ).toBeInTheDocument()
      }
    )

    it.each(members)(
      'tells them nothing that is only true of a personal account (%s)',
      async (_, account) => {
        signIn(account)
        render(<Welcome />)
        const absent = () => {
          for (const text of PERSONAL_ONLY) {
            expect(screen.queryByText(text)).toBeNull()
          }
          expect(
            screen.queryByRole('button', { name: /Add credit/ })
          ).toBeNull()
          expect(
            screen.queryByRole('button', { name: /Check it works/ })
          ).toBeNull()
        }
        absent()
        await byHand()
        absent()
        await userEvent.click(choice('Everyday use'))
        absent()
      }
    )

    it.each([
      members[1],
      members[2],
      members[3],
      ['a member whose place could not be asked', unanswered] as const,
    ])('tells %s, who is handed their keys, whom to ask', (_, account) => {
      signIn(account)
      render(<Welcome />)
      expect(screen.getByText(ASK_FOR_ONE)).toBeInTheDocument()
      expect(screen.queryByText(MAKE_ONE)).toBeNull()
    })

    it.each([
      members[0],
      [
        'a role of the organization’s own that may create keys',
        {
          user: { ...USER, org_id: 1 },
          membership: membershipOf('staff', {
            permissions: ['key.read', 'key.create'],
          }),
        },
      ] as const,
    ])('tells %s, who may create keys, where to make one', (_, account) => {
      signIn(account)
      render(<Welcome />)
      expect(screen.getByText(MAKE_ONE)).toBeInTheDocument()
      expect(screen.queryByText(ASK_FOR_ONE)).toBeNull()
    })

    /** The two steps of the professional console: the keys page, one-click setup. */
    const TWO_STEPS = [
      expect.stringContaining(
        'Open the API keys page — the keys your organization handed you are listed there.'
      ),
      expect.stringContaining(
        'Use “One-click setup” on that page to install a key into the tool you use. There is nothing to copy: a key’s value is never shown.'
      ),
    ]

    it.each(members)(
      'opens on the professional console — "Team" chosen, its two steps — and leads to their keys (%s)',
      async (_, account) => {
        signIn(account)
        render(<Welcome />)
        expect(chosen()).toEqual(['Team'])
        expect(
          screen.getByText('Start using it in 2 steps')
        ).toBeInTheDocument()
        expect(screen.queryByText('Start using it in 3 steps')).toBeNull()
        expect(steps()).toEqual(TWO_STEPS)
        expect(await start(/Get started/)).toEqual({
          to: '/keys',
          replace: true,
        })
        expect(mocks.updateUserSettings).toHaveBeenCalledWith(
          expect.objectContaining({ persona: 'team' })
        )
      }
    )

    it('gives whoever picks "Building / coding" the same two steps, and leads to their keys', async () => {
      signIn(members[2][1])
      render(<Welcome />)
      await byHand()
      expect(chosen()).toEqual(['Building / coding'])
      expect(steps()).toEqual(TWO_STEPS)
      expect(await start(/Get started/)).toEqual({ to: '/keys', replace: true })
      expect(mocks.updateUserSettings).toHaveBeenCalledWith(
        expect.objectContaining({ persona: 'dev' })
      )
    })

    it('gives whoever picks "Everyday use" the three steps of the Simple console, and leads there', async () => {
      signIn(members[2][1])
      render(<Welcome />)
      await userEvent.click(choice('Everyday use'))
      expect(chosen()).toEqual(['Everyday use'])
      expect(screen.getByText('Start using it in 3 steps')).toBeInTheDocument()
      expect(steps()).toEqual([
        expect.stringContaining('Pick what you want to make'),
        expect.stringContaining('Tap “Copy for my AI”.'),
        expect.stringContaining('Paste it into Claude Code or Codex'),
      ])
      expect(await start(/Get started/)).toEqual({
        to: SIMPLE_HOME,
        search: {},
        replace: true,
      })
      expect(mocks.updateUserSettings).toHaveBeenCalledWith(
        expect.objectContaining({ persona: 'casual' })
      )
    })

    it('shows nothing while their place in the organization is being asked', () => {
      signIn(asking)
      const { container } = render(<Welcome />)
      expect(container).toBeEmptyDOMElement()
      expect(mocks.navigate).not.toHaveBeenCalled()
    })

    it('still lets them in when their place could not be asked, and shows no credit', async () => {
      signIn(unanswered)
      render(<Welcome />)
      // The organization's name is not known, so the heading does without it.
      expect(
        screen.getByRole('heading', { name: /Your account is ready, Someone/ })
      ).toBeInTheDocument()
      expect(screen.getByText(NOTICE)).toBeInTheDocument()
      expect(screen.getByText(KEYS_TITLE)).toBeInTheDocument()
      for (const text of PERSONAL_ONLY) {
        expect(screen.queryByText(text)).toBeNull()
      }
      expect(screen.queryByRole('button', { name: /Add credit/ })).toBeNull()
      // Like any member, they start in the professional console.
      expect(chosen()).toEqual(['Team'])
      expect(await start(/Get started/)).toEqual({ to: '/keys', replace: true })
    })
  })
})

describe('the one-tap key check', () => {
  it('is offered to a personal account next to its keys', () => {
    render(<ApiKeysPrimaryButtons />)
    expect(screen.getByRole('link', { name: 'Test a key' })).toHaveAttribute(
      'href',
      '/keys/test'
    )
    expect(
      screen.getByRole('button', { name: 'Create API Key' })
    ).toBeInTheDocument()
  })

  it.each(orgAccounts)(
    'is not offered to %s, who keeps the setup guide',
    (_, account) => {
      signIn(account)
      render(<ApiKeysPrimaryButtons />)
      expect(screen.queryByRole('link', { name: 'Test a key' })).toBeNull()
      expect(
        screen.getByRole('button', { name: 'Setup guide' })
      ).toBeInTheDocument()
    }
  )

  /** Opens the setup guide and walks to its last step. */
  async function lastStepOfTheGuide() {
    render(<ApiKeyIntegrationDialog open onClose={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(await screen.findByText('What you should see')).toBeInTheDocument()
  }

  it('closes the setup guide of a personal account', async () => {
    await lastStepOfTheGuide()
    expect(
      screen.getByRole('link', { name: 'Test this key →' })
    ).toHaveAttribute('href', '/keys/test')
  })

  it.each(orgAccounts)(
    'does not close the setup guide of %s',
    async (_, account) => {
      signIn(account)
      await lastStepOfTheGuide()
      expect(screen.queryByRole('link', { name: 'Test this key →' })).toBeNull()
      expect(
        screen.queryByText('Not a developer? Run a one-click test instead:')
      ).toBeNull()
    }
  )

  it('opens for a personal account', async () => {
    render(<KeySelfCheckPage />)
    expect(screen.getByText('Test your API key')).toBeInTheDocument()
    await waitFor(() => expect(mocks.getUserModels).toHaveBeenCalled())
    expect(mocks.navigate).not.toHaveBeenCalled()
  })

  it.each([...orgAccounts, ['a member not yet placed', unanswered] as const])(
    'sends %s back to their keys, showing and asking nothing',
    (_, account) => {
      signIn(account)
      const { container } = render(<KeySelfCheckPage />)
      expect(container).toBeEmptyDOMElement()
      expect(mocks.navigate).toHaveBeenCalledWith({
        to: '/keys',
        replace: true,
      })
      expect(mocks.getUserModels).not.toHaveBeenCalled()
    }
  )

  it('sends a member back even when their profile does not name the organization', () => {
    signIn({ user: USER, membership: membershipOf('staff') })
    const { container } = render(<KeySelfCheckPage />)
    expect(container).toBeEmptyDOMElement()
    expect(mocks.navigate).toHaveBeenCalledWith({ to: '/keys', replace: true })
  })
})
