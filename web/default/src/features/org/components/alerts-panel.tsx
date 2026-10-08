// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { BellRing, Check, CircleSlash, Settings2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestamp } from '@/lib/format'
import { Button } from '@/components/ui/button'
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
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  fetchOrgAlerts,
  handleOrgAlert,
  orgQueryKeys,
  type OrgAlertsParams,
} from '../api'
import { canManageOrg } from '../hooks/use-org-membership'
import { alertStateLabel, alertSummary, alertTitle } from '../lib/alerts'
import { seesOwnAlertsOnly } from '../lib/reports'
import type { OrgAlert, OrgAlertState, OrgMembership } from '../types'
import { AlertSettingsDialog } from './alert-settings-dialog'
import { Pager } from './pager'

/** How many alerts a page of the list holds. */
const ALERTS_PER_PAGE = 20

const STATE_VARIANT: Record<OrgAlertState, StatusVariant> = {
  '': 'warning',
  handled: 'success',
  false_alarm: 'neutral',
}

type AlertsTableProps = {
  alerts: OrgAlert[]
  /** Marks an alert; left out for a viewer who may only read the list. */
  onMark?: (alert: OrgAlert, state: 'handled' | 'false_alarm') => void
  /** The alert being marked; its buttons wait. */
  busyId: number | null
}

/** The alerts of one page: what happened, to which key, and what was done about it. */
function AlertsTable({ alerts, onMark, busyId }: AlertsTableProps) {
  const { t } = useTranslation()
  return (
    <div className='bg-card overflow-x-auto rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='px-3'>{t('Time')}</TableHead>
            <TableHead>{t('Alert')}</TableHead>
            <TableHead>{t('Key')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            {onMark && (
              <TableHead className='px-3 text-right'>{t('Actions')}</TableHead>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {alerts.map((alert) => {
            const busy = busyId === alert.id
            const keyName =
              alert.detail?.key || t('A key that no longer exists')
            return (
              <TableRow key={alert.id}>
                <TableCell className='text-muted-foreground px-3 text-xs whitespace-nowrap tabular-nums'>
                  {formatTimestamp(alert.created_time)}
                </TableCell>
                <TableCell className='max-w-md min-w-64 whitespace-normal'>
                  <div className='font-medium'>{alertTitle(t, alert)}</div>
                  <div className='text-muted-foreground text-xs break-words'>
                    {alertSummary(t, alert)}
                  </div>
                </TableCell>
                <TableCell>
                  <div>{keyName}</div>
                  <div className='text-muted-foreground text-xs'>
                    {[
                      alert.holder || t('Someone who has left'),
                      alert.department,
                    ]
                      .filter(Boolean)
                      .join(' · ')}
                  </div>
                </TableCell>
                <TableCell>
                  <StatusBadge
                    label={alertStateLabel(t, alert.state)}
                    variant={STATE_VARIANT[alert.state] ?? 'neutral'}
                    copyable={false}
                  />
                  {alert.state !== '' && (
                    <div className='text-muted-foreground text-xs'>
                      {[alert.acked_by_name, formatTimestamp(alert.acked_time)]
                        .filter(Boolean)
                        .join(' · ')}
                    </div>
                  )}
                </TableCell>
                {onMark && (
                  <TableCell className='px-3 text-right whitespace-nowrap'>
                    {alert.state !== 'handled' && (
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={busy}
                        onClick={() => onMark(alert, 'handled')}
                        aria-label={t(
                          'Mark “{{alert}}” on {{name}} as handled',
                          { alert: alertTitle(t, alert), name: keyName }
                        )}
                      >
                        <Check aria-hidden='true' />
                        {t('Handled')}
                      </Button>
                    )}
                    {alert.state !== 'false_alarm' && (
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={busy}
                        onClick={() => onMark(alert, 'false_alarm')}
                        aria-label={t(
                          'Mark “{{alert}}” on {{name}} as a false alarm',
                          { alert: alertTitle(t, alert), name: keyName }
                        )}
                      >
                        <CircleSlash aria-hidden='true' />
                        {t('False alarm')}
                      </Button>
                    )}
                  </TableCell>
                )}
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}

/**
 * The alerts of the organization (Enterprise Org PRD §4): warnings that a key
 * is running out of what it was given, and reports of unusual use. Every
 * member has a list, which the backend has already cut: a role that reads
 * alerts sees all of them, or the ones raised in its departments; everyone
 * else the warnings on the keys they hold themselves (D47). Marking an alert
 * and changing the settings behind them is the owner's and the admins'.
 */
export function AlertsPanel({ membership }: { membership: OrgMembership }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const runs = canManageOrg(membership)
  const ownOnly = seesOwnAlertsOnly(membership)
  const [openOnly, setOpenOnly] = useState(true)
  const [page, setPage] = useState(1)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [busyId, setBusyId] = useState<number | null>(null)

  const params: OrgAlertsParams = {
    p: page,
    page_size: ALERTS_PER_PAGE,
    ...(openOnly ? { state: 'open' as const } : {}),
  }
  const query = useQuery({
    queryKey: orgQueryKeys.alertList(params),
    queryFn: () => fetchOrgAlerts(params),
    placeholderData: keepPreviousData,
  })
  const alerts = query.data?.items ?? []
  const total = query.data?.total ?? 0

  const mark = async (alert: OrgAlert, state: 'handled' | 'false_alarm') => {
    setBusyId(alert.id)
    try {
      const res = await handleOrgAlert(alert.id, state)
      if (res.success) {
        toast.success(
          state === 'handled'
            ? t('Marked as handled')
            : t('Marked as a false alarm')
        )
        await queryClient.invalidateQueries({
          queryKey: orgQueryKeys.alerts(),
        })
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div className='grid gap-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <Tabs
          value={openOnly ? 'open' : 'all'}
          onValueChange={(next) => {
            setOpenOnly(next === 'open')
            setPage(1)
          }}
        >
          <TabsList aria-label={t('Which alerts to show')}>
            <TabsTrigger value='open'>{t('Unresolved')}</TabsTrigger>
            <TabsTrigger value='all'>{t('All alerts')}</TabsTrigger>
          </TabsList>
        </Tabs>
        {runs && (
          <Button
            variant='outline'
            size='sm'
            onClick={() => setSettingsOpen(true)}
          >
            <Settings2 aria-hidden='true' />
            {t('Alert settings')}
          </Button>
        )}
      </div>

      <p className='text-muted-foreground text-sm'>
        {ownOnly
          ? t(
              'These are the warnings about the keys you hold: a key that has used most or all of what it was given, or was refused a request it could not pay for. An administrator of your organization can give a key more.'
            )
          : t(
              'A warning shows up within about a minute of the request that crossed the line; unusual usage is looked for every 10 minutes. An alert never blocks a key — freezing it or giving it a new value is done on the organization keys page.'
            )}
      </p>
      {!ownOnly && membership.role_scope === 'dept' && (
        <p className='text-muted-foreground text-sm'>
          {t(
            'You see the alerts raised on keys held in the departments you manage.'
          )}
        </p>
      )}

      {query.isError ? (
        <ErrorState
          description={t('Could not load the alerts.')}
          onRetry={() => void query.refetch()}
        />
      ) : !query.data ? (
        <div className='space-y-3'>
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className='h-12 rounded-xl' />
          ))}
        </div>
      ) : alerts.length === 0 ? (
        <EmptyState
          icon={BellRing}
          title={openOnly ? t('No unresolved alerts') : t('No alerts yet')}
          description={
            openOnly
              ? t('Nothing is waiting to be looked at.')
              : ownOnly
                ? t(
                    'A warning shows up here when one of your keys runs low on what it was given.'
                  )
                : t(
                    'An alert is raised when a key runs low on what it was given, or is used in a way that is unusual for it.'
                  )
          }
          bordered
        />
      ) : (
        <AlertsTable
          alerts={alerts}
          onMark={runs ? (alert, state) => void mark(alert, state) : undefined}
          busyId={busyId}
        />
      )}
      <Pager
        page={page}
        pageSize={ALERTS_PER_PAGE}
        total={total}
        onPageChange={setPage}
      />

      {runs && (
        <AlertSettingsDialog
          open={settingsOpen}
          onClose={() => setSettingsOpen(false)}
        />
      )}
    </div>
  )
}
