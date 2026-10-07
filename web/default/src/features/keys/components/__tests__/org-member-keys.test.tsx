/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
// Coverage: Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md D16). A
// member of an organization makes no key of their own: the backend refuses it,
// and these tests pin that the personal "API keys" page does not offer it in
// the first place — while a personal account keeps everything it had.
import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { orgEmptyStateCopy } from '../../lib/org-key'
import {
  ApiKeysEmptyState,
  ApiKeysOrgEmptyState,
  OrgKeysLink,
} from '../api-keys-empty-state'
import { ApiKeysPrimaryButtons } from '../api-keys-primary-buttons'

const mocks = vi.hoisted(() => ({
  setOpen: vi.fn(),
  /** What the page is told about the user's organization. */
  membership: { data: null as unknown },
}))

vi.mock('../api-keys-provider', () => ({
  useApiKeys: () => ({
    setOpen: mocks.setOpen,
    setCurrentRow: vi.fn(),
    setResolvedKey: vi.fn(),
  }),
}))
vi.mock('@/features/org/hooks/use-org-membership', () => ({
  useOrgMembership: () => mocks.membership,
}))
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children }: { to?: string; children?: ReactNode }) => (
    <a href={to}>{children}</a>
  ),
}))
const stableT = (key: string) => key
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: stableT, i18n: { language: 'en' } }),
}))

const ASK =
  'In an organization, keys are created and handed out by the members who may do so. Ask an administrator of your organization for one.'
const MAKE_THEM_THERE =
  'Keys are created and handed out on the organization keys page. One that is made out to you shows up here.'

beforeEach(() => {
  vi.clearAllMocks()
  mocks.membership = { data: null }
})

describe('the buttons above the list', () => {
  it('offer a personal account a key of its own', async () => {
    render(<ApiKeysPrimaryButtons />)

    await userEvent.click(
      screen.getByRole('button', { name: 'Create API Key' })
    )
    expect(mocks.setOpen).toHaveBeenCalledWith('mode-picker')
  })

  it('offer a member of an organization the setup guide, and neither a key to create nor the one-tap test', () => {
    mocks.membership = { data: { org_id: 1, permissions: [] } }
    render(<ApiKeysPrimaryButtons />)

    expect(screen.queryByRole('button', { name: 'Create API Key' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Setup guide' })).toBeEnabled()
    // The one-tap test runs on the playground endpoint, which is closed to
    // organization accounts since P7 (PRD §4).
    expect(screen.queryByText('Test a key')).toBeNull()
  })

  it('keep the button for a personal account while the answer is not in, or never comes', () => {
    // The account may be a personal one: taking its button away on a slow or
    // failed question would break a page that worked. The backend refuses a
    // member anyway.
    mocks.membership = { data: undefined }
    render(<ApiKeysPrimaryButtons />)

    expect(screen.getByRole('button', { name: 'Create API Key' })).toBeEnabled()
    expect(screen.getByText('Test a key')).toBeInTheDocument()
  })
})

describe('the empty list', () => {
  it('tells a member where keys come from, in the words that fit what they may do', () => {
    expect(orgEmptyStateCopy(stableT as never, false)).toEqual({
      title: 'No key has been assigned to you yet',
      description: ASK,
    })
    expect(orgEmptyStateCopy(stableT as never, true)).toEqual({
      title: 'No key has been assigned to you yet',
      description: MAKE_THEM_THERE,
    })
  })

  it('points a member who cannot create keys at an administrator, with nothing to press', () => {
    render(<ApiKeysOrgEmptyState createsOrgKeys={false} />)

    expect(
      screen.getByRole('heading', {
        name: 'No key has been assigned to you yet',
      })
    ).toBeInTheDocument()
    expect(screen.getByText(ASK)).toBeInTheDocument()
    expect(screen.queryByRole('button')).toBeNull()
    expect(screen.queryByRole('link')).toBeNull()
    expect(screen.queryByText('Get Started')).toBeNull()
  })

  it('points a member who creates the organization’s keys at the page for that', () => {
    render(<ApiKeysOrgEmptyState createsOrgKeys />)

    expect(screen.getByText(MAKE_THEM_THERE)).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Go to organization keys' })
    ).toHaveAttribute('href', '/org/keys')
    expect(screen.queryByText('Get Started')).toBeNull()
  })

  it('leads to the organization keys page', () => {
    render(<OrgKeysLink />)
    expect(
      screen.getByRole('link', { name: 'Go to organization keys' })
    ).toHaveAttribute('href', '/org/keys')
  })

  it('still invites a personal account to create its first key', async () => {
    const onCreate = vi.fn()
    render(<ApiKeysEmptyState onCreate={onCreate} />)

    expect(
      screen.getByRole('heading', { name: 'Create your first API key' })
    ).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Get Started' }))
    expect(onCreate).toHaveBeenCalledTimes(1)
  })
})
