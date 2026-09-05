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
// Coverage: the cell-rendering logic in useSkillsColumns() — status label
// mapping, free-vs-paid price display, and the active-version empty state —
// none of which had a test (skills-table.tsx's own react-query/URL-state
// wiring is still deliberately left to the real-browser walkthrough; this
// file only isolates the column defs from that plumbing).
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { SkillSummary } from '../../types'
import { useSkillsColumns } from '../skills-columns'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

// Featured/actions cells are each covered by their own test file — stub them
// here so a table-render failure there can't be mistaken for one here.
vi.mock('../skills-featured-cell', () => ({
  FeaturedCell: () => <div data-testid='featured-cell' />,
}))
vi.mock('../data-table-row-actions', () => ({
  DataTableRowActions: () => <div data-testid='row-actions' />,
}))

function makeSkill(overrides: Partial<SkillSummary>): SkillSummary {
  return {
    id: 1,
    slug: 'a-skill',
    name: 'A Skill',
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

function TestTable({ data }: { data: SkillSummary[] }) {
  const columns = useSkillsColumns()
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  return (
    <table>
      <tbody>
        {table.getRowModel().rows.map((row) => (
          <tr key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <td key={cell.id}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

describe('useSkillsColumns cell rendering', () => {
  it('shows a Free badge for a free skill', () => {
    render(<TestTable data={[makeSkill({ monetization_type: 'free' })]} />)
    expect(screen.getByText('Free')).toBeInTheDocument()
  })

  it('shows the formatted price for a paid skill', () => {
    render(
      <TestTable
        data={[makeSkill({ monetization_type: 'paid', price_usd: 4.9 })]}
      />
    )
    expect(screen.getByText('$4.90')).toBeInTheDocument()
  })

  it('shows the active version string when one is set', () => {
    render(<TestTable data={[makeSkill({ active_version: '1.2.0' })]} />)
    expect(screen.getByText('1.2.0')).toBeInTheDocument()
  })

  it('shows a placeholder when there is no active version', () => {
    render(<TestTable data={[makeSkill({ active_version: undefined })]} />)
    expect(screen.getByText('No active version')).toBeInTheDocument()
  })

  it.each([
    ['draft', 'Draft'],
    ['published', 'Published'],
    ['deprecated', 'Deprecated'],
  ] as const)('labels status %s as %s', (status, label) => {
    render(<TestTable data={[makeSkill({ status })]} />)
    expect(screen.getByText(label)).toBeInTheDocument()
  })

  it('renders the slug and name columns as plain text', () => {
    render(
      <TestTable data={[makeSkill({ slug: 'my-slug', name: 'My Name' })]} />
    )
    expect(screen.getByText('my-slug')).toBeInTheDocument()
    expect(screen.getByText('My Name')).toBeInTheDocument()
  })
})
