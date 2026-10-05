// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { findPurpose } from '../lib/purposes'
import { SimpleUsePurpose } from '../pages/use-purpose'

const mockEnsurePurposeKey = vi.hoisted(() => vi.fn())
const mockIssueConnectToken = vi.hoisted(() => vi.fn())

vi.mock('../lib/purpose-key', () => ({
  ensurePurposeKey: mockEnsurePurposeKey,
}))
vi.mock('@/features/keys/api', () => ({
  issueConnectToken: mockIssueConnectToken,
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
  writeText.mockReset().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText },
    configurable: true,
  })
  mockIssueConnectToken.mockResolvedValue({
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
