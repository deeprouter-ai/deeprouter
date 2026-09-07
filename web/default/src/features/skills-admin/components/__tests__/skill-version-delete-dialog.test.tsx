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
// Coverage: this dialog is the UI entry point for DELETE /versions/:vid —
// implemented and tested on the backend since P2, never wired to a button
// until now. Cancel must not call the API; confirm must target the exact
// version passed in, not any other row.
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillVersion } from '../../types'
import { SkillVersionDeleteDialog } from '../skill-version-delete-dialog'

const { mockDeleteVersion, mockToast } = vi.hoisted(() => ({
  mockDeleteVersion: vi.fn(),
  mockToast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('../../api', () => ({
  deleteVersion: mockDeleteVersion,
}))

const draftVersion: SkillVersion = {
  id: 10,
  skill_id: 5,
  version: '1.0.0',
  status: 'draft',
  skill_md_content: '# x',
  manifest_json: {},
  changelog: '',
  created_by: 1,
  created_at: '2026-01-01T00:00:00Z',
}

describe('SkillVersionDeleteDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('is not shown when closed', () => {
    render(
      <SkillVersionDeleteDialog
        open={false}
        onOpenChange={vi.fn()}
        skillId={5}
        version={draftVersion}
        onDeleted={vi.fn()}
      />
    )
    expect(screen.queryByText('Are you sure?')).not.toBeInTheDocument()
  })

  it('shows the version number when open', () => {
    render(
      <SkillVersionDeleteDialog
        open
        onOpenChange={vi.fn()}
        skillId={5}
        version={draftVersion}
        onDeleted={vi.fn()}
      />
    )
    expect(screen.getByText('Are you sure?')).toBeInTheDocument()
    expect(screen.getByText('1.0.0')).toBeInTheDocument()
  })

  it('does not call deleteVersion when Cancel is clicked', async () => {
    render(
      <SkillVersionDeleteDialog
        open
        onOpenChange={vi.fn()}
        skillId={5}
        version={draftVersion}
        onDeleted={vi.fn()}
      />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(mockDeleteVersion).not.toHaveBeenCalled()
  })

  it('deletes the exact version passed in, closes and refreshes on confirm', async () => {
    mockDeleteVersion.mockResolvedValue({ success: true })
    const onOpenChange = vi.fn()
    const onDeleted = vi.fn()

    render(
      <SkillVersionDeleteDialog
        open
        onOpenChange={onOpenChange}
        skillId={5}
        version={draftVersion}
        onDeleted={onDeleted}
      />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(mockDeleteVersion).toHaveBeenCalledWith(5, 10)
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(onDeleted).toHaveBeenCalledTimes(1)
  })
})
