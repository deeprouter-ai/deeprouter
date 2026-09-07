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
// Coverage: task-card AC-4 — "Publish disabled when active_version_id is
// empty" — plus the draft/deprecated/published branching that decides which
// button (Publish / Republish / Deprecate) renders at all.
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillSummary } from '../../types'
import { SkillPublishActions } from '../skill-publish-actions'

const { mockPublishSkill, mockDeprecateSkill, mockToast } = vi.hoisted(() => ({
  mockPublishSkill: vi.fn(),
  mockDeprecateSkill: vi.fn(),
  mockToast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('../../api', () => ({
  publishSkill: mockPublishSkill,
  deprecateSkill: mockDeprecateSkill,
}))

function makeSkill(overrides: Partial<SkillSummary>): SkillSummary {
  return {
    id: 1,
    slug: 'test-skill',
    name: 'Test Skill',
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

describe('SkillPublishActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('disables Publish when the skill has no active version', () => {
    render(
      <SkillPublishActions
        skill={makeSkill({ status: 'draft', active_version_id: undefined })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByRole('button', { name: /Publish/i })).toBeDisabled()
  })

  it('enables Publish once an active version is set, and calls publishSkill on click', async () => {
    const onChanged = vi.fn()
    mockPublishSkill.mockResolvedValue({ success: true })

    render(
      <SkillPublishActions
        skill={makeSkill({ status: 'draft', active_version_id: 42 })}
        onChanged={onChanged}
      />
    )

    const button = screen.getByRole('button', { name: /Publish/i })
    expect(button).toBeEnabled()
    await userEvent.click(button)

    expect(mockPublishSkill).toHaveBeenCalledWith(1)
    expect(onChanged).toHaveBeenCalledTimes(1)
  })

  it('labels the button Republish (not Publish) for a deprecated skill', () => {
    render(
      <SkillPublishActions
        skill={makeSkill({ status: 'deprecated', active_version_id: 42 })}
        onChanged={vi.fn()}
      />
    )
    expect(
      screen.getByRole('button', { name: /Republish/i })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /^Publish$/i })
    ).not.toBeInTheDocument()
  })

  it('shows Deprecate (not Publish) for a published skill, and calls deprecateSkill', async () => {
    const onChanged = vi.fn()
    mockDeprecateSkill.mockResolvedValue({ success: true })

    render(
      <SkillPublishActions
        skill={makeSkill({ status: 'published', active_version_id: 42 })}
        onChanged={onChanged}
      />
    )

    expect(
      screen.queryByRole('button', { name: /Publish/i })
    ).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: /Deprecate/i }))
    expect(mockDeprecateSkill).toHaveBeenCalledWith(1)
    expect(onChanged).toHaveBeenCalledTimes(1)
  })
})
