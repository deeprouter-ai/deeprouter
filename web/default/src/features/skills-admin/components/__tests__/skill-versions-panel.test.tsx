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
// Coverage: PRD §9 — activation failure must show the security-guard error
// detail to the Admin as a persistent alert (not a toast that vanishes),
// since they need it to go fix the package.
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SkillSummary, SkillVersion } from '../../types'
import { SkillVersionsPanel } from '../skill-versions-panel'

const { mockActivateVersion, mockToast } = vi.hoisted(() => ({
  mockActivateVersion: vi.fn(),
  mockToast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('../../api', () => ({
  activateVersion: mockActivateVersion,
}))

// Upload drawer has its own test file — stub it here so this file only
// exercises the panel's own state (activation + the version table).
vi.mock('../skill-version-upload-drawer', () => ({
  SkillVersionUploadDrawer: () => null,
}))

function makeSkill(overrides: Partial<SkillSummary> = {}): SkillSummary {
  return {
    id: 5,
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

function makeVersion(overrides: Partial<SkillVersion>): SkillVersion {
  return {
    id: 10,
    skill_id: 5,
    version: '1.0.0',
    status: 'draft',
    skill_md_content: '# x',
    manifest_json: {},
    changelog: '',
    created_by: 1,
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('SkillVersionsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the empty state when there are no versions', () => {
    render(
      <SkillVersionsPanel
        skill={makeSkill()}
        versions={[]}
        onChanged={vi.fn()}
      />
    )
    expect(
      screen.getByText('No versions yet — upload one to get started.')
    ).toBeInTheDocument()
  })

  it('does not render an Activate button for the already-active version', () => {
    render(
      <SkillVersionsPanel
        skill={makeSkill()}
        versions={[makeVersion({ id: 1, version: '1.0.0', status: 'active' })]}
        onChanged={vi.fn()}
      />
    )
    expect(
      screen.queryByRole('button', { name: /Activate/i })
    ).not.toBeInTheDocument()
  })

  it('activates a draft version and calls onChanged, with no error alert shown', async () => {
    const onChanged = vi.fn()
    mockActivateVersion.mockResolvedValue({ success: true })

    render(
      <SkillVersionsPanel
        skill={makeSkill()}
        versions={[makeVersion({ id: 10, version: '1.0.0', status: 'draft' })]}
        onChanged={onChanged}
      />
    )

    await userEvent.click(screen.getByRole('button', { name: 'Activate' }))

    expect(mockActivateVersion).toHaveBeenCalledWith(5, 10)
    expect(onChanged).toHaveBeenCalledTimes(1)
    expect(screen.queryByText('Activation failed')).not.toBeInTheDocument()
  })

  it('shows a persistent alert with the security-guard error detail on failure', async () => {
    const onChanged = vi.fn()
    mockActivateVersion.mockResolvedValue({
      success: false,
      message: 'package contains an OpenAI API key pattern (sk-...)',
    })

    render(
      <SkillVersionsPanel
        skill={makeSkill()}
        versions={[makeVersion({ id: 10, version: '1.0.0', status: 'draft' })]}
        onChanged={onChanged}
      />
    )

    await userEvent.click(screen.getByRole('button', { name: 'Activate' }))

    expect(
      await screen.findByText(
        'package contains an OpenAI API key pattern (sk-...)'
      )
    ).toBeInTheDocument()
    expect(onChanged).not.toHaveBeenCalled()

    // The alert must stay put — it's not a toast that auto-dismisses.
    await new Promise((r) => setTimeout(r, 50))
    expect(
      screen.getByText('package contains an OpenAI API key pattern (sk-...)')
    ).toBeInTheDocument()
  })

  it('labels a draft-status activate button "Activate" and an archived one "Reactivate"', () => {
    render(
      <SkillVersionsPanel
        skill={makeSkill()}
        versions={[
          makeVersion({ id: 1, version: '1.0.0', status: 'draft' }),
          makeVersion({ id: 2, version: '0.9.0', status: 'archived' }),
        ]}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByRole('button', { name: 'Activate' })).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Reactivate' })
    ).toBeInTheDocument()
  })
})
