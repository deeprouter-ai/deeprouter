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
// Coverage: PRD §8.6 — SKILL.md is prefilled with the scaffold template, and
// manifest.json's version field stays in sync with the Version input until
// the admin hand-edits the JSON (the dirtyFields guard that stops fighting
// a manual edit).
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SkillVersionUploadDrawer } from '../skill-version-upload-drawer'

const { mockUploadVersion, mockToast } = vi.hoisted(() => ({
  mockUploadVersion: vi.fn(),
  mockToast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('../../api', () => ({
  uploadVersion: mockUploadVersion,
}))

function renderDrawer(
  overrides: Partial<Parameters<typeof SkillVersionUploadDrawer>[0]> = {}
) {
  const onOpenChange = vi.fn()
  const onUploaded = vi.fn()
  render(
    <SkillVersionUploadDrawer
      open
      onOpenChange={onOpenChange}
      skillId={5}
      skillSlug='code-review-expert'
      onUploaded={onUploaded}
      {...overrides}
    />
  )
  return { onOpenChange, onUploaded }
}

describe('SkillVersionUploadDrawer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('prefills SKILL.md with the PRD §8.6 scaffold template', () => {
    renderDrawer()
    const textarea = screen.getByLabelText('SKILL.md') as HTMLTextAreaElement
    expect(textarea.value).toContain('## Description')
    expect(textarea.value).toContain('## Instructions')
    expect(textarea.value).toContain('## Output Format')
  })

  it('prefills manifest.json with the skill slug and required fields', () => {
    renderDrawer()
    const textarea = screen.getByLabelText(
      'manifest.json'
    ) as HTMLTextAreaElement
    expect(textarea.value).toContain('"slug": "code-review-expert"')
    expect(textarea.value).toContain('"requires_deeprouter_key": true')
  })

  it("syncs manifest.json's version field as the admin types the version", async () => {
    renderDrawer()
    await userEvent.type(screen.getByLabelText('Version'), '2.0.0')

    const manifest = screen.getByLabelText(
      'manifest.json'
    ) as HTMLTextAreaElement
    expect(manifest.value).toContain('"version": "2.0.0"')
  })

  it('stops syncing manifest.json once the admin has hand-edited it', async () => {
    renderDrawer()

    const manifest = screen.getByLabelText(
      'manifest.json'
    ) as HTMLTextAreaElement
    // userEvent.type treats { and } as key-descriptor syntax, which makes
    // typing raw JSON unreliable — fireEvent.change sets the value directly
    // and still fires the same onChange React Hook Form listens to.
    fireEvent.change(manifest, { target: { value: '{"custom":true}' } })

    await userEvent.type(screen.getByLabelText('Version'), '3.0.0')

    expect(manifest.value).toBe('{"custom":true}')
  })

  it('uploads the parsed manifest and closes on success', async () => {
    mockUploadVersion.mockResolvedValue({ success: true })
    const { onOpenChange, onUploaded } = renderDrawer()

    await userEvent.type(screen.getByLabelText('Version'), '1.0.0')
    await userEvent.click(screen.getByRole('button', { name: 'Upload' }))

    expect(mockUploadVersion).toHaveBeenCalledWith(
      5,
      expect.objectContaining({
        version: '1.0.0',
        manifest_json: expect.objectContaining({
          slug: 'code-review-expert',
          version: '1.0.0',
          requires_deeprouter_key: true,
        }),
      })
    )
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(onUploaded).toHaveBeenCalledTimes(1)
  })

  it('shows the backend validation message and keeps the drawer open on failure', async () => {
    mockUploadVersion.mockResolvedValue({
      success: false,
      message: 'manifest_json is missing a required field: slug',
    })
    const { onOpenChange, onUploaded } = renderDrawer()

    await userEvent.type(screen.getByLabelText('Version'), '1.0.0')
    await userEvent.click(screen.getByRole('button', { name: 'Upload' }))

    expect(mockToast.error).toHaveBeenCalledWith(
      'manifest_json is missing a required field: slug'
    )
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(onUploaded).not.toHaveBeenCalled()
  })

  it('blocks submission when the version is not semver X.Y.Z', async () => {
    renderDrawer()
    await userEvent.type(screen.getByLabelText('Version'), 'not-a-version')
    await userEvent.click(screen.getByRole('button', { name: 'Upload' }))

    expect(
      await screen.findByText('Version must be in X.Y.Z format')
    ).toBeInTheDocument()
    expect(mockUploadVersion).not.toHaveBeenCalled()
  })
})
