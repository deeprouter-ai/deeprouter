// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useId, useMemo, useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import { useTranslation } from 'react-i18next'
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
  usageRowName,
  USAGE_TREND_BUCKETS,
} from '../lib/usage'
import type {
  OrgUsageGroupBy,
  OrgUsageRow,
  OrgUsageTrendBucket,
} from '../types'

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
  /** What the lines are: one per department, model, member or key. */
  groupBy: OrgUsageGroupBy
  /** The period and the department of the report this is drawn for. */
  period: OrgPeriod
  departmentId?: number
  /** The one member, or the one key, whose line alone is drawn (PRD D49). */
  userId?: number
  tokenId?: number
}

/**
 * The usage report drawn over time (Enterprise Org PRD D46): what was spent
 * in each day, week, month or year of the period, with a line per department
 * or per model — or, opened from a row of the report, the line of that one
 * member or key (D49). It asks for the period and the department the report
 * is showing, and — like the report — is sent exactly the part of the company
 * the viewer may see. Days are the viewer's own days.
 */
export function UsageTrend({
  groupBy,
  period,
  departmentId,
  userId,
  tokenId,
}: UsageTrendProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const [bucket, setBucket] = useState<OrgUsageTrendBucket>('day')
  const titleId = useId()

  const params: OrgUsageTrendParams = {
    group_by: groupBy,
    bucket,
    timezone: viewerTimeZone(),
    ...periodBounds(period, new Date()),
    ...(departmentId ? { department_id: departmentId } : {}),
    ...(userId ? { user_id: userId } : {}),
    ...(tokenId ? { token_id: tokenId } : {}),
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
    <section aria-labelledby={titleId} className='bg-card rounded-xl border'>
      <div className='flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3'>
        <h3 id={titleId} className='text-sm font-semibold'>
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
          aria-label={
            // One member's or one key's line is named by the dialog around it.
            userId || tokenId
              ? t('Spend over time')
              : t('Spend over time, a line per {{group}}', {
                  group: usageGroupLabel(t, groupBy).toLowerCase(),
                })
          }
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

type UsageTrendDialogProps = {
  /** The row of the report to draw over time; null keeps the dialog closed. */
  row: OrgUsageRow | null
  /** What the row is — a member or a key — and the report's period and department. */
  groupBy: OrgUsageGroupBy
  period: OrgPeriod
  departmentId?: number
  onClose: () => void
}

/**
 * One row of the report drawn over time (Enterprise Org PRD D49): the line of
 * that one member or key, in the period and department the report is showing,
 * with the chart's own choice of time unit. The row's name is the title.
 */
export function UsageTrendDialog({
  row,
  groupBy,
  period,
  departmentId,
  onClose,
}: UsageTrendDialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog open={row !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className='sm:max-w-3xl'>
        {row && (
          <>
            <DialogHeader>
              <DialogTitle>{usageRowName(t, groupBy, row)}</DialogTitle>
              <DialogDescription>
                {groupBy === 'key'
                  ? t(
                      'What was spent with this key over the period the report shows.'
                    )
                  : t(
                      'What this member spent over the period the report shows.'
                    )}
                {/* Three keys called "Claude Code" are told apart by who used them. */}
                {row.used_by.length > 0 &&
                  ' ' +
                    t('Used by {{names}}', {
                      names: row.used_by
                        .map((name) => name || t('Someone who has left'))
                        .join(t(', ')),
                    })}
              </DialogDescription>
            </DialogHeader>
            <UsageTrend
              groupBy={groupBy}
              period={period}
              departmentId={departmentId}
              userId={groupBy === 'member' ? row.id : undefined}
              tokenId={groupBy === 'key' ? row.id : undefined}
            />
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
