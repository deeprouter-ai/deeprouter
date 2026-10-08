// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { BarChart3, Bot, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { fetchOrgUsage, orgQueryKeys, type OrgUsageParams } from '../api'
import {
  drawsTrend,
  formatSpend,
  type OrgPeriod,
  periodBounds,
  periodIsValid,
  USAGE_GROUPINGS,
  usageGoneLabel,
  usageGroupLabel,
  usageRowName,
  usageShare,
} from '../lib/usage'
import type {
  OrgMembership,
  OrgUsageFigures,
  OrgUsageGroupBy,
  OrgUsageReport,
} from '../types'
import { OrgSelect } from './org-select'
import { PeriodSelect } from './period-select'
import { UsageTrend } from './usage-trend'

/** How many rows a long report shows before it is asked for the rest. */
const ROWS_SHOWN = 100

/** The choice of the department filter that narrows nothing down. */
const EVERY_DEPARTMENT = 0

/** The two numbers a report adds up to: the money, and the requests behind it. */
function UsageTotals({ total }: { total: OrgUsageFigures }) {
  const { t } = useTranslation()
  const figures = [
    { label: t('Spend'), value: formatSpend(total.quota) },
    { label: t('Requests'), value: formatNumber(total.requests) },
  ]
  return (
    <dl className='grid gap-3 sm:grid-cols-2'>
      {figures.map((figure) => (
        <div key={figure.label} className='bg-card rounded-xl border px-4 py-3'>
          <dt className='text-muted-foreground text-xs'>{figure.label}</dt>
          <dd className='text-2xl font-semibold tabular-nums'>
            {figure.value}
          </dd>
        </div>
      ))}
    </dl>
  )
}

type UsageTableProps = {
  report: OrgUsageReport
  /** How many rows to show; the rest wait behind a button. */
  limit: number
}

/**
 * The rows of a report: who or what, the requests, the money (PRD D44: usage
 * is read as what it cost, never as tokens), and the share of the total each
 * row is.
 */
function UsageTable({ report, limit }: UsageTableProps) {
  const { t } = useTranslation()
  const groupBy = report.group_by
  return (
    <div className='bg-card overflow-x-auto rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='px-3'>
              {usageGroupLabel(t, groupBy)}
            </TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Spend')}</TableHead>
            <TableHead className='w-40 px-3'>{t('Share of spend')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {report.rows.slice(0, limit).map((row) => {
            const share = usageShare(row, report)
            return (
              <TableRow key={`${row.id}:${row.name}`}>
                <TableCell className='px-3'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span className='font-medium'>
                      {usageRowName(t, groupBy, row)}
                    </span>
                    {row.is_service && (
                      <Badge variant='outline'>
                        <Bot aria-hidden='true' />
                        {t('Service account')}
                      </Badge>
                    )}
                    {row.gone && (
                      <Badge variant='secondary'>
                        {usageGoneLabel(t, groupBy)}
                      </Badge>
                    )}
                  </div>
                  {row.used_by.length > 0 && (
                    <div className='text-muted-foreground text-xs'>
                      {t('Used by {{names}}', {
                        names: row.used_by
                          .map((name) => name || t('Someone who has left'))
                          .join(t(', ')),
                      })}
                    </div>
                  )}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatNumber(row.requests)}
                </TableCell>
                <TableCell className='text-right font-medium tabular-nums'>
                  {formatSpend(row.quota)}
                </TableCell>
                <TableCell className='px-3'>
                  <div className='flex items-center gap-2'>
                    {/* The bar draws the number beside it and says nothing
                        of its own. */}
                    <div
                      aria-hidden='true'
                      className='bg-muted h-1 flex-1 overflow-hidden rounded-full'
                    >
                      <div
                        className='bg-primary h-full'
                        style={{ width: `${share}%` }}
                      />
                    </div>
                    <span className='text-muted-foreground w-12 text-right text-xs tabular-nums'>
                      {`${share.toFixed(1)}%`}
                    </span>
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}

/**
 * The usage report of the organization (Enterprise Org PRD §6): what was spent
 * over a period, cut by department, model, member or key. The backend decides
 * how much of the company the viewer is shown — all of it, or the departments
 * they manage — and says so in its answer; this only asks, and never filters
 * rows itself.
 */
export function UsageReport({ membership }: { membership: OrgMembership }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  // A manager of one department would get a single row by department.
  const [groupBy, setGroupBy] = useState<OrgUsageGroupBy>(
    membership.role_scope === 'org' ? 'department' : 'member'
  )
  const [period, setPeriod] = useState<OrgPeriod>({ preset: 'last30' })
  const [chosenDepartmentId, setChosenDepartmentId] = useState(EVERY_DEPARTMENT)
  const [showsAll, setShowsAll] = useState(false)

  const params: OrgUsageParams = {
    group_by: groupBy,
    ...periodBounds(period, new Date()),
    ...(chosenDepartmentId !== EVERY_DEPARTMENT
      ? { department_id: chosenDepartmentId }
      : {}),
  }
  const query = useQuery({
    queryKey: orgQueryKeys.usage(params),
    queryFn: () => fetchOrgUsage(params),
    enabled: periodIsValid(period),
    // The last report stays on screen while the next one is fetched.
    placeholderData: keepPreviousData,
  })
  const report = query.data
  const departments = report?.departments ?? []

  return (
    <div className='grid gap-4'>
      <div className='flex flex-wrap items-center gap-2'>
        <span id='org-usage-group-by' className='text-muted-foreground text-sm'>
          {t('Group by')}
        </span>
        <Tabs
          value={groupBy}
          onValueChange={(next) => setGroupBy(next as OrgUsageGroupBy)}
        >
          <TabsList aria-labelledby='org-usage-group-by'>
            {USAGE_GROUPINGS.map((grouping) => (
              <TabsTrigger key={grouping} value={grouping}>
                {usageGroupLabel(t, grouping)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        {/* On a phone the filters take the line under the groupings. */}
        <div className='flex w-full flex-wrap items-center gap-2 sm:w-auto sm:flex-1 sm:justify-end'>
          <PeriodSelect
            id='org-usage-period'
            presets={[
              'today',
              'last7',
              'last30',
              'month',
              'last-month',
              'custom',
            ]}
            value={period}
            onChange={setPeriod}
          />
          {/* A filter with one choice narrows nothing down and is left out. */}
          {departments.length > 1 && (
            <div className='min-w-0 flex-1 sm:w-44 sm:flex-none'>
              <Label htmlFor='org-usage-department' className='sr-only'>
                {t('Department')}
              </Label>
              <OrgSelect
                id='org-usage-department'
                options={[
                  { value: EVERY_DEPARTMENT, label: t('All departments') },
                  ...departments.map((department) => ({
                    value: department.id,
                    label: department.name,
                  })),
                ]}
                value={chosenDepartmentId}
                onChange={setChosenDepartmentId}
              />
            </div>
          )}
          <Button
            variant='outline'
            size='icon'
            aria-label={t('Refresh')}
            disabled={query.isFetching}
            // The report and the chart above it are asked for again together.
            onClick={() =>
              void queryClient.invalidateQueries({
                queryKey: orgQueryKeys.usageAll(),
              })
            }
          >
            <RefreshCw
              aria-hidden='true'
              className={query.isFetching ? 'animate-spin' : undefined}
            />
          </Button>
        </div>
      </div>

      {/* Said out loud, so a short report reads as a scope and not as a
          company that spent this little. */}
      {report?.scope === 'dept' && (
        <p className='text-muted-foreground text-sm'>
          {t('You see what was spent in the departments you manage.')}
        </p>
      )}

      {query.isError ? (
        <ErrorState
          description={t('Could not load the usage report.')}
          onRetry={() => void query.refetch()}
        />
      ) : !report ? (
        <div className='space-y-3'>
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className='h-12 rounded-xl' />
          ))}
        </div>
      ) : (
        <>
          <UsageTotals total={report.total} />
          {report.rows.length === 0 ? (
            <EmptyState
              icon={BarChart3}
              title={t('No usage in this period')}
              description={t(
                'What the organization’s keys spend shows up here, a few seconds after each request.'
              )}
              bordered
            />
          ) : (
            <>
              {/* Departments and models are drawn over time as well. */}
              {drawsTrend(groupBy) && (
                <UsageTrend
                  groupBy={groupBy}
                  period={period}
                  departmentId={
                    chosenDepartmentId !== EVERY_DEPARTMENT
                      ? chosenDepartmentId
                      : undefined
                  }
                />
              )}
              <UsageTable
                report={report}
                limit={showsAll ? report.rows.length : ROWS_SHOWN}
              />
              {!showsAll && report.rows.length > ROWS_SHOWN && (
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Showing the {{shown}} that spent the most, of {{total}}.',
                      { shown: ROWS_SHOWN, total: report.rows.length }
                    )}
                  </p>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => setShowsAll(true)}
                  >
                    {t('Show all')}
                  </Button>
                </div>
              )}
            </>
          )}
          <p className='text-muted-foreground text-xs'>
            {t(
              'Spend is what left the company wallet: refunds for failed tasks are taken off. Usage counts in the department a member was in when they spent it, so moving someone does not move their past usage.'
            )}
          </p>
        </>
      )}
    </div>
  )
}
