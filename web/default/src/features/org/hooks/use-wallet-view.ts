// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useAuthStore } from '@/stores/auth-store'
import type { OrgMembership } from '../types'
import { useOrgMembership } from './use-org-membership'

/**
 * Whose balance this account's spending comes out of, as far as the console
 * needs to know (Enterprise Org, meta-repo `docs/enterprise-org-prd.md` §4).
 *
 * - `own` — the balance on the account is the one that gets spent, and the
 *   account's holder is the one who tops it up: a personal account, and the
 *   owner of an organization, whose balance *is* the company wallet.
 * - `company` — a member of an organization who is not its owner. Their keys
 *   spend the company wallet; the balance on their own account is not used
 *   and the wallet is not theirs to see, so the console shows them neither a
 *   balance nor a way to top up.
 * - `pending` — a member, and it is not known yet whether the owner. Nothing
 *   about a balance is shown until it is: a number that would have to be
 *   taken back a moment later is worse than a short gap.
 */
export type WalletView = 'own' | 'company' | 'pending'

/**
 * Decides the wallet view from what the console knows: the organization the
 * signed-in user's profile names (`undefined` for a personal account) and the
 * answer to "who am I in my organization" — `undefined` while that is still
 * being asked, or could not be asked.
 *
 * A personal account is `own` at once, whatever the question's state: it must
 * not wait on, or be changed by, anything to do with organizations.
 */
export function walletViewOf(
  orgId: number | undefined,
  membership: OrgMembership | null | undefined
): WalletView {
  if (membership) return membership.is_owner ? 'own' : 'company'
  if (orgId && membership === undefined) return 'pending'
  return 'own'
}

/** The wallet view of the signed-in user. */
export function useWalletView(): WalletView {
  const orgId = useAuthStore((s) => s.auth.user?.org_id)
  const { data: membership } = useOrgMembership()
  return walletViewOf(orgId, membership)
}
