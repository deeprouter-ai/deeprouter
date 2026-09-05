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
// Coverage: PRD AC-4 — "Admin can edit a draft-status version's
// skill_md_content/manifest_json/changelog". This drawer is the UI entry
// point for the PUT /versions/:vid endpoint that P1/P2 implemented and
// tested but P5 never actually wired a button to.
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillVersion } from '../../types'
import { SkillVersionEditDrawer } from '../skill-version-edit-drawer'

const { mockUpdateVersion, mockToast } = vi.hoisted(() => ({
  mockUpdateVersion: vi.fn(),
  mockToast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('../../api', () => ({
  updateVersion: mockUpdateVersion,
}))

const draftVersion: SkillVersion = {
  id: 10,
  skill_id: 5,
  version: '1.0.0',
  status: 'draft',
  skill_md_content: '# Existing content',
  manifest_json: { slug: 'code-review-expert', version: '1.0.0' },
  changelog: 'initial draft',
  created_by: 1,
  created_at: '2026-01-01T00:00:00Z',
}

function renderDrawer(
  overrides: Partial<Parameters<typeof SkillVersionEditDrawer>[0]> = {}
) {
  const onOpenChange = vi.fn()
  const onUpdated = vi.fn()
  render(
    <SkillVersionEditDrawer
      open
      onOpenChange={onOpenChange}
      skillId={5}
      version={draftVersion}
      onUpdated={onUpdated}
      {...overrides}
    />
  )
  return { onOpenChange, onUpdated }
}

describe('SkillVersionEditDrawer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('prefills all fields from the version being edited', () => {
    renderDrawer()
    expect(
      (screen.getByLabelText('SKILL.md') as HTMLTextAreaElement).value
    ).toBe('# Existing content')
    expect(
      (screen.getByLabelText('manifest.json') as HTMLTextAreaElement).value
    ).toContain('"slug": "code-review-expert"')
    expect(
      (screen.getByLabelText('Changelog') as HTMLTextAreaElement).value
    ).toBe('initial draft')
  })

  it('shows the version number as read-only — it cannot be changed here', () => {
    renderDrawer()
    const versionInput = screen.getByLabelText('Version') as HTMLInputElement
    expect(versionInput.value).toBe('1.0.0')
    expect(versionInput).toBeDisabled()
  })

  it('submits the edited fields and closes on success', async () => {
    mockUpdateVersion.mockResolvedValue({ success: true })
    const { onOpenChange, onUpdated } = renderDrawer()

    const changelog = screen.getByLabelText('Changelog')
    await userEvent.clear(changelog)
    await userEvent.type(changelog, 'fixed a typo')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(mockUpdateVersion).toHaveBeenCalledWith(
      5,
      10,
      expect.objectContaining({ changelog: 'fixed a typo' })
    )
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(onUpdated).toHaveBeenCalledTimes(1)
  })

  it('shows the backend error and keeps the drawer open on failure', async () => {
    mockUpdateVersion.mockResolvedValue({
      success: false,
      message: 'version is not in draft status',
    })
    const { onOpenChange, onUpdated } = renderDrawer()

    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(mockToast.error).toHaveBeenCalledWith(
      'version is not in draft status'
    )
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(onUpdated).not.toHaveBeenCalled()
  })

  it('re-seeds the form when a different version is opened', () => {
    const { rerender } = render(
      <SkillVersionEditDrawer
        open
        onOpenChange={vi.fn()}
        skillId={5}
        version={draftVersion}
        onUpdated={vi.fn()}
      />
    )
    expect(
      (screen.getByLabelText('Changelog') as HTMLTextAreaElement).value
    ).toBe('initial draft')

    rerender(
      <SkillVersionEditDrawer
        open
        onOpenChange={vi.fn()}
        skillId={5}
        version={{ ...draftVersion, id: 11, changelog: 'second draft' }}
        onUpdated={vi.fn()}
      />
    )
    expect(
      (screen.getByLabelText('Changelog') as HTMLTextAreaElement).value
    ).toBe('second draft')
  })
})
