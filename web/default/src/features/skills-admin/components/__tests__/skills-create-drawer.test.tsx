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
// Coverage: skill creation is the entry point into the whole P5 workflow —
// slug/price validation, and the create -> auto-navigate-to-edit-page chain
// that the rest of the feature (versions, publish) depends on existing.
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SkillsCreateDrawer } from '../skills-create-drawer'

const { mockCreateSkill, mockNavigate, mockTriggerRefresh, mockToast } =
  vi.hoisted(() => ({
    mockCreateSkill: vi.fn(),
    mockNavigate: vi.fn(),
    mockTriggerRefresh: vi.fn(),
    mockToast: { success: vi.fn(), error: vi.fn() },
  }))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../../api', () => ({
  createSkill: mockCreateSkill,
}))

vi.mock('../skills-provider', () => ({
  useSkills: () => ({ triggerRefresh: mockTriggerRefresh }),
}))

async function fillRequiredFields() {
  await userEvent.type(screen.getByLabelText('Name'), 'Code Review Expert')
  await userEvent.type(screen.getByLabelText('Slug'), 'code-review-expert')
  await userEvent.type(screen.getByLabelText('Description'), 'Reviews code.')
  await userEvent.type(screen.getByLabelText('Category'), 'code')
}

describe('SkillsCreateDrawer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('rejects a slug with uppercase letters or spaces', async () => {
    render(<SkillsCreateDrawer open onOpenChange={vi.fn()} />)
    await fillRequiredFields()

    const slugInput = screen.getByLabelText('Slug')
    await userEvent.clear(slugInput)
    await userEvent.type(slugInput, 'Not A Valid Slug')
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    expect(
      await screen.findByText(
        'Slug must be lowercase letters, numbers and hyphens only'
      )
    ).toBeInTheDocument()
    expect(mockCreateSkill).not.toHaveBeenCalled()
  })

  it('requires a price greater than 0 for a paid skill', async () => {
    render(<SkillsCreateDrawer open onOpenChange={vi.fn()} />)
    await fillRequiredFields()

    await userEvent.selectOptions(screen.getByLabelText('Monetization'), 'paid')
    // Price field defaults to 0 — left untouched, that's the violation.
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    expect(
      await screen.findByText('Price must be greater than 0 for a paid skill')
    ).toBeInTheDocument()
    expect(mockCreateSkill).not.toHaveBeenCalled()
  })

  it('creates a free skill and navigates to its edit page on success', async () => {
    const onOpenChange = vi.fn()
    mockCreateSkill.mockResolvedValue({ success: true, data: { id: 99 } })

    render(<SkillsCreateDrawer open onOpenChange={onOpenChange} />)
    await fillRequiredFields()
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    expect(mockCreateSkill).toHaveBeenCalledWith(
      expect.objectContaining({
        slug: 'code-review-expert',
        name: 'Code Review Expert',
        monetization_type: 'free',
        price_usd: 0,
      })
    )
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(mockTriggerRefresh).toHaveBeenCalledTimes(1)
    expect(mockNavigate).toHaveBeenCalledWith({
      to: '/admin/skills/$id/edit',
      params: { id: '99' },
    })
  })

  it('creates a paid skill with the entered price', async () => {
    mockCreateSkill.mockResolvedValue({ success: true, data: { id: 100 } })

    render(<SkillsCreateDrawer open onOpenChange={vi.fn()} />)
    await fillRequiredFields()
    await userEvent.selectOptions(screen.getByLabelText('Monetization'), 'paid')
    const priceInput = await screen.findByLabelText('Price (USD)')
    await userEvent.clear(priceInput)
    await userEvent.type(priceInput, '4.99')
    await userEvent.click(screen.getByRole('button', { name: 'Create' }))

    expect(mockCreateSkill).toHaveBeenCalledWith(
      expect.objectContaining({ monetization_type: 'paid', price_usd: 4.99 })
    )
  })
})
