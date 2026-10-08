// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SignUpForm } from '../components/sign-up-form'

// Enterprise Org (meta-repo docs/enterprise-org-prd.md): the sign-up form
// offers "create an organization for my company" (P2), and joins an existing
// one when the visitor arrived through an invite link (P3). With neither, the
// request must be exactly the personal sign-up it always was.

const mockRegister = vi.hoisted(() => vi.fn())
const mockToastError = vi.hoisted(() => vi.fn())
const mockHandleLoginSuccess = vi.hoisted(() => vi.fn())
const mockSetWelcomeHandoff = vi.hoisted(() => vi.fn())
const mockInvitePreview = vi.hoisted(() => vi.fn())

vi.mock('@/features/auth/api', () => ({
  register: mockRegister,
  wechatLoginByCode: vi.fn(),
}))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: {}, loading: false }),
}))
vi.mock('@/features/auth/hooks/use-turnstile', () => ({
  useTurnstile: () => ({
    isTurnstileEnabled: false,
    turnstileSiteKey: '',
    turnstileToken: '',
    setTurnstileToken: vi.fn(),
    validateTurnstile: () => true,
  }),
}))
vi.mock('@/features/auth/hooks/use-auth-redirect', () => ({
  useAuthRedirect: () => ({ handleLoginSuccess: mockHandleLoginSuccess }),
}))
vi.mock('@/features/org/hooks/use-org-invite-preview', () => ({
  useOrgInvitePreview: mockInvitePreview,
}))
vi.mock('@/features/auth/hooks/use-email-verification', () => ({
  useEmailVerification: () => ({
    isSending: false,
    secondsLeft: 0,
    isActive: false,
    sendCode: vi.fn(),
  }),
}))
vi.mock('@/features/auth/components/oauth-providers', () => ({
  OAuthProviders: () => <div data-testid='oauth-providers' />,
}))
vi.mock('@/features/auth/lib/storage', () => ({
  getAffiliateCode: () => '',
  setWelcomeHandoff: mockSetWelcomeHandoff,
  captureAcquisitionMeta: vi.fn(),
  readAcquisitionMeta: () => ({ channel: '', timezone: '' }),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    // The key with its {{placeholders}} filled in, so a test can read the
    // values a sentence was given.
    t: (key: string, values?: Record<string, unknown>) =>
      key.replace(/{{(\w+)}}/g, (_, name: string) =>
        String(values?.[name] ?? '')
      ),
  }),
}))
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: mockToastError },
}))

const ORG_OPTION = /Create an organization for my company/

async function fillAccount() {
  await userEvent.type(
    screen.getByPlaceholderText('Enter your username'),
    'founder'
  )
  await userEvent.type(
    screen.getByPlaceholderText('Enter password (8-20 characters)'),
    'password123'
  )
  await userEvent.type(
    screen.getByPlaceholderText('Confirm password'),
    'password123'
  )
}

function submit() {
  return userEvent.click(screen.getByRole('button', { name: 'Create account' }))
}

beforeEach(() => {
  mockRegister.mockReset().mockResolvedValue({ success: true, data: { id: 7 } })
  mockToastError.mockReset()
  mockHandleLoginSuccess.mockReset()
  mockSetWelcomeHandoff.mockReset()
  mockInvitePreview.mockReset().mockReturnValue({ status: 'none' })
})

describe('sign-up form: company option', () => {
  it('sends a personal sign-up when the option is left alone', async () => {
    render(<SignUpForm />)
    expect(screen.queryByLabelText('Organization name')).toBeNull()

    await fillAccount()
    await submit()

    await waitFor(() => expect(mockRegister).toHaveBeenCalledTimes(1))
    const payload = mockRegister.mock.calls[0][0]
    expect(payload.username).toBe('founder')
    // undefined is dropped from the JSON body, so the backend sees no org_name.
    expect(payload.org_name).toBeUndefined()
  })

  it('asks for the name once ticked and sends it trimmed', async () => {
    render(<SignUpForm />)
    await fillAccount()

    await userEvent.click(screen.getByRole('checkbox', { name: ORG_OPTION }))
    await userEvent.type(
      screen.getByLabelText('Organization name'),
      '  Acme Pty Ltd  '
    )
    await submit()

    await waitFor(() => expect(mockRegister).toHaveBeenCalledTimes(1))
    expect(mockRegister.mock.calls[0][0].org_name).toBe('Acme Pty Ltd')
  })

  it('refuses to submit a company sign-up without a name', async () => {
    render(<SignUpForm />)
    await fillAccount()

    await userEvent.click(screen.getByRole('checkbox', { name: ORG_OPTION }))
    await userEvent.type(screen.getByLabelText('Organization name'), '   ')
    await submit()

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        'Please enter your organization name'
      )
    )
    expect(mockRegister).not.toHaveBeenCalled()
  })

  it('does not send a name that was typed and then unticked', async () => {
    render(<SignUpForm />)
    await fillAccount()

    const option = screen.getByRole('checkbox', { name: ORG_OPTION })
    await userEvent.click(option)
    await userEvent.type(screen.getByLabelText('Organization name'), 'Acme')
    await userEvent.click(option)
    await submit()

    await waitFor(() => expect(mockRegister).toHaveBeenCalledTimes(1))
    expect(mockRegister.mock.calls[0][0].org_name).toBeUndefined()
  })

  it('hides third-party sign-up while ticked, since it cannot carry the name', async () => {
    render(<SignUpForm />)
    expect(screen.getByTestId('oauth-providers')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('checkbox', { name: ORG_OPTION }))
    expect(screen.queryByTestId('oauth-providers')).toBeNull()
  })
})

// Decision D24: whoever founds an organization goes straight to the Advanced
// console. /welcome preselects "Everyday use" and saves it on the way out, so
// a founder sent there would land in the Simple console, where there is no
// organization management.
describe('sign-up form: where a new account lands', () => {
  it('sends a founder to the Advanced console, past the welcome page', async () => {
    render(<SignUpForm />)
    await fillAccount()
    await userEvent.click(screen.getByRole('checkbox', { name: ORG_OPTION }))
    await userEvent.type(screen.getByLabelText('Organization name'), 'Acme')
    await submit()

    await waitFor(() => expect(mockHandleLoginSuccess).toHaveBeenCalledTimes(1))
    expect(mockHandleLoginSuccess).toHaveBeenCalledWith({ id: 7 }, '/dashboard')
    // Nothing is stashed for a welcome page the founder never opens.
    expect(mockSetWelcomeHandoff).not.toHaveBeenCalled()
  })

  it('still sends a personal sign-up to the welcome page', async () => {
    render(<SignUpForm />)
    await fillAccount()
    await submit()

    await waitFor(() => expect(mockHandleLoginSuccess).toHaveBeenCalledTimes(1))
    expect(mockHandleLoginSuccess).toHaveBeenCalledWith({ id: 7 }, '/welcome')
    expect(mockSetWelcomeHandoff).toHaveBeenCalledWith({ id: 7 })
  })
})

describe('sign-up form: arriving through an invite link', () => {
  const validInvite = {
    status: 'valid',
    preview: { org_name: 'Acme', role: 'manager', department: 'Sales' },
  }

  it('says where the link leads and joins that organization', async () => {
    mockInvitePreview.mockReturnValue(validInvite)
    render(<SignUpForm orgInvite='CODE123' />)

    expect(mockInvitePreview).toHaveBeenCalledWith('CODE123')
    expect(screen.getByRole('status')).toHaveTextContent("You're joining Acme")
    expect(screen.getByRole('status')).toHaveTextContent(
      'Department: Sales · Role: Manager'
    )

    await fillAccount()
    await submit()

    await waitFor(() => expect(mockRegister).toHaveBeenCalledTimes(1))
    const payload = mockRegister.mock.calls[0][0]
    expect(payload.org_invite).toBe('CODE123')
    expect(payload.org_name).toBeUndefined()
    // An invited member is not a founder: the welcome page asks them the
    // usual question.
    await waitFor(() =>
      expect(mockHandleLoginSuccess).toHaveBeenCalledWith({ id: 7 }, '/welcome')
    )
  })

  it('offers neither founding a company nor third-party sign-up', () => {
    mockInvitePreview.mockReturnValue(validInvite)
    render(<SignUpForm orgInvite='CODE123' />)

    expect(screen.queryByRole('checkbox', { name: ORG_OPTION })).toBeNull()
    // A third-party sign-up could not carry the invite code.
    expect(screen.queryByTestId('oauth-providers')).toBeNull()
  })

  it('blocks the form when the link is dead, rather than making a personal account', async () => {
    mockInvitePreview.mockReturnValue({ status: 'invalid' })
    render(<SignUpForm orgInvite='EXPIRED' />)

    expect(screen.getByRole('alert')).toHaveTextContent(
      'This invite link is invalid or has expired.'
    )
    expect(
      screen.getByRole('link', { name: 'Sign up without an invite' })
    ).toHaveAttribute('href', '/sign-up')
    const button = screen.getByRole('button', { name: 'Create account' })
    expect(button).toBeDisabled()

    await fillAccount()
    await userEvent.click(button)
    expect(mockRegister).not.toHaveBeenCalled()
  })

  it('waits for the link to be checked before it lets anyone submit', () => {
    mockInvitePreview.mockReturnValue({ status: 'loading' })
    render(<SignUpForm orgInvite='CODE123' />)

    expect(
      screen.getByRole('button', { name: 'Create account' })
    ).toBeDisabled()
    expect(screen.queryByRole('status')).toBeNull()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('does not send an invite code when there is no invite link', async () => {
    render(<SignUpForm />)
    await fillAccount()
    await submit()

    await waitFor(() => expect(mockRegister).toHaveBeenCalledTimes(1))
    expect(mockRegister.mock.calls[0][0].org_invite).toBeUndefined()
  })
})
