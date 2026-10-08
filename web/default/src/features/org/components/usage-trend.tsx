// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useMemo, useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import { useTranslation } from 'react-i18next'
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ErrorState } from '@/components/error-state'
import {
  fetchOrgUsageTrend,
  orgQueryKeys,
  type OrgUsageTrendParams,
} from '../api'
import { viewerTimeZone } from '../lib/alerts'
import {
  formatSpend,
  type OrgPeriod,
  periodBounds,
  periodIsValid,
  trendBucketLabel,
  trendChart,
  type TrendPoint,
  usageGroupLabel,
  USAGE_TREND_BUCKETS,
} from '../lib/usage'
import type { OrgUsageTrendBucket, OrgUsageTrendGroupBy } from '../types'

/** The theme's chart colours, in the order lines take them. */
const LINE_COLOURS = [
  '--chart-1',
  '--chart-2',
  '--chart-3',
  '--chart-4',
  '--chart-5',
]

/** How a line is dashed once the colours have gone round, and the rest's own. */
const SECOND_ROUND_DASH = [6, 4]
const REST_DASH = [2, 3]

/** How many points a line may have and still mark each of them with a dot. */
const DOTS_UP_TO = 62

/** How big those dots are, and how big when a line is nothing but one. */
const DOT_SIZE = 5
const LONE_DOT_SIZE = 10

/** Reads a colour of the theme off the page; empty where it is not set. */
function themeColour(name: string): string {
  return window.getComputedStyle(document.body).getPropertyValue(name).trim()
}

type UsageTrendProps = {
  /** What the lines are: one per department, or one per model. */
  groupBy: OrgUsageTrendGroupBy
  /** The period and the department of the report this is drawn above. */
  period: OrgPeriod
  departmentId?: number
}

/**
 * The usage report drawn over time (Enterprise Org PRD D46): what was spent
 * in each day, week, month or year of the period, with a line per department
 * or per model. It asks for the period and the department the report is
 * showing, and — like the report — is sent exactly the part of the company
 * the viewer may see. Days are the viewer's own days.
 */
export function UsageTrend({ groupBy, period, departmentId }: UsageTrendProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const [bucket, setBucket] = useState<OrgUsageTrendBucket>('day')

  const params: OrgUsageTrendParams = {
    group_by: groupBy,
    bucket,
    timezone: viewerTimeZone(),
    ...periodBounds(period, new Date()),
    ...(departmentId ? { department_id: departmentId } : {}),
  }
  const query = useQuery({
    queryKey: orgQueryKeys.usageTrend(params),
    queryFn: () => fetchOrgUsageTrend(params),
    enabled: periodIsValid(period),
    // The last chart stays on screen while the next one is fetched.
    placeholderData: keepPreviousData,
  })
  const trend = query.data

  const spec = useMemo(() => {
    if (!themeReady || !trend || trend.series.length === 0) return null
    const chart = trendChart(t, trend)
    const colours = LINE_COLOURS.map(themeColour).filter(Boolean)
    const muted = themeColour('--muted-foreground')
    // The words on the axes are secondary text, in the theme's colour for it.
    const axisText = { fontSize: 11, ...(muted ? { fill: muted } : {}) }
    const dashOf = new Map(
      chart.series.map((name, i) => [
        name,
        chart.rest[i]
          ? REST_DASH
          : i >= LINE_COLOURS.length
            ? SECOND_ROUND_DASH
            : [],
      ])
    )
    return {
      type: 'line' as const,
      data: [{ id: 'trend', values: chart.points }],
      xField: 'bucket',
      yField: 'quota',
      seriesField: 'series',
      // Without the theme's colours the chart falls back on its own.
      ...(colours.length > 0
        ? {
            color: {
              type: 'ordinal',
              domain: chart.series,
              range: chart.series.map((_, i) =>
                chart.rest[i] && muted ? muted : colours[i % colours.length]
              ),
            },
          }
        : {}),
      line: {
        style: {
          lineWidth: 2,
          lineDash: (point: TrendPoint) => dashOf.get(point.series) ?? [],
        },
      },
      // A trend of one bucket is a dot, and has to be seen.
      point: {
        visible: trend.buckets.length <= DOTS_UP_TO,
        style: { size: trend.buckets.length === 1 ? LONE_DOT_SIZE : DOT_SIZE },
      },
      legends: { visible: true, orient: 'bottom' },
      tooltip: {
        mark: {
          title: { value: (point: TrendPoint) => point.bucket },
          content: [
            {
              key: (point: TrendPoint) => point.series,
              value: (point: TrendPoint) => formatSpend(point.quota),
            },
          ],
        },
        dimension: {
          content: [
            {
              key: (point: TrendPoint) => point.series,
              value: (point: TrendPoint) => formatSpend(point.quota),
            },
          ],
        },
      },
      axes: [
        {
          orient: 'bottom',
          label: { style: axisText, autoHide: true, autoLimit: true },
          tick: { visible: false },
        },
        {
          orient: 'left',
          label: {
            formatMethod: (value: number | string) =>
              formatSpend(Number(value)),
            style: axisText,
          },
          grid: { visible: true, style: { lineDash: [3, 3] } },
        },
      ],
    }
    // The colours are read off the page, once the theme has settled there.
  }, [trend, t, themeReady])

  return (
    <section
      aria-labelledby='org-usage-trend-title'
      className='bg-card rounded-xl border'
    >
      <div className='flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3'>
        <h3 id='org-usage-trend-title' className='text-sm font-semibold'>
          {t('Spend over time')}
        </h3>
        <Tabs
          value={bucket}
          onValueChange={(next) => setBucket(next as OrgUsageTrendBucket)}
        >
          <TabsList aria-label={t('Time unit')}>
            {USAGE_TREND_BUCKETS.map((choice) => (
              <TabsTrigger key={choice} value={choice}>
                {trendBucketLabel(t, choice)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>
      {query.isError ? (
        <ErrorState
          description={t('Could not load the trend.')}
          onRetry={() => void query.refetch()}
        />
      ) : !trend ? (
        <div className='p-4'>
          <Skeleton className='h-72 rounded-lg' />
        </div>
      ) : trend.series.length === 0 ? (
        <p className='text-muted-foreground px-4 py-10 text-center text-sm'>
          {t('No usage in this period')}
        </p>
      ) : (
        <div
          role='img'
          aria-label={t('Spend over time, a line per {{group}}', {
            group: usageGroupLabel(t, groupBy).toLowerCase(),
          })}
          className='h-72 p-2 sm:h-80'
        >
          {spec && (
            <VChart
              key={`org-usage-trend-${resolvedTheme}`}
              spec={{
                ...spec,
                theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                background: 'transparent',
              }}
              option={VCHART_OPTION}
            />
          )}
        </div>
      )}
    </section>
  )
}
