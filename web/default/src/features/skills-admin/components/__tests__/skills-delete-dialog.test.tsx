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
// Coverage: delete only ever targets `currentRow` from context (not a prop),
// the dialog is gated on `open === 'delete'`, and Cancel must not call the
// API — draft-only deletion is enforced by disabling the row action
// upstream, but this dialog is the last line before an irreversible delete.
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillSummary } from '../../types'
import { SkillsDeleteDialog } from '../skills-delete-dialog'

const {
  mockDeleteSkill,
  mockUseSkills,
  mockSetOpen,
  mockTriggerRefresh,
  mockToast,
} = vi.hoisted(() => ({
  mockDeleteSkill: vi.fn(),
  mockUseSkills: vi.fn(),
  mockSetOpen: vi.fn(),
  mockTriggerRefresh: vi.fn(),
  mockToast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('../../api', () => ({
  deleteSkill: mockDeleteSkill,
}))

vi.mock('../skills-provider', () => ({
  useSkills: () => mockUseSkills(),
}))

const draftSkill: SkillSummary = {
  id: 21,
  slug: 'draft-skill',
  name: 'Draft Skill',
  description: '',
  category: 'code',
  tags: [],
  status: 'draft',
  monetization_type: 'free',
  price_usd: 0,
  featured_flag: false,
  featured_rank: 0,
  created_by: 1,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

describe('SkillsDeleteDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('is not shown when the open dialog is not "delete"', () => {
    mockUseSkills.mockReturnValue({
      open: null,
      setOpen: mockSetOpen,
      currentRow: draftSkill,
      triggerRefresh: mockTriggerRefresh,
    })
    render(<SkillsDeleteDialog />)
    expect(screen.queryByText('Are you sure?')).not.toBeInTheDocument()
  })

  it('shows the skill name and both actions when open', () => {
    mockUseSkills.mockReturnValue({
      open: 'delete',
      setOpen: mockSetOpen,
      currentRow: draftSkill,
      triggerRefresh: mockTriggerRefresh,
    })
    render(<SkillsDeleteDialog />)
    expect(screen.getByText('Are you sure?')).toBeInTheDocument()
    expect(screen.getByText('Draft Skill')).toBeInTheDocument()
  })

  it('does not call deleteSkill when Cancel is clicked', async () => {
    mockUseSkills.mockReturnValue({
      open: 'delete',
      setOpen: mockSetOpen,
      currentRow: draftSkill,
      triggerRefresh: mockTriggerRefresh,
    })
    render(<SkillsDeleteDialog />)
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(mockDeleteSkill).not.toHaveBeenCalled()
  })

  it('deletes currentRow, closes the dialog and refreshes on confirm', async () => {
    mockUseSkills.mockReturnValue({
      open: 'delete',
      setOpen: mockSetOpen,
      currentRow: draftSkill,
      triggerRefresh: mockTriggerRefresh,
    })
    mockDeleteSkill.mockResolvedValue({ success: true })

    render(<SkillsDeleteDialog />)
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(mockDeleteSkill).toHaveBeenCalledWith(21)
    expect(mockSetOpen).toHaveBeenCalledWith(null)
    expect(mockTriggerRefresh).toHaveBeenCalledTimes(1)
  })
})
