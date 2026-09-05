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
// Coverage: the container's own three render branches — loading, not-found,
// and success — previously uncovered (this file was assumed "too thin to
// break," but it has real branching logic worth pinning).
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillSummary } from '../../types'
import { SkillEditPage } from '../skill-edit-page'

const { mockGetSkill, mockListVersions } = vi.hoisted(() => ({
  mockGetSkill: vi.fn(),
  mockListVersions: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('../../api', () => ({
  getSkill: mockGetSkill,
  listVersions: mockListVersions,
}))

// This page's own job is just "which branch to render" — its children are
// each fully covered by their own test files, so stub them here to isolate
// SkillEditPage's branching from theirs.
vi.mock('../skill-metadata-form', () => ({
  SkillMetadataForm: ({ skill }: { skill: SkillSummary }) => (
    <div data-testid='metadata-form'>{skill.name}</div>
  ),
}))
vi.mock('../skill-versions-panel', () => ({
  SkillVersionsPanel: () => <div data-testid='versions-panel' />,
}))
vi.mock('../skill-publish-actions', () => ({
  SkillPublishActions: () => <div data-testid='publish-actions' />,
}))
vi.mock('../skill-activity-log', () => ({
  SkillActivityLog: () => <div data-testid='activity-log' />,
}))

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}

describe('SkillEditPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockListVersions.mockResolvedValue({ success: true, data: [] })
  })

  it('shows a loading state before the skill query resolves', () => {
    mockGetSkill.mockReturnValue(new Promise(() => {})) // never resolves
    renderWithQuery(<SkillEditPage skillId={1} />)

    expect(screen.getByText('Loading...')).toBeInTheDocument()
    expect(screen.queryByTestId('metadata-form')).not.toBeInTheDocument()
  })

  it('shows "Skill not found" when the query resolves with no data', async () => {
    mockGetSkill.mockResolvedValue({ success: false, data: undefined })
    renderWithQuery(<SkillEditPage skillId={1} />)

    expect(await screen.findByText('Skill not found')).toBeInTheDocument()
    expect(screen.queryByTestId('metadata-form')).not.toBeInTheDocument()
  })

  it('renders the full page and passes the skill down once loaded', async () => {
    mockGetSkill.mockResolvedValue({
      success: true,
      data: {
        id: 1,
        slug: 'code-review-expert',
        name: 'Code Review Expert',
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
      } satisfies SkillSummary,
    })
    renderWithQuery(<SkillEditPage skillId={1} />)

    expect(await screen.findByTestId('metadata-form')).toHaveTextContent(
      'Code Review Expert'
    )
    expect(screen.getByTestId('versions-panel')).toBeInTheDocument()
    expect(screen.getByTestId('publish-actions')).toBeInTheDocument()
    expect(screen.getByTestId('activity-log')).toBeInTheDocument()
  })
})
