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
/**
 * Utilities for managing authentication-related browser storage
 */

// ============================================================================
// LocalStorage Keys
// ============================================================================

const STORAGE_KEYS = {
  USER_ID: 'uid',
  AFFILIATE: 'aff',
  STATUS: 'status',
  // One-shot register → /welcome handoff. The Welcome screen reads the
  // default token + trial quota + next route from this and clears it on
  // first read. Lives in sessionStorage (not localStorage) so refresh
  // wipes it — token is intentionally non-recoverable after first read.
  WELCOME_HANDOFF: 'dr_welcome_handoff',
  // Captured at /sign-up mount from document.referrer + URL utm_*
  // params. Sent up to backend in the register payload then cleared
  // by /welcome after consumption.
  ACQUISITION_META: 'dr_acquisition_meta',
} as const

// ============================================================================
// User ID Storage
// ============================================================================

/**
 * Save user ID to localStorage
 */
export function saveUserId(userId: number | string): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEYS.USER_ID, String(userId))
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save user ID:', error)
  }
}

/**
 * Get user ID from localStorage
 */
export function getUserId(): string | null {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage.getItem(STORAGE_KEYS.USER_ID)
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to get user ID:', error)
    return null
  }
}

/**
 * Remove user ID from localStorage
 */
export function removeUserId(): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.removeItem(STORAGE_KEYS.USER_ID)
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to remove user ID:', error)
  }
}

// ============================================================================
// Affiliate Code Storage
// ============================================================================

/**
 * Get affiliate code from localStorage
 */
export function getAffiliateCode(): string {
  if (typeof window === 'undefined') return ''
  try {
    return window.localStorage.getItem(STORAGE_KEYS.AFFILIATE) ?? ''
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to get affiliate code:', error)
    return ''
  }
}

/**
 * Save affiliate code to localStorage
 */
export function saveAffiliateCode(code: string): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEYS.AFFILIATE, code)
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to save affiliate code:', error)
  }
}

// ============================================================================
// Welcome screen handoff (sessionStorage — one-shot register response)
// ============================================================================

/**
 * Stash the register response so /welcome can display the one-time
 * default token + trial quota + next route. Token is gone after read.
 */
export function setWelcomeHandoff(data: unknown): void {
  if (typeof window === 'undefined') return
  try {
    window.sessionStorage.setItem(
      STORAGE_KEYS.WELCOME_HANDOFF,
      JSON.stringify(data)
    )
  } catch {
    /* sessionStorage unavailable in private mode — Welcome still works,
     * just without the default-token banner. */
  }
}

/** Read + clear the handoff. Returns null when not present. */
export function takeWelcomeHandoff<T>(): T | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.sessionStorage.getItem(STORAGE_KEYS.WELCOME_HANDOFF)
    if (!raw) return null
    window.sessionStorage.removeItem(STORAGE_KEYS.WELCOME_HANDOFF)
    return JSON.parse(raw) as T
  } catch {
    return null
  }
}

// ============================================================================
// Acquisition meta (UTM / referrer captured at /sign-up mount)
// ============================================================================

export type AcquisitionMeta = {
  channel: string // priority: utm_source > utm_medium > referrer host > 'direct'
  timezone: string // IANA tz from browser
}

/**
 * Snapshot referrer + UTM params on /sign-up mount so we know how the
 * user got here even if they navigate inside the app before submitting.
 */
export function captureAcquisitionMeta(): AcquisitionMeta {
  const fallback: AcquisitionMeta = { channel: 'direct', timezone: '' }
  if (typeof window === 'undefined') return fallback
  try {
    const url = new URL(window.location.href)
    const params = url.searchParams
    const utmSource = params.get('utm_source')
    const utmMedium = params.get('utm_medium')
    const ref = document.referrer
    let refHost = ''
    if (ref) {
      try {
        refHost = new URL(ref).host
      } catch {
        refHost = ''
      }
    }
    const channel = utmSource || utmMedium || refHost || 'direct'
    const timezone =
      Intl.DateTimeFormat().resolvedOptions().timeZone || ''
    const meta: AcquisitionMeta = { channel, timezone }
    try {
      window.sessionStorage.setItem(
        STORAGE_KEYS.ACQUISITION_META,
        JSON.stringify(meta)
      )
    } catch {
      /* private mode — meta still returned for current request */
    }
    return meta
  } catch {
    return fallback
  }
}

/** Read the snapshot if present (used by sign-up form to include in
 *  register payload). Does NOT clear — sign-up may submit multiple
 *  times (e.g. after fixing an error). */
export function readAcquisitionMeta(): AcquisitionMeta {
  const fallback: AcquisitionMeta = { channel: 'direct', timezone: '' }
  if (typeof window === 'undefined') return fallback
  try {
    const raw = window.sessionStorage.getItem(STORAGE_KEYS.ACQUISITION_META)
    if (!raw) return fallback
    return JSON.parse(raw) as AcquisitionMeta
  } catch {
    return fallback
  }
}
