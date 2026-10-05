/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '../../types'
import { ApiKeysSetupCard } from '../api-keys-setup-card'

const CODING_KEY = {
  id: 2,
  name: 'my-coding-key',
  status: 1,
  model_limits_enabled: false,
  simple_purpose: 'coding',
} as ApiKey

vi.mock('../api-keys-provider', () => ({
  useApiKeys: () => ({
    setupKeys: [CODING_KEY],
    setupKeyId: CODING_KEY.id,
    setSetupKeyId: vi.fn(),
    setupKey: CODING_KEY,
  }),
}))

vi.mock('../../api', () => ({
  getConnectTools: () =>
    Promise.resolve({
      success: true,
      data: [{ id: 'claude-code', name: 'Claude Code' }],
    }),
}))

vi.mock('@/features/chat/hooks/use-chat-presets', () => ({
  useChatPresets: () => ({ chatPresets: [], serverAddress: '' }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

// The sections below the picker are not under test here.
vi.mock('../api-keys-one-click-card', () => ({
  ApiKeysOneClickSection: () => null,
}))
vi.mock('../api-keys-quick-apps-card', () => ({
  ApiKeysQuickAppsSection: () => null,
}))
vi.mock('../api-keys-ask-ai-section', () => ({
  ApiKeysAskAiSection: () => null,
}))

// A native stand-in for the themed Select: what matters is that it renders.
vi.mock('@/components/ui/select', () => ({
  Select: ({
    items,
    value,
  }: {
    items: { value: string; label: string }[]
    value: string
  }) => (
    <select aria-label='Key to set up' value={value} onChange={() => {}}>
      {items.map((item) => (
        <option key={item.value} value={item.value}>
          {item.label}
        </option>
      ))}
    </select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: () => null,
  SelectGroup: () => null,
  SelectItem: () => null,
}))

describe('ApiKeysSetupCard — the key picker row', () => {
  it('shows the picker and the media-key note even with a single candidate', async () => {
    // Media keys are filtered out of the candidates, so a user holding one
    // video key and one chat key sees a single candidate — and must still be
    // told which key gets configured, and why the video key is not offered
    // (@sam, 2026-10-05). The row used to render only for 2+ candidates.
    render(<ApiKeysSetupCard />)

    const picker = await screen.findByLabelText<HTMLSelectElement>(
      'Key to set up'
    )
    expect(picker.value).toBe(String(CODING_KEY.id))
    expect(
      screen.getByText(/Video, image and voice keys can't be used/)
    ).toBeInTheDocument()
  })
})
