// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { formatQuotaWithCurrency } from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import type { OrgUsageGroupBy, OrgUsageReport, OrgUsageRow } from '../types'

/**
 * The four ways a usage report can be cut, in the order the page offers them:
 * the two a company reads first, then the two that name people and keys.
 */
export const USAGE_GROUPINGS: OrgUsageGroupBy[] = [
  'department',
  'model',
  'member',
  'key',
]

/** The stretches of time a report or the audit log can be asked for. */
export type OrgPeriodPreset =
  | 'today'
  | 'last7'
  | 'last30'
  | 'month'
  | 'last-month'
  | 'all'
  | 'custom'

/**
 * A stretch of time as the page keeps it: one of the presets, or — for
 * `custom` — a first and a last day, either of which may still be unset.
 */
export type OrgPeriod = {
  preset: OrgPeriodPreset
  from?: Date
  to?: Date
}

/** Name of a preset period in words. */
export function periodLabel(t: TFunction, preset: OrgPeriodPreset): string {
  switch (preset) {
    case 'today':
      return t('Today')
    case 'last7':
      return t('Last 7 days')
    case 'last30':
      return t('Last 30 days')
    case 'month':
      return t('This calendar month')
    case 'last-month':
      return t('Last calendar month')
    case 'all':
      return t('All time')
    case 'custom':
      return t('Custom dates')
  }
}

/**
 * The bounds of a period in Unix seconds, both included, as the backend takes
 * them. Days are the viewer's own days. A bound that is left out leaves that
 * side open: a period that runs up to now has no end, so that a request made a
 * second ago is in the next refresh.
 */
export function periodBounds(
  period: OrgPeriod,
  now: Date
): { start_timestamp?: number; end_timestamp?: number } {
  const today = dayjs(now).startOf('day')
  switch (period.preset) {
    case 'today':
      return { start_timestamp: today.unix() }
    case 'last7':
      return { start_timestamp: today.subtract(6, 'day').unix() }
    case 'last30':
      return { start_timestamp: today.subtract(29, 'day').unix() }
    case 'month':
      return { start_timestamp: today.startOf('month').unix() }
    case 'last-month': {
      const first = today.startOf('month').subtract(1, 'month')
      return {
        start_timestamp: first.unix(),
        end_timestamp: first.endOf('month').unix(),
      }
    }
    case 'all':
      return {}
    case 'custom':
      return {
        ...(period.from
          ? { start_timestamp: dayjs(period.from).startOf('day').unix() }
          : {}),
        ...(period.to
          ? { end_timestamp: dayjs(period.to).endOf('day').unix() }
          : {}),
      }
  }
}

/**
 * Whether a period can be asked for as it stands: hand-picked dates must not
 * end before they start.
 */
export function periodIsValid(period: OrgPeriod): boolean {
  if (period.preset !== 'custom' || !period.from || !period.to) return true
  return !dayjs(period.to).isBefore(dayjs(period.from), 'day')
}

/** What the rows of a report are, in words: the heading of their column. */
export function usageGroupLabel(
  t: TFunction,
  groupBy: OrgUsageGroupBy
): string {
  switch (groupBy) {
    case 'department':
      return t('Department')
    case 'member':
      return t('Member')
    case 'key':
      return t('Key')
    case 'model':
      return t('Model')
  }
}

/**
 * What to call a row. The backend names everything that still has a name; the
 * words here are for what does not — usage stamped with no department, an
 * account or a key that no longer exists at all.
 */
export function usageRowName(
  t: TFunction,
  groupBy: OrgUsageGroupBy,
  row: OrgUsageRow
): string {
  if (row.name) return row.name
  switch (groupBy) {
    case 'department':
      return t('No department')
    case 'member':
      return t('Someone who has left')
    case 'key':
      return t('A key that no longer exists')
    case 'model':
      return t('Unknown model')
  }
}

/** The word for a row that is gone: a member was removed, the rest deleted. */
export function usageGoneLabel(t: TFunction, groupBy: OrgUsageGroupBy): string {
  return groupBy === 'member'
    ? t('Removed from the organization')
    : t('Deleted since')
}

/**
 * A row's share of what the report adds up to, in percent — of money, the
 * measure the rows are sorted by. Zero when nothing was spent, and never
 * outside 0–100: a row that is all refunds has no share to show.
 */
export function usageShare(row: OrgUsageRow, report: OrgUsageReport): number {
  if (report.total.quota <= 0 || row.quota <= 0) return 0
  return Math.min(100, (row.quota / report.total.quota) * 100)
}

/**
 * Money in a report: the amount in full, never shortened to thousands — a
 * report is read against invoices.
 */
export function formatSpend(quota: number): string {
  return formatQuotaWithCurrency(quota, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: false,
  })
}
