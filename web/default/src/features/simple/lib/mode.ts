// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
/**
 * Which console a signed-in user sees (PRD `docs/console-simple-advanced-prd.md`
 * in the meta-repo, D5): the persona already stored on the user decides it.
 *
 *   casual                       → Simple
 *   dev / team                   → Advanced
 *   unset / missing / unreadable → the default for the kind of account
 *
 * Simple is the default on purpose: DeepRouter's paying audience is
 * non-technical, and new accounts land in Simple until they opt out. The
 * members of an organization are the exception (Enterprise Org, meta-repo
 * `docs/enterprise-org-prd.md` D42): until they choose, they work in the
 * Advanced console — where their keys are listed and installed, and where the
 * organization is managed.
 */
export type ConsoleMode = 'simple' | 'advanced'

export const SIMPLE_HOME = '/simple'
export const ADVANCED_HOME = '/dashboard'

/** Read `persona` from a `setting` field that may be an object or JSON string. */
export function readPersona(setting: unknown): string | undefined {
  let value: unknown = setting
  if (typeof value === 'string') {
    try {
      value = JSON.parse(value)
    } catch {
      return undefined
    }
  }
  if (!value || typeof value !== 'object') return undefined
  const persona = (value as { persona?: unknown }).persona
  return typeof persona === 'string' ? persona : undefined
}

export function consoleModeFor(
  user: { setting?: unknown; org_id?: number } | null | undefined
): ConsoleMode {
  const persona = readPersona(user?.setting)
  if (persona === 'dev' || persona === 'team') return 'advanced'
  if (persona === 'casual') return 'simple'
  // Nothing chosen yet: the profile names an organization only for its members.
  return user?.org_id ? 'advanced' : 'simple'
}

export function homePathFor(
  user: { setting?: unknown; org_id?: number } | null | undefined
): string {
  return consoleModeFor(user) === 'simple' ? SIMPLE_HOME : ADVANCED_HOME
}
