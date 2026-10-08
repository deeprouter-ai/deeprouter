/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: what the Simple console's frame records about whoever lands in it.
// A brand-new personal account is sent here by default, so landing here is
// its choice and is stored once (Console Simple/Advanced PRD D5). A member of
// an organization is not sent here by default — they start in the Advanced
// console (Enterprise Org, meta-repo docs/enterprise-org-prd.md D42) — so a
// visit is not stored as their choice.
import type { ReactNode } from 'react'
import { render } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SimpleShell } from '../components/simple-shell'

const mocks = vi.hoisted(() => ({
  user: null as Record<string, unknown> | null,
  persistPersona: vi.fn(),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: mocks.user } }),
}))
vi.mock('@/features/profile/lib/persist-persona', () => ({
  persistPersona: mocks.persistPersona,
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children }: { to?: string; children?: ReactNode }) => (
    <a href={to}>{children}</a>
  ),
  Outlet: () => null,
  useRouterState: () => '/simple',
}))

/** Renders the frame for an account with the given stored choice. */
function landAs(persona: string, more: Record<string, unknown> = {}) {
  mocks.user = { id: 7, setting: { persona }, ...more }
  render(<SimpleShell />)
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.persistPersona.mockResolvedValue(null)
})

describe('landing in the Simple console', () => {
  it('is stored as the choice of a personal account that has not chosen', () => {
    landAs('unset')
    expect(mocks.persistPersona).toHaveBeenCalledExactlyOnceWith('casual')
  })

  it('is not stored for a member of an organization who has not chosen', () => {
    landAs('unset', { org_id: 5 })
    expect(mocks.persistPersona).not.toHaveBeenCalled()
  })

  it.each(['casual', 'dev', 'team'])(
    'changes nothing for an account that chose %s',
    (persona) => {
      landAs(persona)
      landAs(persona, { org_id: 5 })
      expect(mocks.persistPersona).not.toHaveBeenCalled()
    }
  )
})
