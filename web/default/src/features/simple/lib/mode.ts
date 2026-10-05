// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
/**
 * Which console a signed-in user sees (PRD `docs/console-simple-advanced-prd.md`
 * in the meta-repo, D5): the persona already stored on the user decides it.
 *
 *   casual / unset / missing / unreadable → Simple
 *   dev / team                            → Advanced
 *
 * Simple is the default on purpose: DeepRouter's paying audience is
 * non-technical, and new accounts land in Simple until they opt out.
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
  user: { setting?: unknown } | null | undefined
): ConsoleMode {
  const persona = readPersona(user?.setting)
  return persona === 'dev' || persona === 'team' ? 'advanced' : 'simple'
}

export function homePathFor(
  user: { setting?: unknown } | null | undefined
): string {
  return consoleModeFor(user) === 'simple' ? SIMPLE_HOME : ADVANCED_HOME
}
