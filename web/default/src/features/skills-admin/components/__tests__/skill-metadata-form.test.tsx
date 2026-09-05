/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// Coverage: PRD §6.3 verbatim rule — changing monetization_type on a skill
// that has ever been published must confirm first ("this will affect
// existing users' download rights"); a draft skill must not be blocked by
// a confirm nobody needs yet.
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillSummary } from '../../types'
import { SkillMetadataForm } from '../skill-metadata-form'

const { mockUpdateSkill, mockToast } = vi.hoisted(() => ({
  mockUpdateSkill: vi.fn(),
  mockToast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('../../api', () => ({
  updateSkill: mockUpdateSkill,
}))

function makeSkill(overrides: Partial<SkillSummary>): SkillSummary {
  return {
    id: 7,
    slug: 'test-skill',
    name: 'Test Skill',
    description: 'A skill',
    category: 'code',
    tags: ['code'],
    status: 'draft',
    monetization_type: 'free',
    price_usd: 0,
    featured_flag: false,
    featured_rank: 0,
    created_by: 1,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

async function switchToPaidAndSetPrice() {
  await userEvent.selectOptions(screen.getByLabelText('Monetization'), 'paid')
  const priceInput = await screen.findByLabelText('Price (USD)')
  await userEvent.clear(priceInput)
  await userEvent.type(priceInput, '5')
}

describe('SkillMetadataForm — monetization change confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('does NOT confirm when the skill is still a draft', async () => {
    mockUpdateSkill.mockResolvedValue({ success: true })
    render(
      <SkillMetadataForm
        skill={makeSkill({ status: 'draft' })}
        onSaved={vi.fn()}
      />
    )

    await switchToPaidAndSetPrice()
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(screen.queryByText('Are you sure?')).not.toBeInTheDocument()
    expect(mockUpdateSkill).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ monetization_type: 'paid', price_usd: 5 })
    )
  })

  it('confirms before saving when the skill has been published, and does not save on Cancel', async () => {
    render(
      <SkillMetadataForm
        skill={makeSkill({ status: 'published' })}
        onSaved={vi.fn()}
      />
    )

    await switchToPaidAndSetPrice()
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(
      await screen.findByText(
        'This will affect the download entitlement of users who already have this skill. Continue?'
      )
    ).toBeInTheDocument()
    expect(mockUpdateSkill).not.toHaveBeenCalled()

    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(mockUpdateSkill).not.toHaveBeenCalled()
  })

  it('saves after the admin confirms on a deprecated (previously-published) skill', async () => {
    const onSaved = vi.fn()
    mockUpdateSkill.mockResolvedValue({ success: true })

    render(
      <SkillMetadataForm
        skill={makeSkill({ status: 'deprecated' })}
        onSaved={onSaved}
      />
    )

    await switchToPaidAndSetPrice()
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await userEvent.click(screen.getByRole('button', { name: 'Continue' }))

    expect(mockUpdateSkill).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ monetization_type: 'paid', price_usd: 5 })
    )
    expect(onSaved).toHaveBeenCalledTimes(1)
  })

  it('does not confirm when monetization_type is left unchanged, even if published', async () => {
    mockUpdateSkill.mockResolvedValue({ success: true })
    render(
      <SkillMetadataForm
        skill={makeSkill({ status: 'published', name: 'Old Name' })}
        onSaved={vi.fn()}
      />
    )

    const nameInput = screen.getByLabelText('Name')
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'New Name')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(screen.queryByText('Are you sure?')).not.toBeInTheDocument()
    expect(mockUpdateSkill).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ name: 'New Name', monetization_type: 'free' })
    )
  })
})

// Coverage: AC-9 — "Skill 处于 draft 时可修改 slug；published 后修改 slug 返回
// 409". The slug field previously showed the same "locked — cannot be
// changed here" text unconditionally, in every status; UpdateSkillRequest
// (Go) had no slug field at all, so there was no draft-editable path.
describe('SkillMetadataForm — slug editability (AC-9)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the slug as an editable input while draft, and submits changes', async () => {
    mockUpdateSkill.mockResolvedValue({ success: true })
    render(
      <SkillMetadataForm
        skill={makeSkill({ status: 'draft', slug: 'old-slug' })}
        onSaved={vi.fn()}
      />
    )

    const slugInput = screen.getByLabelText('Slug') as HTMLInputElement
    expect(slugInput.value).toBe('old-slug')
    await userEvent.clear(slugInput)
    await userEvent.type(slugInput, 'new-slug')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(mockUpdateSkill).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ slug: 'new-slug' })
    )
  })

  it.each(['published', 'deprecated'] as const)(
    'shows the slug as locked text, not an input, once %s',
    (status) => {
      render(
        <SkillMetadataForm
          skill={makeSkill({ status, slug: 'locked-slug' })}
          onSaved={vi.fn()}
        />
      )

      expect(screen.getByText(/locked-slug/)).toBeInTheDocument()
      expect(
        screen.getByText('(locked — cannot be changed here)')
      ).toBeInTheDocument()
      expect(screen.queryByLabelText('Slug')).not.toBeInTheDocument()
    }
  )
})
