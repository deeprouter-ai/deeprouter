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
// Coverage: the list page's row menu — a separate component from the edit
// page's SkillPublishActions, with its own canPublish/canDeprecate/canDelete
// gating and its own "navigate to the edit route" wiring. Zero shared code
// with skill-publish-actions.test.tsx, so it needs its own coverage.
import type { Row } from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillSummary } from '../../types'
import { DataTableRowActions } from '../data-table-row-actions'

const {
  mockPublishSkill,
  mockDeprecateSkill,
  mockNavigate,
  mockSetOpen,
  mockSetCurrentRow,
  mockTriggerRefresh,
  mockToast,
} = vi.hoisted(() => ({
  mockPublishSkill: vi.fn(),
  mockDeprecateSkill: vi.fn(),
  mockNavigate: vi.fn(),
  mockSetOpen: vi.fn(),
  mockSetCurrentRow: vi.fn(),
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
  publishSkill: mockPublishSkill,
  deprecateSkill: mockDeprecateSkill,
}))

vi.mock('../skills-provider', () => ({
  useSkills: () => ({
    setOpen: mockSetOpen,
    setCurrentRow: mockSetCurrentRow,
    triggerRefresh: mockTriggerRefresh,
  }),
}))

function makeSkill(overrides: Partial<SkillSummary> = {}): SkillSummary {
  return {
    id: 11,
    slug: 'row-skill',
    name: 'Row Skill',
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
    ...overrides,
  }
}

function renderRow(skill: SkillSummary) {
  const row = { original: skill } as Row<SkillSummary>
  render(<DataTableRowActions row={row} />)
}

// Base UI's menu mounts asynchronously — poll for it (findBy*) rather than
// assuming it is already in the DOM the instant the click resolves.
async function openMenu() {
  await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))
  await screen.findByRole('menu')
}

describe('DataTableRowActions (list page)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('navigates to the edit route with the skill id when Edit is clicked', async () => {
    renderRow(makeSkill({ id: 11 }))
    await openMenu()
    await userEvent.click(
      await screen.findByRole('menuitem', { name: /Edit/i })
    )

    expect(mockNavigate).toHaveBeenCalledWith({
      to: '/admin/skills/$id/edit',
      params: { id: '11' },
    })
  })

  it('disables Publish for a draft skill with no active version', async () => {
    renderRow(makeSkill({ status: 'draft', active_version_id: undefined }))
    await openMenu()
    expect(
      await screen.findByRole('menuitem', { name: /Publish/i })
    ).toHaveAttribute('aria-disabled', 'true')
  })

  it('enables Publish for a draft skill with an active version, and calls publishSkill', async () => {
    mockPublishSkill.mockResolvedValue({ success: true })
    renderRow(makeSkill({ id: 11, status: 'draft', active_version_id: 3 }))
    await openMenu()

    const publishItem = await screen.findByRole('menuitem', {
      name: /Publish/i,
    })
    expect(publishItem).not.toHaveAttribute('aria-disabled', 'true')
    await userEvent.click(publishItem)

    expect(mockPublishSkill).toHaveBeenCalledWith(11)
    expect(mockTriggerRefresh).toHaveBeenCalledTimes(1)
  })

  it('does not show a Deprecate item for a draft skill', async () => {
    renderRow(makeSkill({ status: 'draft' }))
    await openMenu()
    // Publish is always present for a draft skill — wait for the menu's
    // own content before asserting Deprecate's absence.
    await screen.findByRole('menuitem', { name: /Publish/i })
    expect(
      screen.queryByRole('menuitem', { name: /Deprecate/i })
    ).not.toBeInTheDocument()
  })

  it('shows Deprecate (not Publish) for a published skill, and calls deprecateSkill', async () => {
    mockDeprecateSkill.mockResolvedValue({ success: true })
    renderRow(makeSkill({ id: 11, status: 'published' }))
    await openMenu()

    const deprecateItem = await screen.findByRole('menuitem', {
      name: /Deprecate/i,
    })
    expect(
      screen.queryByRole('menuitem', { name: /Publish/i })
    ).not.toBeInTheDocument()

    await userEvent.click(deprecateItem)
    expect(mockDeprecateSkill).toHaveBeenCalledWith(11)
    expect(mockTriggerRefresh).toHaveBeenCalledTimes(1)
  })

  it('disables Delete unless the skill is a draft', async () => {
    renderRow(makeSkill({ status: 'published' }))
    await openMenu()
    expect(
      await screen.findByRole('menuitem', { name: /Delete/i })
    ).toHaveAttribute('aria-disabled', 'true')
  })

  it('opens the delete dialog for the current row when Delete is clicked on a draft skill', async () => {
    const skill = makeSkill({ status: 'draft' })
    renderRow(skill)
    await openMenu()
    const deleteItem = await screen.findByRole('menuitem', { name: /Delete/i })
    await userEvent.click(deleteItem)

    expect(mockSetCurrentRow).toHaveBeenCalledWith(skill)
    expect(mockSetOpen).toHaveBeenCalledWith('delete')
  })
})
