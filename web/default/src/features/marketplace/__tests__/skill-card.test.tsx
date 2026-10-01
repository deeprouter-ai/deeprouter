/*
Copyright (C) 2026 DeepRouter

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
*/
// Coverage: PRD §13 — the marketplace card's Price badge only makes sense
// for a listing DR actually charges for; a reference listing has no
// purchase flow at all.
import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SkillCard } from '../components/skill-card'
import type { MarketplaceSkill } from '../types'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children }: { children: ReactNode }) => <a>{children}</a>,
}))

function makeSkill(overrides: Partial<MarketplaceSkill>): MarketplaceSkill {
  return {
    id: 1,
    slug: 'test-skill',
    name: 'Test Skill',
    description: 'Does things.',
    tags: [],
    status: 'published',
    monetization_type: 'free',
    price_usd: 0,
    featured_flag: false,
    featured_rank: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    version: '',
    listing_type: 'hosted',
    ...overrides,
  }
}

describe('SkillCard', () => {
  it('does not render a Price badge for a reference listing', () => {
    render(<SkillCard skill={makeSkill({ listing_type: 'reference' })} />)
    expect(screen.queryByText('Free')).not.toBeInTheDocument()
  })

  it('still renders the Price badge for a hosted skill', () => {
    render(<SkillCard skill={makeSkill({ listing_type: 'hosted' })} />)
    expect(screen.getByText('Free')).toBeInTheDocument()
  })

  it('renders a $-formatted Price badge for a paid hosted skill', () => {
    render(
      <SkillCard
        skill={makeSkill({
          listing_type: 'hosted',
          monetization_type: 'paid',
          price_usd: 4.99,
        })}
      />
    )
    expect(screen.getByText('$4.99')).toBeInTheDocument()
  })

  it('renders each tag as its own badge', () => {
    render(<SkillCard skill={makeSkill({ tags: ['writing', 'review'] })} />)
    expect(screen.getByText('writing')).toBeInTheDocument()
    expect(screen.getByText('review')).toBeInTheDocument()
  })
})
