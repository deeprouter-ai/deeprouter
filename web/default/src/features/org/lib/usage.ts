// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { formatQuotaWithCurrency } from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import type {
  OrgUsageGroupBy,
  OrgUsageReport,
  OrgUsageRow,
  OrgUsageTrend,
  OrgUsageTrendBucket,
} from '../types'

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
  row: Pick<OrgUsageRow, 'name'>
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

/**
 * The stretches of time a trend can be cut into, in the order the page offers
 * them (PRD D46).
 */
export const USAGE_TREND_BUCKETS: OrgUsageTrendBucket[] = [
  'day',
  'week',
  'month',
  'year',
]

/**
 * Whether a grouping is drawn over time above the table: a line per department
 * or per model. Members and keys are drawn one at a time, from their row (PRD
 * D49) — a line per member would be too many to read.
 */
export function drawsTrend(groupBy: OrgUsageGroupBy): boolean {
  return groupBy === 'department' || groupBy === 'model'
}

/** Name of a way to cut time, as the choice the page offers. */
export function trendBucketLabel(
  t: TFunction,
  bucket: OrgUsageTrendBucket
): string {
  switch (bucket) {
    case 'day':
      return t('By day')
    case 'week':
      return t('By week')
    case 'month':
      return t('By month')
    case 'year':
      return t('By year')
  }
}

/**
 * What a bucket is called on the time axis, from its first day as the backend
 * names it (`2026-10-05`): the day, the week as its first and last day, the
 * month or the year. Days and weeks carry the year only when asked to — when
 * the trend runs over more than one, and `10-05` alone would not say which.
 */
export function trendBucketName(
  bucket: OrgUsageTrendBucket,
  firstDay: string,
  withYear: boolean
): string {
  const day = dayjs(firstDay)
  const dated = day.format(withYear ? 'YYYY-MM-DD' : 'MM-DD')
  switch (bucket) {
    case 'day':
      return dated
    case 'week':
      return `${dated} – ${day.add(6, 'day').format('MM-DD')}`
    case 'month':
      return day.format('YYYY-MM')
    case 'year':
      return day.format('YYYY')
  }
}

/** One point of a trend, as the chart takes it. */
export type TrendPoint = {
  /** Where on the time axis: the name of the bucket. */
  bucket: string
  /** Which line: the name of the series. */
  series: string
  /** What was spent, in quota units. It can be negative: a refund. */
  quota: number
}

/** A trend laid out for the chart. */
export type TrendChart = {
  /** The names of the lines, in the order the backend sent them. */
  series: string[]
  /** Whether each of those lines is the one that stands for the rest. */
  rest: boolean[]
  /** Every point of every line, bucket by bucket. */
  points: TrendPoint[]
}

/**
 * Lays a trend out for the chart: names its buckets and its lines, and turns
 * the lines into points. Two lines never share a name — a department deleted
 * and created again would — because the chart tells lines apart by it.
 */
export function trendChart(t: TFunction, trend: OrgUsageTrend): TrendChart {
  const years = new Set(trend.buckets.map((day) => day.slice(0, 4)))
  const buckets = trend.buckets.map((day) =>
    trendBucketName(trend.bucket, day, years.size > 1)
  )
  const taken = new Set<string>()
  const series = trend.series.map((line) => {
    const name = line.other ? t('Other') : usageRowName(t, trend.group_by, line)
    const unique = taken.has(name) ? `${name} (#${line.id})` : name
    taken.add(unique)
    return unique
  })
  return {
    series,
    rest: trend.series.map((line) => line.other),
    points: buckets.flatMap((bucket, at) =>
      trend.series.map((line, i) => ({
        bucket,
        series: series[i],
        quota: line.points[at] ?? 0,
      }))
    ),
  }
}
