// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ProfileHeader } from '../components/profile-header'
import type { UserProfile } from '../types'

// Enterprise Org D48: a member changes their own display name from the
// profile page. These pin what the dialog asks the backend for — the name
// alone, so that no password is needed — and what it does with the answer.

const mocks = vi.hoisted(() => ({
  updateUserProfile: vi.fn(),
  setUser: vi.fn(),
  user: { id: 7, username: 'wang', display_name: 'Wang' } as Record<
    string,
    unknown
  >,
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  updateUserProfile: mocks.updateUserProfile,
}))
vi.mock('sonner', () => ({ toast: mocks.toast }))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: Object.assign(
    (selector: (state: unknown) => unknown) =>
      selector({ auth: { user: mocks.user, setUser: mocks.setUser } }),
    {
      getState: () => ({
        auth: { user: mocks.user, setUser: mocks.setUser },
      }),
    }
  ),
}))
vi.mock('@/features/org/hooks/use-wallet-view', () => ({
  useWalletView: () => 'company',
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, vars?: Record<string, unknown>) =>
      vars
        ? key.replace(/{{(\w+)}}/g, (_, name) => String(vars[name] ?? ''))
        : key,
    i18n: { language: 'en' },
  }),
}))

const profile: UserProfile = {
  id: 7,
  username: 'wang',
  display_name: 'Wang',
  role: 1,
  status: 1,
  group: 'default',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  aff_count: 0,
  aff_quota: 0,
  aff_history_quota: 0,
  created_time: 1700000000,
  setting: '{}',
} as UserProfile

/** Renders the header and opens the dialog the pencil button leads to. */
async function openDialog(onProfileUpdate = vi.fn()) {
  render(
    <ProfileHeader
      profile={profile}
      loading={false}
      onProfileUpdate={onProfileUpdate}
    />
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Change display name' })
  )
  const field = await screen.findByLabelText('Display Name')
  return { field, onProfileUpdate }
}

/** The dialog's Save button. */
const saveButton = () => screen.getByRole('button', { name: 'Save' })

beforeEach(() => {
  vi.clearAllMocks()
  mocks.updateUserProfile.mockResolvedValue({ success: true })
})

describe('changing the display name', () => {
  it('opens from a button beside the name, holding the name as it is, and says what the name is for', async () => {
    const { field } = await openDialog()
    expect(field).toHaveValue('Wang')
    expect(
      screen.getByText(
        'This is the name others see: in the member list, the reports and the alerts. Your username stays wang.'
      )
    ).toBeInTheDocument()
    // Nothing to save until the name is changed.
    expect(saveButton()).toBeDisabled()
  })

  it('sends the name alone — so no password is asked — then tells the page and the sign-in', async () => {
    const { field, onProfileUpdate } = await openDialog()
    await userEvent.clear(field)
    await userEvent.type(field, '  王芳 ')
    await userEvent.click(saveButton())

    await waitFor(() =>
      expect(mocks.updateUserProfile).toHaveBeenCalledWith({
        display_name: '王芳',
      })
    )
    expect(mocks.updateUserProfile).toHaveBeenCalledTimes(1)
    expect(mocks.setUser).toHaveBeenCalledWith({
      id: 7,
      username: 'wang',
      display_name: '王芳',
    })
    expect(onProfileUpdate).toHaveBeenCalledTimes(1)
    expect(mocks.toast.success).toHaveBeenCalledWith('Display name changed')
    await waitFor(() =>
      expect(screen.queryByLabelText('Display Name')).toBeNull()
    )
  })

  it('does not let an empty, blank or overlong name be saved', async () => {
    const { field } = await openDialog()
    await userEvent.clear(field)
    expect(saveButton()).toBeDisabled()
    await userEvent.type(field, '   ')
    expect(saveButton()).toBeDisabled()
    await userEvent.clear(field)
    // Twenty characters is the most; one more and the hint turns red.
    await userEvent.type(field, '名'.repeat(20))
    expect(saveButton()).toBeEnabled()
    expect(screen.getByText('Up to 20 characters.')).not.toHaveClass(
      'text-destructive'
    )
    await userEvent.type(field, '名')
    expect(saveButton()).toBeDisabled()
    expect(screen.getByText('Up to 20 characters.')).toHaveClass(
      'text-destructive'
    )
    expect(mocks.updateUserProfile).not.toHaveBeenCalled()
  })

  it('keeps the dialog open and says so when the backend refuses', async () => {
    mocks.updateUserProfile.mockResolvedValueOnce({
      success: false,
      message: 'The display name must be between 1 and 20 characters.',
    })
    const { field, onProfileUpdate } = await openDialog()
    await userEvent.clear(field)
    await userEvent.type(field, 'Wang Fang')
    await userEvent.click(saveButton())

    await waitFor(() =>
      expect(mocks.toast.error).toHaveBeenCalledWith(
        'The display name must be between 1 and 20 characters.'
      )
    )
    expect(screen.getByLabelText('Display Name')).toHaveValue('Wang Fang')
    expect(mocks.setUser).not.toHaveBeenCalled()
    expect(onProfileUpdate).not.toHaveBeenCalled()
  })

  it('starts over from the current name each time it is opened', async () => {
    const { field } = await openDialog()
    await userEvent.clear(field)
    await userEvent.type(field, 'Half typed')
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() =>
      expect(screen.queryByLabelText('Display Name')).toBeNull()
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Change display name' })
    )
    expect(await screen.findByLabelText('Display Name')).toHaveValue('Wang')
    expect(mocks.updateUserProfile).not.toHaveBeenCalled()
  })
})
