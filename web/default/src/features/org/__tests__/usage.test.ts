// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'
import { formatQuota } from '@/lib/format'
import {
  drawsTrend,
  formatSpend,
  periodBounds,
  periodIsValid,
  periodLabel,
  trendBucketLabel,
  trendBucketName,
  trendChart,
  USAGE_GROUPINGS,
  USAGE_TREND_BUCKETS,
  usageGoneLabel,
  usageGroupLabel,
  usageRowName,
  usageShare,
} from '../lib/usage'
import type {
  OrgUsageReport,
  OrgUsageRow,
  OrgUsageTrend,
  OrgUsageTrendSeries,
} from '../types'

// Enterprise Org P9 (meta-repo docs/enterprise-org-prd.md §6): what the usage
// report asks the backend for and how it reads the answer. The period is the
// viewer's own days, so every moment here is built from local dates and the
// tests hold in any time zone.

/** A translator that fills in the placeholders and changes nothing else. */
const t = ((key: string, values?: Record<string, unknown>) =>
  key.replace(/{{(\w+)}}/g, (_, name: string) =>
    String(values?.[name] ?? '')
  )) as unknown as TFunction

/** A local moment in Unix seconds. Months are counted from 1. */
function at(
  year: number,
  month: number,
  day: number,
  hour = 0,
  minute = 0,
  second = 0
): number {
  return Math.floor(
    new Date(year, month - 1, day, hour, minute, second).getTime() / 1000
  )
}

/** Thursday 8 October 2026, mid-afternoon. */
const NOW = new Date(2026, 9, 8, 15, 30, 12)

/** A report row with the given figures. */
function rowOf(over: Partial<OrgUsageRow> = {}): OrgUsageRow {
  return {
    id: 1,
    name: 'Sales',
    is_service: false,
    gone: false,
    used_by: [],
    requests: 1,
    quota: 100,
    ...over,
  }
}

/** A report whose total spend is the given amount. */
function reportOf(quota: number): OrgUsageReport {
  return {
    group_by: 'department',
    scope: 'org',
    departments: [],
    total: { requests: 9, quota },
    rows: [],
  }
}

describe('the bounds of a period', () => {
  it('starts a period that runs up to now at midnight and leaves its end open', () => {
    expect(periodBounds({ preset: 'today' }, NOW)).toEqual({
      start_timestamp: at(2026, 10, 8),
    })
    // Seven days, today included: the 2nd to the 8th.
    expect(periodBounds({ preset: 'last7' }, NOW)).toEqual({
      start_timestamp: at(2026, 10, 2),
    })
    expect(periodBounds({ preset: 'last30' }, NOW)).toEqual({
      start_timestamp: at(2026, 9, 9),
    })
    expect(periodBounds({ preset: 'month' }, NOW)).toEqual({
      start_timestamp: at(2026, 10, 1),
    })
  })

  it('closes last month on its last second', () => {
    expect(periodBounds({ preset: 'last-month' }, NOW)).toEqual({
      start_timestamp: at(2026, 9, 1),
      end_timestamp: at(2026, 9, 30, 23, 59, 59),
    })
    // In January, last month is in last year.
    expect(
      periodBounds({ preset: 'last-month' }, new Date(2027, 0, 5, 9))
    ).toEqual({
      start_timestamp: at(2026, 12, 1),
      end_timestamp: at(2026, 12, 31, 23, 59, 59),
    })
  })

  it('does not bound "all time" at all', () => {
    expect(periodBounds({ preset: 'all' }, NOW)).toEqual({})
  })

  it('takes hand-picked dates as whole days, and leaves an unset side open', () => {
    const from = new Date(2026, 9, 1, 13, 45)
    const to = new Date(2026, 9, 3, 8, 15)
    expect(periodBounds({ preset: 'custom', from, to }, NOW)).toEqual({
      start_timestamp: at(2026, 10, 1),
      end_timestamp: at(2026, 10, 3, 23, 59, 59),
    })
    expect(periodBounds({ preset: 'custom', from }, NOW)).toEqual({
      start_timestamp: at(2026, 10, 1),
    })
    expect(periodBounds({ preset: 'custom', to }, NOW)).toEqual({
      end_timestamp: at(2026, 10, 3, 23, 59, 59),
    })
    expect(periodBounds({ preset: 'custom' }, NOW)).toEqual({})
  })

  it('ignores dates left over from "custom" once a preset is chosen', () => {
    const leftover = { from: new Date(2020, 0, 1), to: new Date(2020, 0, 2) }
    expect(periodBounds({ preset: 'today', ...leftover }, NOW)).toEqual({
      start_timestamp: at(2026, 10, 8),
    })
  })
})

describe('whether a period can be asked for', () => {
  it('refuses hand-picked dates that end before they start, and nothing else', () => {
    const first = new Date(2026, 9, 1)
    const third = new Date(2026, 9, 3)
    expect(periodIsValid({ preset: 'custom', from: third, to: first })).toBe(
      false
    )
    expect(periodIsValid({ preset: 'custom', from: first, to: third })).toBe(
      true
    )
    // One day is a period: the time of day on either date does not count.
    expect(
      periodIsValid({
        preset: 'custom',
        from: new Date(2026, 9, 1, 18),
        to: new Date(2026, 9, 1, 9),
      })
    ).toBe(true)
    expect(periodIsValid({ preset: 'custom', from: third })).toBe(true)
    expect(periodIsValid({ preset: 'custom' })).toBe(true)
    // Dates left over from "custom" do not spoil a preset.
    expect(periodIsValid({ preset: 'last7', from: third, to: first })).toBe(
      true
    )
  })
})

describe('what a report is called', () => {
  it('has words for every period and every grouping', () => {
    const presets = [
      'today',
      'last7',
      'last30',
      'month',
      'last-month',
      'all',
      'custom',
    ] as const
    const labels = presets.map((preset) => periodLabel(t, preset))
    expect(new Set(labels).size).toBe(presets.length)
    // Department and model first: what a company reads before who spent it.
    expect(USAGE_GROUPINGS).toEqual(['department', 'model', 'member', 'key'])
    expect(
      USAGE_GROUPINGS.map((grouping) => usageGroupLabel(t, grouping))
    ).toEqual(['Department', 'Model', 'Member', 'Key'])
  })

  it('calls a row by the name the backend gave it', () => {
    for (const grouping of USAGE_GROUPINGS) {
      expect(usageRowName(t, grouping, rowOf({ name: 'Acme' }))).toBe('Acme')
    }
  })

  it('has words for a row the backend could not name', () => {
    const unnamed = rowOf({ name: '' })
    expect(usageRowName(t, 'department', unnamed)).toBe('No department')
    expect(usageRowName(t, 'member', unnamed)).toBe('Someone who has left')
    expect(usageRowName(t, 'key', unnamed)).toBe('A key that no longer exists')
    expect(usageRowName(t, 'model', unnamed)).toBe('Unknown model')
  })

  it('says a member was removed and anything else was deleted', () => {
    expect(usageGoneLabel(t, 'member')).toBe('Removed from the organization')
    for (const grouping of ['department', 'key', 'model'] as const) {
      expect(usageGoneLabel(t, grouping)).toBe('Deleted since')
    }
  })
})

describe('the figures of a report', () => {
  it('gives a row its share of what was spent', () => {
    expect(usageShare(rowOf({ quota: 25 }), reportOf(100))).toBe(25)
    expect(usageShare(rowOf({ quota: 100 }), reportOf(100))).toBe(100)
    expect(usageShare(rowOf({ quota: 1 }), reportOf(3))).toBeCloseTo(33.333, 2)
  })

  it('gives no share where there is nothing to share out', () => {
    // Nothing was spent at all, or the total came to nothing after refunds.
    expect(usageShare(rowOf({ quota: 0 }), reportOf(0))).toBe(0)
    expect(usageShare(rowOf({ quota: 40 }), reportOf(0))).toBe(0)
    expect(usageShare(rowOf({ quota: 40 }), reportOf(-10))).toBe(0)
    // A row that is all refunds has none either.
    expect(usageShare(rowOf({ quota: -40 }), reportOf(100))).toBe(0)
  })

  it('never shows more than the whole', () => {
    // Refunds elsewhere can leave one row bigger than the total.
    expect(usageShare(rowOf({ quota: 150 }), reportOf(100))).toBe(100)
  })

  it('writes money out in full, however much it is', () => {
    // 500,000 quota units to the dollar. A report is read against invoices,
    // so twelve thousand dollars is not "12.3k".
    expect(formatSpend(500000)).toBe('$1')
    expect(formatSpend(12345.5 * 500000)).toBe('$12,345.5')
    expect(formatSpend(47100)).toBe('$0.0942')
    expect(formatSpend(0)).toBe('$0')
    expect(formatSpend(-20000)).toBe('-$0.04')
  })

  it('writes it out in full where the console shows quota units instead of money', () => {
    // An administrator can have the console show raw quota units. Elsewhere
    // those are shortened to thousands ("6172.8k"); a report keeps every one.
    const { setConfig } = useSystemConfigStore.getState()
    setConfig({
      currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'TOKENS' },
    })
    try {
      expect(formatSpend(6172750)).toBe('6172750')
      expect(formatQuota(6172750)).toBe('6172.8k')
    } finally {
      setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
    }
  })
})

// Enterprise Org P10 (PRD D46): the same usage drawn over time.

/** A line of a trend with the given fields. */
function lineOf(over: Partial<OrgUsageTrendSeries> = {}): OrgUsageTrendSeries {
  return { id: 11, name: 'Sales', other: false, quota: 0, points: [], ...over }
}

/** A trend by department over the given buckets. */
function trendOf(over: Partial<OrgUsageTrend> = {}): OrgUsageTrend {
  return {
    group_by: 'department',
    bucket: 'day',
    scope: 'org',
    buckets: [],
    total: 0,
    series: [],
    ...over,
  }
}

describe('what a trend is drawn by', () => {
  it('has a line per department or per model, and none per member or key', () => {
    expect(USAGE_GROUPINGS.filter(drawsTrend)).toEqual(['department', 'model'])
  })

  it('cuts time into days, weeks, months and years, each with a name of its own', () => {
    expect(USAGE_TREND_BUCKETS).toEqual(['day', 'week', 'month', 'year'])
    expect(
      USAGE_TREND_BUCKETS.map((bucket) => trendBucketLabel(t, bucket))
    ).toEqual(['By day', 'By week', 'By month', 'By year'])
  })
})

describe('what a bucket is called on the time axis', () => {
  it('is the day, the week from its first day to its last, the month or the year', () => {
    expect(trendBucketName('day', '2026-10-05', false)).toBe('10-05')
    expect(trendBucketName('week', '2026-10-05', false)).toBe('10-05 – 10-11')
    expect(trendBucketName('month', '2026-10-01', false)).toBe('2026-10')
    expect(trendBucketName('year', '2026-01-01', false)).toBe('2026')
  })

  it('carries the year on days and weeks when it is asked to', () => {
    expect(trendBucketName('day', '2026-12-31', true)).toBe('2026-12-31')
    // The week of the new year ends in the next one.
    expect(trendBucketName('week', '2026-12-28', true)).toBe(
      '2026-12-28 – 01-03'
    )
    expect(trendBucketName('month', '2026-12-01', true)).toBe('2026-12')
    expect(trendBucketName('year', '2027-01-01', true)).toBe('2027')
  })

  it('reads the day as the calendar date it names, in any time zone', () => {
    // "2026-10-05" is the fifth for the viewer, not midnight UTC of it.
    for (let day = 1; day <= 28; day++) {
      const name = `2026-03-${String(day).padStart(2, '0')}`
      expect(trendBucketName('day', name, true)).toBe(name)
    }
  })
})

describe('a trend laid out for the chart', () => {
  it('gives every line a point in every bucket, bucket by bucket', () => {
    const chart = trendChart(
      t,
      trendOf({
        buckets: ['2026-10-05', '2026-10-06'],
        series: [
          lineOf({ name: 'Sales', points: [300, 100] }),
          // Money given back on the second day.
          lineOf({ id: 12, name: 'Product', points: [500, -40] }),
        ],
      })
    )
    expect(chart.series).toEqual(['Sales', 'Product'])
    expect(chart.rest).toEqual([false, false])
    expect(chart.points).toEqual([
      { bucket: '10-05', series: 'Sales', quota: 300 },
      { bucket: '10-05', series: 'Product', quota: 500 },
      { bucket: '10-06', series: 'Sales', quota: 100 },
      { bucket: '10-06', series: 'Product', quota: -40 },
    ])
  })

  it('names buckets with their year only when the trend runs over more than one', () => {
    const over = (buckets: string[]) =>
      trendChart(
        t,
        trendOf({ buckets, series: [lineOf({ points: buckets.map(() => 1) })] })
      ).points.map((point) => point.bucket)
    expect(over(['2026-12-30', '2026-12-31'])).toEqual(['12-30', '12-31'])
    expect(over(['2026-12-31', '2027-01-01'])).toEqual([
      '2026-12-31',
      '2027-01-01',
    ])
  })

  it('has words for the line of the rest and for one the backend could not name', () => {
    const chart = trendChart(
      t,
      trendOf({
        buckets: ['2026-10-05'],
        series: [
          lineOf({ id: 0, name: '', points: [5] }),
          lineOf({ id: 0, name: '', other: true, points: [2] }),
        ],
      })
    )
    expect(chart.series).toEqual(['No department', 'Other'])
    expect(chart.rest).toEqual([false, true])
    expect(
      trendChart(
        t,
        trendOf({
          group_by: 'model',
          buckets: ['2026-10-05'],
          series: [lineOf({ id: 0, name: '', points: [5] })],
        })
      ).series
    ).toEqual(['Unknown model'])
  })

  it('never gives two lines one name', () => {
    // A department that was deleted and created again; and one called what
    // the line of the rest is called.
    const chart = trendChart(
      t,
      trendOf({
        buckets: ['2026-10-05'],
        series: [
          lineOf({ id: 11, name: 'Sales', points: [3] }),
          lineOf({ id: 14, name: 'Sales', points: [2] }),
          lineOf({ id: 15, name: 'Other', points: [1] }),
          lineOf({ id: 0, name: '', other: true, points: [1] }),
        ],
      })
    )
    expect(chart.series).toEqual([
      'Sales',
      'Sales (#14)',
      'Other',
      'Other (#0)',
    ])
    expect(new Set(chart.points.map((point) => point.series)).size).toBe(4)
  })

  it('draws nothing where the backend sent no point', () => {
    const chart = trendChart(
      t,
      trendOf({
        buckets: ['2026-10-05', '2026-10-06'],
        series: [lineOf({ points: [7] })],
      })
    )
    expect(chart.points.map((point) => point.quota)).toEqual([7, 0])
  })
})
