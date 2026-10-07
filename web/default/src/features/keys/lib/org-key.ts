// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later

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
