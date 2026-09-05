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
// Coverage: AC-7 — the activity log is a collapsed-by-default section that
// only calls GET .../logs once expanded, and renders time/admin/action per
// row once it has data. Rows are a table (Time/Admin/Action/Details) rather
// than a flat list — the Details column opens a dialog with the raw JSON
// instead of dumping it inline, and is disabled when a log has no details.
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SkillActivityLog } from '../skill-activity-log'

const { mockGetSkillLogs } = vi.hoisted(() => ({
  mockGetSkillLogs: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('../../api', () => ({
  getSkillLogs: mockGetSkillLogs,
}))

function renderWithQuery(ui: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}

describe('SkillActivityLog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('does not fetch logs while collapsed', () => {
    renderWithQuery(<SkillActivityLog skillId={9} />)
    expect(mockGetSkillLogs).not.toHaveBeenCalled()
    expect(screen.queryByText('No activity yet')).not.toBeInTheDocument()
  })

  it('fetches and renders logs once expanded', async () => {
    mockGetSkillLogs.mockResolvedValue({
      success: true,
      data: [
        {
          id: 1,
          admin_id: 5,
          skill_id: 9,
          action: 'publish',
          details: { from_status: 'draft', to_status: 'published' },
          created_at: '2026-02-01T10:00:00Z',
        },
      ],
    })

    renderWithQuery(<SkillActivityLog skillId={9} />)

    await userEvent.click(screen.getByText('Activity Log'))

    expect(mockGetSkillLogs).toHaveBeenCalledWith(9)
    expect(await screen.findByText('publish')).toBeInTheDocument()
    expect(screen.getByText('#5')).toBeInTheDocument()
  })

  it('shows the empty state when there are no log entries', async () => {
    mockGetSkillLogs.mockResolvedValue({ success: true, data: [] })

    renderWithQuery(<SkillActivityLog skillId={9} />)
    await userEvent.click(screen.getByText('Activity Log'))

    expect(await screen.findByText('No activity yet')).toBeInTheDocument()
  })

  it('opens a dialog with the raw JSON when Details is clicked', async () => {
    mockGetSkillLogs.mockResolvedValue({
      success: true,
      data: [
        {
          id: 1,
          admin_id: 5,
          skill_id: 9,
          action: 'publish',
          details: { from_status: 'draft', to_status: 'published' },
          created_at: '2026-02-01T10:00:00Z',
        },
      ],
    })

    renderWithQuery(<SkillActivityLog skillId={9} />)
    await userEvent.click(screen.getByText('Activity Log'))
    await userEvent.click(await screen.findByRole('button', { name: 'View' }))

    expect(await screen.findByText('Log Details')).toBeInTheDocument()
    expect(
      screen.getByText(
        (_, node) =>
          node?.textContent ===
          '{\n  "from_status": "draft",\n  "to_status": "published"\n}'
      )
    ).toBeInTheDocument()
  })

  it('disables the Details button when a log has no details', async () => {
    mockGetSkillLogs.mockResolvedValue({
      success: true,
      data: [
        {
          id: 2,
          admin_id: 5,
          skill_id: 9,
          action: 'create',
          details: {},
          created_at: '2026-02-01T10:00:00Z',
        },
      ],
    })

    renderWithQuery(<SkillActivityLog skillId={9} />)
    await userEvent.click(screen.getByText('Activity Log'))

    expect(await screen.findByRole('button', { name: 'View' })).toBeDisabled()
  })
})
