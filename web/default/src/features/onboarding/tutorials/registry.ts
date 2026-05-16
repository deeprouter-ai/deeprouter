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

import { CHATBOX } from './chatbox'
import { CHERRY_STUDIO } from './cherry-studio'
import { CLAUDE_CODE } from './claude-code'
import { CODE } from './code'
import { CURSOR } from './cursor'
import { LOBECHAT } from './lobechat'

/**
 * The slug is what appears in the URL (`/onboarding/<slug>`) and the
 * `<chip slug>` field in the API Key success dialog. Keep in sync with
 * `api-key-success-dialog.tsx::CLIENT_LINKS`.
 */
export type TutorialSlug =
  | 'cherry-studio'
  | 'chatbox'
  | 'lobechat'
  | 'cursor'
  | 'claude-code'
  | 'code'

/** Per-language markdown body. Frontend picks `zh` when the user's i18next
 *  language starts with "zh", otherwise falls back to `en`. */
export type TutorialContent = Record<'en' | 'zh', string>

export type Tutorial = {
  slug: TutorialSlug
  /** Display name shown in breadcrumb and as the page header. Not
   *  translated per-language — these are product names. */
  label: string
  /** One-line description, i18n key (translated by useTranslation). */
  descriptionKey: string
  /** Lucide icon name (resolved client-side); keep as string so the
   *  registry stays a plain TS module. */
  icon: 'cherry' | 'chat' | 'lobe' | 'cursor' | 'terminal' | 'code'
  /** Whether this client is "Recommended for non-tech" — surfaces a
   *  badge on the tutorial page header. Cherry Studio is the default
   *  recommendation for casual persona. */
  recommended?: boolean
  /** Markdown body. Built as a function so callers can interpolate the
   *  current base URL + model name at render time. */
  content: (vars: TutorialVars) => TutorialContent
}

export type TutorialVars = {
  /** Pulled from useStatus() at render time. */
  baseUrl: string
  /** What model name to suggest in the client config. Defaults to
   *  `deeprouter` (the virtual alias). */
  modelName: string
}

export const TUTORIALS: Record<TutorialSlug, Tutorial> = {
  'cherry-studio': CHERRY_STUDIO,
  chatbox: CHATBOX,
  lobechat: LOBECHAT,
  cursor: CURSOR,
  'claude-code': CLAUDE_CODE,
  code: CODE,
}

export const TUTORIAL_SLUGS: TutorialSlug[] = [
  'cherry-studio',
  'chatbox',
  'lobechat',
  'cursor',
  'claude-code',
  'code',
]

export function isTutorialSlug(slug: string): slug is TutorialSlug {
  return TUTORIAL_SLUGS.includes(slug as TutorialSlug)
}

export function getTutorial(slug: string): Tutorial | undefined {
  if (!isTutorialSlug(slug)) return undefined
  return TUTORIALS[slug]
}
