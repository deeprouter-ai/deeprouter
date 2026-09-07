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
// Coverage: AC-8 — Admin can toggle featured_flag and edit featured_rank on
// a published skill — plus the guard that featured only ever applies to
// published skills (PRD §8.5: "对已发布技能开启/关闭").
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillSummary } from '../../types'
import { FeaturedCell } from '../skills-featured-cell'

const { mockUpdateFeatured, mockTriggerRefresh, mockToast } = vi.hoisted(
  () => ({
    mockUpdateFeatured: vi.fn(),
    mockTriggerRefresh: vi.fn(),
    mockToast: { success: vi.fn(), error: vi.fn() },
  })
)

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('../../api', () => ({
  updateFeatured: mockUpdateFeatured,
}))

vi.mock('../skills-provider', () => ({
  useSkills: () => ({ triggerRefresh: mockTriggerRefresh }),
}))

function makeSkill(overrides: Partial<SkillSummary>): SkillSummary {
  return {
    id: 3,
    slug: 'featured-skill',
    name: 'Featured Skill',
    description: '',
    category: 'code',
    tags: [],
    status: 'published',
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

describe('FeaturedCell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('disables the toggle and rank input for a non-published skill', () => {
    render(<FeaturedCell skill={makeSkill({ status: 'draft' })} />)
    // Base UI's Switch is a <span role="switch">, not a native <input>, so
    // it signals disabled via aria-disabled rather than the disabled prop.
    expect(screen.getByRole('switch')).toHaveAttribute('aria-disabled', 'true')
    expect(screen.getByRole('spinbutton')).toBeDisabled()
  })

  it('turns featured on and calls updateFeatured with the current rank', async () => {
    mockUpdateFeatured.mockResolvedValue({ success: true })
    render(
      <FeaturedCell
        skill={makeSkill({ featured_flag: false, featured_rank: 2 })}
      />
    )

    await userEvent.click(screen.getByRole('switch'))

    expect(mockUpdateFeatured).toHaveBeenCalledWith(3, {
      featured_flag: true,
      featured_rank: 2,
    })
    expect(mockTriggerRefresh).toHaveBeenCalledTimes(1)
  })

  it('updates featured_rank on blur when the value changed', async () => {
    mockUpdateFeatured.mockResolvedValue({ success: true })
    render(
      <FeaturedCell
        skill={makeSkill({ featured_flag: true, featured_rank: 1 })}
      />
    )

    const rankInput = screen.getByRole('spinbutton')
    await userEvent.clear(rankInput)
    await userEvent.type(rankInput, '5')
    await userEvent.tab()

    expect(mockUpdateFeatured).toHaveBeenCalledWith(3, {
      featured_flag: true,
      featured_rank: 5,
    })
  })

  it('does not call updateFeatured on blur when the rank is unchanged', async () => {
    render(
      <FeaturedCell
        skill={makeSkill({ featured_flag: true, featured_rank: 4 })}
      />
    )

    const rankInput = screen.getByRole('spinbutton')
    await userEvent.click(rankInput)
    await userEvent.tab()

    expect(mockUpdateFeatured).not.toHaveBeenCalled()
  })
})
