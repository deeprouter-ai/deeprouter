// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'

/**
 * Whether a key belongs to an organization (Enterprise Org, meta-repo
 * `docs/enterprise-org-prd.md` §3). The personal key pages only look at such a
 * key: it is changed on the organization's keys page by whoever is allowed to
 * there, and its value never reaches the browser — its holder installs it with
 * one-click setup. So wherever these pages would change a key or fetch its
 * value, they ask this first. The backend refuses those calls anyway; asking
 * here keeps buttons that can only fail off the screen.
 */
export function isOrgKey(
  key: { org_id?: number | null } | null | undefined
): boolean {
  return Boolean(key?.org_id)
}

/**
 * What the empty key list says to a member of an organization, who makes no
 * key of their own (PRD D16): where their keys come from, in the words that
 * fit what they may do. Someone who may create the organization's keys is
 * pointed at the page for that; everyone else at an administrator.
 */
export function orgEmptyStateCopy(
  t: TFunction,
  createsOrgKeys: boolean
): { title: string; description: string } {
  return {
    title: t('No key has been assigned to you yet'),
    description: createsOrgKeys
      ? t(
          'Keys are created and handed out on the organization keys page. One that is made out to you shows up here.'
        )
      : t(
          'In an organization, keys are created and handed out by the members who may do so. Ask an administrator of your organization for one.'
        ),
  }
}
