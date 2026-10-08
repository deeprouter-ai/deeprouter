// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'
import type { OrgAlert, OrgAlertSettings, OrgAlertState } from '../types'

/**
 * What an alert is about, in a few words. A warning at 100% says the
 * allowance is gone, which is what it means for the key: it has stopped — and
 * so has a key that was refused a request it could not pay for, with a little
 * left (PRD D45).
 */
export function alertTitle(t: TFunction, alert: OrgAlert): string {
  switch (alert.rule) {
    case 'quota':
      if ((alert.detail?.needed ?? 0) > 0) return t('Quota too low to use')
      return alert.level >= 100
        ? t('Quota used up')
        : t('{{level}}% of quota used', { level: alert.level })
    case 'monthly':
      return alert.level >= 100
        ? t('Monthly requests used up')
        : t('{{level}}% of monthly requests used', { level: alert.level })
    case 'spike':
      return t('Sudden rise in spending')
    case 'offhours':
      return t('Spending outside working hours')
    case 'new_ip':
      return t('Unfamiliar IP address')
    default:
      return alert.rule
  }
}

/** The numbers an alert was raised with, as a sentence. */
export function alertSummary(t: TFunction, alert: OrgAlert): string {
  const detail = alert.detail ?? {}
  switch (alert.rule) {
    case 'quota':
      if ((detail.needed ?? 0) > 0) {
        return t(
          '{{used}} used of {{limit}}. A request that needed {{needed}} was refused.',
          {
            used: formatQuota(detail.used ?? 0),
            limit: formatQuota(detail.limit ?? 0),
            needed: formatQuota(detail.needed ?? 0),
          }
        )
      }
      return t('{{used}} used of {{limit}}', {
        used: formatQuota(detail.used ?? 0),
        limit: formatQuota(detail.limit ?? 0),
      })
    case 'monthly':
      return t('{{used}} of {{limit}} requests made this month', {
        used: detail.used ?? 0,
        limit: detail.limit ?? 0,
      })
    case 'spike':
      return t(
        '{{spent}} in the last 24 hours, against {{average}} a day over the week before',
        {
          spent: formatQuota(detail.spent ?? 0),
          average: formatQuota(detail.daily_average ?? 0),
        }
      )
    case 'offhours':
      return t(
        '{{spent}} outside working hours in the last 24 hours, against {{average}} a day over the week before',
        {
          spent: formatQuota(detail.spent ?? 0),
          average: formatQuota(detail.daily_average ?? 0),
        }
      )
    case 'new_ip': {
      const ips = (detail.ips ?? []).join(', ')
      return (detail.known_ips ?? 0) > 0
        ? t(
            'Used from {{ips}}. In the 30 days before it was used from {{known}} other address(es).',
            { ips, known: detail.known_ips }
          )
        : t(
            'Used from {{ips}}. No address is on record for it in the 30 days before.',
            { ips }
          )
    }
    default:
      return ''
  }
}

/** Whether an alert has been dealt with, in words. */
export function alertStateLabel(t: TFunction, state: OrgAlertState): string {
  switch (state) {
    case 'handled':
      return t('Handled')
    case 'false_alarm':
      return t('False alarm')
    default:
      return t('Unresolved')
  }
}

/** The most warning levels an organization can set. */
export const MAX_WARN_LEVELS = 5

/**
 * The alert settings as the form holds them: text wherever someone types, the
 * floor in the currency the console shows instead of quota units, and the
 * hours of a working day as the clock times a time field holds.
 */
export type OrgAlertSettingsForm = {
  /** The warning levels, as typed: "80, 100". */
  warnAt: string
  spikeMultiple: string
  offHoursPercent: string
  minSpend: string
  /** Whether the organization has said when it works. */
  hasWorkHours: boolean
  timezone: string
  /** 0 is Sunday. */
  days: number[]
  /** "09:00". */
  start: string
  end: string
}

/** The fields of the form that can be wrong. */
export type OrgAlertSettingsField =
  | 'warnAt'
  | 'spikeMultiple'
  | 'offHoursPercent'
  | 'minSpend'
  | 'timezone'
  | 'days'
  | 'hours'

/** What is wrong with the form, by field. */
export type OrgAlertSettingsErrors = Partial<
  Record<OrgAlertSettingsField, string>
>

/** Minutes after midnight as the clock time a time field holds: 540 is "09:00". */
function clockOf(minutes: number): string {
  // A day that ends at midnight is stored as minute 1440, which a clock shows
  // as 00:00.
  const wrapped = minutes % (24 * 60)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(wrapped / 60))}:${pad(wrapped % 60)}`
}

/** A clock time as minutes after midnight, or null when it is not one. */
function minutesOf(clock: string): number | null {
  const match = /^(\d{1,2}):(\d{2})$/.exec(clock.trim())
  if (!match) return null
  const hours = Number(match[1])
  const minutes = Number(match[2])
  return hours < 24 && minutes < 60 ? hours * 60 + minutes : null
}

/**
 * Whether the browser knows a time zone by that name. It knows neither the
 * empty name nor "Local", which the backend refuses as well: they would mean
 * wherever the server happens to run.
 */
function isTimeZone(name: string): boolean {
  try {
    new Intl.DateTimeFormat('en', { timeZone: name })
    return true
  } catch {
    return false
  }
}

/**
 * The form for the settings an organization has. With no working hours set it
 * is prepared with the usual ones — Monday to Friday, nine to six, in the
 * viewer's own zone — so that switching them on is one click.
 */
export function alertSettingsForm(
  settings: OrgAlertSettings,
  viewerZone: string
): OrgAlertSettingsForm {
  const hours = settings.work_hours
  return {
    warnAt: settings.warn_at.join(', '),
    spikeMultiple: String(settings.spike_multiple),
    offHoursPercent: String(settings.off_hours_percent),
    // Rounded to what the field shows; converting back gives the same units
    // for any amount that was typed into it.
    minSpend: String(
      Number(quotaUnitsToDollars(settings.min_spend).toFixed(6))
    ),
    hasWorkHours: hours !== null,
    timezone: hours?.timezone ?? viewerZone,
    days: hours?.days ?? [1, 2, 3, 4, 5],
    start: clockOf(hours?.start ?? 9 * 60),
    end: clockOf(hours?.end ?? 18 * 60),
  }
}

/** A whole number out of a text field, or null when it is anything else. */
function wholeNumber(raw: string): number | null {
  const text = raw.trim()
  return /^\d+$/.test(text) ? Number(text) : null
}

/**
 * Reads the form back into settings, or says which fields are wrong. The
 * backend makes the same checks and turns the whole request down with one
 * message for all of them, so the form has to be the one that points.
 */
export function alertSettingsOf(
  t: TFunction,
  form: OrgAlertSettingsForm
): { settings?: OrgAlertSettings; errors: OrgAlertSettingsErrors } {
  const errors: OrgAlertSettingsErrors = {}

  const typed = form.warnAt.split(/[\s,，、;；]+/).filter(Boolean)
  const levels = [...new Set(typed.map(wholeNumber))]
  if (levels.some((level) => level === null || level < 1 || level > 100)) {
    errors.warnAt = t('Each level is a whole number from 1 to 100.')
  } else if (levels.length > MAX_WARN_LEVELS) {
    errors.warnAt = t('At most {{max}} levels.', { max: MAX_WARN_LEVELS })
  }

  const spikeMultiple = wholeNumber(form.spikeMultiple)
  if (spikeMultiple === null || spikeMultiple < 2 || spikeMultiple > 1000) {
    errors.spikeMultiple = t('A whole number from 2 to 1000.')
  }
  const offHoursPercent = wholeNumber(form.offHoursPercent)
  if (
    offHoursPercent === null ||
    offHoursPercent < 1 ||
    offHoursPercent > 1000
  ) {
    errors.offHoursPercent = t('A whole number from 1 to 1000.')
  }
  const floor = form.minSpend.trim() === '' ? NaN : Number(form.minSpend)
  if (!Number.isFinite(floor) || floor < 0) {
    errors.minSpend = t('An amount of 0 or more.')
  }

  let start = 0
  let end = 0
  if (form.hasWorkHours) {
    if (!isTimeZone(form.timezone.trim())) {
      errors.timezone = t('Choose a time zone from the list.')
    }
    if (form.days.length === 0) {
      errors.days = t('Choose at least one working day.')
    }
    const from = minutesOf(form.start)
    // Midnight as the end of a day is the end of that day, not its start.
    const until = minutesOf(form.end) === 0 ? 24 * 60 : minutesOf(form.end)
    if (from === null || until === null || from >= until) {
      errors.hours = t(
        'The day has to end later than it starts. Hours that run past midnight are not supported.'
      )
    } else {
      start = from
      end = until
    }
  }

  if (Object.keys(errors).length > 0) return { errors }
  return {
    errors,
    settings: {
      warn_at: (levels as number[]).sort((a, b) => a - b),
      spike_multiple: spikeMultiple as number,
      off_hours_percent: offHoursPercent as number,
      min_spend: parseQuotaFromDollars(floor),
      work_hours: form.hasWorkHours
        ? {
            timezone: form.timezone.trim(),
            days: [...form.days].sort((a, b) => a - b),
            start,
            end,
          }
        : null,
    },
  }
}

/**
 * The time zones to offer, the viewer's own first. A browser too old to list
 * them still offers that one, and any name can be typed.
 */
export function timeZoneChoices(current: string): string[] {
  const listed =
    typeof Intl.supportedValuesOf === 'function'
      ? Intl.supportedValuesOf('timeZone')
      : []
  return [...new Set([current, 'UTC', ...listed].filter(Boolean))]
}

/** The viewer's own time zone, as the browser names it. */
export function viewerTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}
