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
// Coverage: this component's only job is wiring `open === 'create'` to the
// create drawer's `open` prop and closing it back to null — worth pinning
// since it's the one place that mapping could silently invert.
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { SkillsDialogs } from '../skills-dialogs'

const { mockUseSkills, mockSetOpen } = vi.hoisted(() => ({
  mockUseSkills: vi.fn(),
  mockSetOpen: vi.fn(),
}))

vi.mock('../skills-provider', () => ({
  useSkills: () => mockUseSkills(),
}))

vi.mock('../skills-create-drawer', () => ({
  SkillsCreateDrawer: ({
    open,
    onOpenChange,
  }: {
    open: boolean
    onOpenChange: (v: boolean) => void
  }) => (
    <div data-testid='create-drawer' data-open={open}>
      <button onClick={() => onOpenChange(false)}>close</button>
    </div>
  ),
}))
vi.mock('../skills-delete-dialog', () => ({
  SkillsDeleteDialog: () => <div data-testid='delete-dialog' />,
}))

describe('SkillsDialogs', () => {
  it('passes open=true to the create drawer only when open is "create"', () => {
    mockUseSkills.mockReturnValue({ open: 'create', setOpen: mockSetOpen })
    render(<SkillsDialogs />)
    expect(screen.getByTestId('create-drawer')).toHaveAttribute(
      'data-open',
      'true'
    )
  })

  it('passes open=false to the create drawer for any other state', () => {
    mockUseSkills.mockReturnValue({ open: 'delete', setOpen: mockSetOpen })
    render(<SkillsDialogs />)
    expect(screen.getByTestId('create-drawer')).toHaveAttribute(
      'data-open',
      'false'
    )
  })

  it('always renders the delete dialog regardless of open state', () => {
    mockUseSkills.mockReturnValue({ open: null, setOpen: mockSetOpen })
    render(<SkillsDialogs />)
    expect(screen.getByTestId('delete-dialog')).toBeInTheDocument()
  })

  it('calls setOpen(null) when the create drawer closes', async () => {
    mockUseSkills.mockReturnValue({ open: 'create', setOpen: mockSetOpen })
    render(<SkillsDialogs />)
    await userEvent.click(screen.getByText('close'))
    expect(mockSetOpen).toHaveBeenCalledWith(null)
  })
})
