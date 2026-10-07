// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { walletViewOf } from '../hooks/use-wallet-view'
import { membershipOf } from './fixtures'

// Enterprise Org P7 (meta-repo docs/enterprise-org-prd.md §4): the keys of an
// organization's members spend the company wallet, which is the owner's
// balance. So the console shows a balance, and a way to top it up, only to
// whoever's own balance is the one being spent.

describe('whose balance an account spends', () => {
  it('is its own for a personal account, whatever is known about organizations', () => {
    // Asked and answered "no organization", still being asked, never answered:
    // a personal account's pages must not wait on any of it.
    expect(walletViewOf(undefined, null)).toBe('own')
    expect(walletViewOf(undefined, undefined)).toBe('own')
    expect(walletViewOf(0, undefined)).toBe('own')
  })

  it('is its own for the owner, whose balance is the company wallet', () => {
    expect(walletViewOf(1, membershipOf('owner'))).toBe('own')
  })

  it('is the company’s for every other member, the admins included', () => {
    for (const role of ['admin', 'manager', 'staff', 'readonly'] as const) {
      expect(walletViewOf(1, membershipOf(role)), role).toBe('company')
    }
    // A custom role with every permission is still not the owner.
    expect(
      walletViewOf(
        1,
        membershipOf('staff', { role: 'Everything', role_scope: 'org' })
      )
    ).toBe('company')
  })

  it('is not decided for a member until it is known whether they are the owner', () => {
    expect(walletViewOf(1, undefined)).toBe('pending')
  })

  it('goes by the membership when the stored profile predates the field', () => {
    // A browser that signed in before profiles named the organization.
    expect(walletViewOf(undefined, membershipOf('staff'))).toBe('company')
    expect(walletViewOf(undefined, membershipOf('owner'))).toBe('own')
  })

  it('is its own once the backend says the account is in no organization', () => {
    expect(walletViewOf(1, null)).toBe('own')
  })
})
