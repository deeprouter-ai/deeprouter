/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: MCP PRD D7 — the MCP guide is listed with the other guides on the
// docs pages, but the "ask your AI" picker on the keys page leaves it out: the
// text that picker writes asks for an address to enter, and MCP is not that
// kind of tool, so the text it would write for it is wrong.
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { DOC_TITLES } from '@/features/docs/catalog'
import { ApiKeysAskAiSection } from '../api-keys-ask-ai-section'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en' },
  }),
}))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

// A native stand-in for the themed Select, as api-keys-setup-card.test does.
vi.mock('@/components/ui/select', () => ({
  Select: ({
    items,
    value,
  }: {
    items: { value: string; label: string }[]
    value: string
  }) => (
    <select aria-label='Your tool' value={value} onChange={() => {}}>
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

describe('ApiKeysAskAiSection — which guides it offers', () => {
  it('lists the MCP guide on the docs pages but not in the picker', () => {
    expect(DOC_TITLES.mcp).toBe('MCP server')
    render(<ApiKeysAskAiSection />)

    const names = screen
      .getAllByRole('option', { hidden: true })
      .map((option) => option.textContent)
    expect(names).toContain('Cherry Studio')
    expect(names).toContain('Cursor')
    expect(names).not.toContain('MCP server')
  })
})
