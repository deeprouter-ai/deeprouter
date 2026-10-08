// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { NoAssignedKeyError } from '../lib/purpose-key'
import { findPurpose } from '../lib/purposes'
import { SimpleUsePurpose } from '../pages/use-purpose'

const mockEnsurePurposeKey = vi.hoisted(() => vi.fn())
const mockIssueConnectToken = vi.hoisted(() => vi.fn())
/**
 * What the page is told about the user's organization: a personal account
 * unless a test says otherwise.
 */
const membership = vi.hoisted(() => ({
  current: { data: null as unknown, isLoading: false },
}))

vi.mock('../lib/purpose-key', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/purpose-key')>()),
  ensurePurposeKey: mockEnsurePurposeKey,
}))
vi.mock('@/features/keys/api', () => ({
  issueConnectToken: mockIssueConnectToken,
}))
vi.mock('@/features/org/api', () => ({ fetchOrgSelfKeys: vi.fn() }))
vi.mock('@/features/org/hooks/use-org-membership', () => ({
  useOrgMembership: () => membership.current,
}))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: { default_use_auto_group: false },
    loading: false,
  }),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, vars?: Record<string, string>) =>
      vars ? key.replace(/{{(\w+)}}/g, (_, k) => vars[k] ?? '') : key,
    i18n: { language: 'en' },
  }),
}))
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: ReactNode }) => <a href='/'>{children}</a>,
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const writeText = vi.fn()

function videoKey(modelLimits: string) {
  return {
    id: 7,
    name: 'my-video-key',
    status: 1,
    simple_purpose: 'video',
    model_limits_enabled: true,
    model_limits: modelLimits,
  }
}

beforeEach(() => {
  mockEnsurePurposeKey.mockReset()
  membership.current = { data: null, isLoading: false }
  writeText.mockReset().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText },
    configurable: true,
  })
  mockIssueConnectToken.mockReset().mockResolvedValue({
    success: true,
    data: { base_url: 'https://api.deeprouter.co', script_path: '/i/TOKEN' },
  })
})

describe('Simple video page', () => {
  it('lets the owner pick among the key’s models and copies the pick', async () => {
    mockEnsurePurposeKey.mockResolvedValue(
      videoKey('doubao-seedance-2-5-260628,doubao-seedance-2-0-260128')
    )
    render(<SimpleUsePurpose purpose={findPurpose('video')!} />)

    const radios = await screen.findAllByRole('radio')
    expect(radios).toHaveLength(2)
    // Cheapest known model is preselected.
    expect(radios[0]).toHaveAttribute('aria-checked', 'true')
    expect(radios[0]).toHaveTextContent('Seedance 2.0')

    await userEvent.click(radios[1])
    expect(radios[1]).toHaveAttribute('aria-checked', 'true')

    await userEvent.click(
      screen.getByRole('button', { name: /Copy for my AI/ })
    )
    await waitFor(() => expect(writeText).toHaveBeenCalled())
    const copied = writeText.mock.calls[0][0] as string
    expect(copied).toContain('Default model: doubao-seedance-2-5-260628')
    expect(copied).toContain('/i/TOKEN')
  })

  it('shows a single price line when the key holds one model', async () => {
    mockEnsurePurposeKey.mockResolvedValue(
      videoKey('doubao-seedance-2-0-260128')
    )
    render(<SimpleUsePurpose purpose={findPurpose('video')!} />)

    expect(await screen.findByText(/One clip: 5 s 1080p/)).toBeInTheDocument()
    expect(screen.queryAllByRole('radio')).toHaveLength(0)
  })

  it('tells the owner how to use it afterwards, with a concrete example', async () => {
    mockEnsurePurposeKey.mockResolvedValue(
      videoKey('doubao-seedance-2-5-260628,doubao-seedance-2-0-260128')
    )
    render(<SimpleUsePurpose purpose={findPurpose('video')!} />)

    expect(
      await screen.findByText('How to use it afterwards')
    ).toBeInTheDocument()
    expect(screen.getByText(/a cup of milk tea spinning/)).toBeInTheDocument()
    // With several models, step 4 shows how to switch.
    expect(
      screen.getByText(/Use Seedance 2.5 for this one/)
    ).toBeInTheDocument()
  })
})

// Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md D16): a member of an
// organization makes no key of their own. The page uses one they were handed
// and, when none of those can serve the purpose, says whom to ask.
describe('a member of an organization', () => {
  const member = { data: { org_id: 1 }, isLoading: false }
  const WHOM_TO_ASK =
    'You have no key that can do this yet. Keys are handed out by your organization — ask an administrator for one.'

  it('asks for a key the way a personal account does unless told otherwise', async () => {
    mockEnsurePurposeKey.mockResolvedValue(
      videoKey('doubao-seedance-2-0-260128')
    )
    render(<SimpleUsePurpose purpose={findPurpose('video')!} />)

    await waitFor(() =>
      expect(mockEnsurePurposeKey).toHaveBeenCalledWith('video', false, false)
    )
  })

  it('is set up with a key they were handed', async () => {
    membership.current = member
    mockEnsurePurposeKey.mockResolvedValue({
      id: 31,
      model_limits_enabled: true,
      model_limits: 'doubao-seedance-2-0-260128',
    })
    render(<SimpleUsePurpose purpose={findPurpose('video')!} />)

    await waitFor(() =>
      expect(mockEnsurePurposeKey).toHaveBeenCalledWith('video', false, true)
    )
    await waitFor(() =>
      expect(mockIssueConnectToken).toHaveBeenCalledWith(31, ['claude-code'])
    )
    expect(
      await screen.findByRole('button', { name: /Copy for my AI/ })
    ).toBeEnabled()
  })

  it('is told whom to ask when no key of theirs can do this, and offered nothing to copy', async () => {
    membership.current = member
    mockEnsurePurposeKey.mockRejectedValue(new NoAssignedKeyError())
    render(<SimpleUsePurpose purpose={findPurpose('image')!} />)

    expect(await screen.findByText(WHOM_TO_ASK)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Copy for my AI/ })).toBeNull()
    // Nothing went wrong, so it does not read as a failure either.
    expect(screen.queryByText(/Could not get this ready/)).toBeNull()
    expect(mockIssueConnectToken).not.toHaveBeenCalled()

    // Once an administrator has handed them a key, checking again finds it.
    mockEnsurePurposeKey.mockResolvedValue({
      id: 32,
      model_limits_enabled: false,
      model_limits: '',
    })
    await userEvent.click(screen.getByRole('button', { name: 'Check again' }))
    expect(
      await screen.findByRole('button', { name: /Copy for my AI/ })
    ).toBeEnabled()
    expect(screen.queryByText(WHOM_TO_ASK)).toBeNull()
    expect(mockEnsurePurposeKey).toHaveBeenCalledTimes(2)
  })

  it('still calls a failure a failure', async () => {
    membership.current = member
    mockEnsurePurposeKey.mockRejectedValue(new Error('the network is down'))
    render(<SimpleUsePurpose purpose={findPurpose('image')!} />)

    expect(
      await screen.findByText('Could not get this ready: the network is down')
    ).toBeInTheDocument()
    expect(screen.queryByText(WHOM_TO_ASK)).toBeNull()
    expect(screen.getByRole('button', { name: 'Try again' })).toBeEnabled()
  })

  it('makes no key while it does not know yet whether the user is in an organization', async () => {
    membership.current = { data: undefined, isLoading: true }
    render(<SimpleUsePurpose purpose={findPurpose('video')!} />)

    expect(screen.getByText('Getting ready…')).toBeInTheDocument()
    // A beat for an effect that should not run.
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(mockEnsurePurposeKey).not.toHaveBeenCalled()
  })
})
