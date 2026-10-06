// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SignUpForm } from '../components/sign-up-form'

// Enterprise Org P2 (meta-repo docs/enterprise-org-prd.md): the sign-up form
// offers "create an organization for my company". Unticked, the request must
// be exactly the personal sign-up it always was.

const mockRegister = vi.hoisted(() => vi.fn())
const mockToastError = vi.hoisted(() => vi.fn())

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
  useAuthRedirect: () => ({ handleLoginSuccess: vi.fn() }),
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
  setWelcomeHandoff: vi.fn(),
  captureAcquisitionMeta: vi.fn(),
  readAcquisitionMeta: () => ({ channel: '', timezone: '' }),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
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
  mockRegister.mockReset().mockResolvedValue({ success: true, data: null })
  mockToastError.mockReset()
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
