// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { type ReactNode, useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import dayjs from '@/lib/dayjs'
import { viewerTimeZone } from '../lib/alerts'
import type { OrgReportsSection } from '../lib/reports'
import type { TrendPoint } from '../lib/usage'
import { OrgReportsPage } from '../reports'
import type {
  OrgAlert,
  OrgAlertSettings,
  OrgAuditLog,
  OrgMembership,
  OrgUsageReport,
  OrgUsageRow,
  OrgUsageTrend,
} from '../types'
import { membershipOf } from './fixtures'

// Enterprise Org P9 (meta-repo docs/enterprise-org-prd.md §6): the "Reports &
// alerts" page. The backend decides how much of the company each viewer is
// sent; these tests pin what the page asks it for, how it shows the answer,
// and which of its three sections — and which buttons — each kind of member
// is offered.

const api = vi.hoisted(() => ({
  fetchOrgMembership: vi.fn(),
  fetchOrgUsage: vi.fn(),
  fetchOrgUsageTrend: vi.fn(),
  fetchOrgAlerts: vi.fn(),
  handleOrgAlert: vi.fn(),
  fetchOrgAlertSettings: vi.fn(),
  updateOrgAlertSettings: vi.fn(),
  fetchOrgAuditLogs: vi.fn(),
}))
const mockToast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
/** What the chart was last given to draw; the real one needs a canvas. */
const chart = vi.hoisted(() => ({
  spec: null as Record<string, unknown> | null,
}))

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  ...api,
}))
vi.mock('sonner', () => ({ toast: mockToast }))
vi.mock('@visactor/react-vchart', () => ({
  VChart: ({ spec }: { spec: Record<string, unknown> }) => {
    chart.spec = spec
    return <div data-testid='chart' />
  },
}))
vi.mock('@/lib/use-chart-theme', () => ({
  useChartTheme: () => ({ resolvedTheme: 'light', themeReady: true }),
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: { id: 1 } } }),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, unknown>) =>
      key.replace(/{{(\w+)}}/g, (_, name: string) =>
        String(values?.[name] ?? '')
      ),
    i18n: { language: 'en' },
  }),
}))
// The layout pulls in the whole console shell; the page only needs its slots.
vi.mock('@/components/layout', () => {
  const Slot = ({ children }: { children?: ReactNode }) => <>{children}</>
  const Layout = ({ children }: { children?: ReactNode }) => (
    <div>{children}</div>
  )
  return {
    SectionPageLayout: Object.assign(Layout, {
      Title: Slot,
      Actions: Slot,
      Content: Slot,
    }),
  }
})

/** A custom role that holds exactly the given primitives across the organization. */
function customRole(role: string, permissions: string[]): OrgMembership {
  return membershipOf('staff', { role, role_scope: 'org', permissions })
}

/** A report row with the given fields. */
function usageRow(over: Partial<OrgUsageRow>): OrgUsageRow {
  return {
    id: 1,
    name: 'row',
    is_service: false,
    gone: false,
    used_by: [],
    requests: 1,
    quota: 500000,
    ...over,
  }
}

const DEPARTMENTS = [
  { id: 10, name: 'General' },
  { id: 11, name: 'Sales' },
  { id: 12, name: 'Product' },
]

/** The company's usage by department: Sales $6, Product $3, General $1. */
const byDepartment: OrgUsageReport = {
  group_by: 'department',
  scope: 'org',
  departments: DEPARTMENTS,
  total: {
    requests: 1587,
    quota: 5000000,
  },
  rows: [
    usageRow({
      id: 11,
      name: 'Sales',
      requests: 900,
      quota: 3000000,
    }),
    usageRow({
      id: 12,
      name: 'Product',
      requests: 600,
      quota: 1500000,
    }),
    usageRow({
      id: 10,
      name: 'General',
      requests: 87,
      quota: 500000,
    }),
  ],
}

/** The same usage by member: a person, a service account and someone removed. */
const byMember: OrgUsageReport = {
  ...byDepartment,
  group_by: 'member',
  rows: [
    usageRow({ id: 3, name: 'sally', quota: 3000000 }),
    usageRow({
      id: 4,
      name: 'CI Pipeline',
      is_service: true,
      quota: 1500000,
    }),
    usageRow({ id: 5, name: 'Gone Gary', gone: true, quota: 500000 }),
  ],
}

/** …and by key: one that changed hands, one deleted since. */
const byKey: OrgUsageReport = {
  ...byDepartment,
  group_by: 'key',
  rows: [
    usageRow({
      id: 100,
      name: 'Claude Code',
      used_by: ['sally', 'bo'],
      quota: 4500000,
    }),
    usageRow({
      id: 101,
      name: 'Old poster key',
      gone: true,
      used_by: ['sally'],
      quota: 500000,
    }),
  ],
}

/** …and by model. */
const byModel: OrgUsageReport = {
  ...byDepartment,
  group_by: 'model',
  rows: [
    usageRow({ id: 0, name: 'claude-sonnet-5', quota: 4000000 }),
    usageRow({ id: 0, name: 'gpt-4o-mini', quota: 1000000 }),
  ],
}

const REPORTS: Record<string, OrgUsageReport> = {
  department: byDepartment,
  member: byMember,
  key: byKey,
  model: byModel,
}

/** The same usage over three days: a line per department… */
const trendByDepartment: OrgUsageTrend = {
  group_by: 'department',
  bucket: 'day',
  scope: 'org',
  buckets: ['2026-10-05', '2026-10-06', '2026-10-07'],
  total: 5000000,
  series: [
    {
      id: 11,
      name: 'Sales',
      other: false,
      quota: 3000000,
      points: [1000000, 500000, 1500000],
    },
    {
      id: 12,
      name: 'Product',
      other: false,
      quota: 1500000,
      points: [0, 1500000, 0],
    },
    {
      id: 10,
      name: 'General',
      other: false,
      quota: 500000,
      points: [500000, 0, 0],
    },
  ],
}

/** …and per model, with the small ones summed into one line. */
const trendByModel: OrgUsageTrend = {
  ...trendByDepartment,
  group_by: 'model',
  series: [
    {
      id: 0,
      name: 'claude-sonnet-5',
      other: false,
      quota: 4000000,
      points: [1500000, 2000000, 500000],
    },
    {
      id: 0,
      name: '',
      other: true,
      quota: 1000000,
      points: [0, 0, 1000000],
    },
  ],
}

const TRENDS: Record<string, OrgUsageTrend> = {
  department: trendByDepartment,
  model: trendByModel,
}

/** An alert on sally's key, as `GET /api/org/alerts` lists one. */
function alertOf(over: Partial<OrgAlert>): OrgAlert {
  return {
    id: 1,
    rule: 'quota',
    level: 80,
    key_id: 100,
    holder_id: 3,
    holder: 'sally',
    department_id: 11,
    department: 'Sales',
    detail: { key: 'Design tools', used: 4000000, limit: 5000000 },
    created_time: 1790000000,
    state: '',
    acked_by: 0,
    acked_by_name: '',
    acked_time: 0,
    ...over,
  }
}

const quotaWarning = alertOf({ id: 7 })
const spike = alertOf({
  id: 8,
  rule: 'spike',
  level: 0,
  holder: 'bo',
  detail: { key: 'Claude Code', spent: 2500000, daily_average: 250000 },
})
const handledAlert = alertOf({
  id: 9,
  rule: 'new_ip',
  level: 0,
  detail: { key: 'Nightly build', ips: ['203.0.113.5'], known_ips: 2 },
  state: 'handled',
  acked_by: 2,
  acked_by_name: 'Ada Admin',
  acked_time: 1790003600,
})

/** A page of the alert list. */
function alertPage(items: OrgAlert[], total = items.length) {
  return { page: 1, page_size: 20, total, items }
}

const SETTINGS: OrgAlertSettings = {
  warn_at: [80, 100],
  spike_multiple: 5,
  off_hours_percent: 50,
  min_spend: 500000,
  work_hours: null,
}

/** A record of the audit log. */
function recordOf(over: Partial<OrgAuditLog>): OrgAuditLog {
  return {
    id: 1,
    actor_user_id: 1,
    actor: 'Fiona Founder',
    action: 'department.rename',
    target_type: 'department',
    target_id: 11,
    target: 'Field Sales',
    detail: { before: { name: 'Sales' }, after: { name: 'Field Sales' } },
    ip: '203.0.113.7',
    created_time: 1790000000,
    ...over,
  }
}

const renamed = recordOf({ id: 31 })
const invited = recordOf({
  id: 32,
  actor: 'Ada Admin',
  action: 'member.invite',
  target_type: 'invite',
  target: '',
  detail: {
    after: {
      role_id: 4,
      role: 'staff',
      department_id: 11,
      department: 'Sales',
    },
  },
  ip: '',
})

/** A page of the audit log. */
function auditPage(items: OrgAuditLog[], total = items.length) {
  return { page: 1, page_size: 20, total, items }
}

/** The page with the section it shows held in state, as the route holds it in the address. */
function Harness({ initial }: { initial?: OrgReportsSection }) {
  const [section, setSection] = useState(initial)
  return <OrgReportsPage section={section} onSectionChange={setSection} />
}

/** Renders the page as the given member and waits until its tabs are there. */
async function renderPage(
  membership: OrgMembership = membershipOf('owner'),
  section?: OrgReportsSection
) {
  api.fetchOrgMembership.mockResolvedValue(membership)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const view = render(
    <QueryClientProvider client={queryClient}>
      <Harness initial={section} />
    </QueryClientProvider>
  )
  await screen.findByText('Acme')
  return view
}

/** The sections the page offers, in order. */
function sectionTabs(): string[] {
  const list = screen.getAllByRole('tablist')[0]
  return within(list)
    .getAllByRole('tab')
    .map((tab) => tab.textContent ?? '')
}

/** The table row that mentions the given text. */
function rowOf(text: string): HTMLElement {
  const row = screen.getByText(text).closest('tr')
  if (!row) throw new Error(`no table row for ${text}`)
  return row
}

/** What one row of a list says: its pieces of text, in order, with bars between. */
function rowText(row: HTMLElement): string {
  const pieces: string[] = []
  const walker = document.createTreeWalker(row, NodeFilter.SHOW_TEXT)
  while (walker.nextNode()) {
    const piece = walker.currentNode.textContent?.trim()
    if (piece) pieces.push(piece)
  }
  return pieces.join(' | ')
}

/** Picks one choice of a dropdown, the way a person would. */
async function choose(label: string, option: string) {
  await userEvent.click(screen.getByRole('combobox', { name: label }))
  await userEvent.click(await screen.findByRole('option', { name: option }))
  await waitFor(() => expect(screen.queryByRole('option')).toBeNull())
}

/** What the last call of a mocked request was made with. */
function lastCall(mock: ReturnType<typeof vi.fn>): unknown {
  return mock.mock.calls[mock.mock.calls.length - 1]?.[0]
}

/** Midnight, the viewer's time, so many days before today, in Unix seconds. */
function daysAgo(days: number): number {
  return dayjs().startOf('day').subtract(days, 'day').unix()
}

/** What the chart is drawing: its points, and its lines in order. */
function drawn(): { points: TrendPoint[]; lines: string[] } {
  const spec = chart.spec as {
    data: { values: TrendPoint[] }[]
    color?: { domain: string[] }
  } | null
  if (!spec) throw new Error('no chart was drawn')
  const points = spec.data[0].values
  return { points, lines: [...new Set(points.map((point) => point.series))] }
}

/** The choices of the chart's time unit, with the one that is on. */
function timeUnits(): { names: string[]; on: string } {
  const tabs = within(
    screen.getByRole('tablist', { name: 'Time unit' })
  ).getAllByRole('tab')
  return {
    names: tabs.map((tab) => tab.textContent ?? ''),
    on:
      tabs.find((tab) => tab.getAttribute('aria-selected') === 'true')
        ?.textContent ?? '',
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  chart.spec = null
  api.fetchOrgUsage.mockImplementation(
    async (params: { group_by: string }) => REPORTS[params.group_by]
  )
  api.fetchOrgUsageTrend.mockImplementation(
    async (params: { group_by: string; bucket: string }) => ({
      ...TRENDS[params.group_by],
      bucket: params.bucket,
    })
  )
  api.fetchOrgAlerts.mockResolvedValue(alertPage([spike, quotaWarning]))
  api.handleOrgAlert.mockResolvedValue({ success: true })
  api.fetchOrgAlertSettings.mockResolvedValue(SETTINGS)
  api.updateOrgAlertSettings.mockImplementation(
    async (settings: OrgAlertSettings) => ({ success: true, data: settings })
  )
  api.fetchOrgAuditLogs.mockResolvedValue(auditPage([invited, renamed]))
})

describe('the sections each kind of member is offered', () => {
  it('are all three for the owner, the admins and a read-only member', async () => {
    for (const role of ['owner', 'admin', 'readonly'] as const) {
      const view = await renderPage(membershipOf(role))
      expect(sectionTabs(), role).toEqual(['Usage', 'Alerts', 'Audit log'])
      view.unmount()
    }
  })

  it('leave out the audit log for a manager, whose reach is their departments', async () => {
    await renderPage(membershipOf('manager'))
    expect(sectionTabs()).toEqual(['Usage', 'Alerts'])
  })

  it('give a role that reads usage or the audit log that tab, beside the alerts every member has', async () => {
    for (const [permission, tabs] of [
      ['usage.read', ['Usage', 'Alerts']],
      ['alert.read', ['Alerts']],
      ['audit.read', ['Alerts', 'Audit log']],
    ] as const) {
      const view = await renderPage(customRole('One thing', [permission]))
      expect(sectionTabs(), permission).toEqual(tabs)
      view.unmount()
    }
    // Only the section on show was asked for: usage once, the alerts by the
    // two roles that open on them, the audit log by nobody.
    expect(api.fetchOrgUsage).toHaveBeenCalledTimes(1)
    expect(api.fetchOrgAlerts).toHaveBeenCalledTimes(2)
    expect(api.fetchOrgAuditLogs).not.toHaveBeenCalled()
  })

  it('are the alerts alone for staff', async () => {
    api.fetchOrgAlerts.mockResolvedValue(alertPage([quotaWarning]))
    await renderPage(membershipOf('staff'))
    expect(sectionTabs()).toEqual(['Alerts'])
    await screen.findByText('80% of quota used')
    expect(api.fetchOrgUsage).not.toHaveBeenCalled()
    expect(api.fetchOrgAuditLogs).not.toHaveBeenCalled()
  })

  it('open on the section the address names, and only fetch that one', async () => {
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('Sudden rise in spending')
    expect(api.fetchOrgUsage).not.toHaveBeenCalled()
    expect(api.fetchOrgAuditLogs).not.toHaveBeenCalled()
  })

  it('fall back to the first one when the address names a section the viewer may not read', async () => {
    await renderPage(customRole('Finance Ops', ['usage.read']), 'audit')
    await screen.findByText('Sales')
    expect(api.fetchOrgAuditLogs).not.toHaveBeenCalled()
  })

  it('switch when a tab is pressed', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByText('Sales')
    await userEvent.click(screen.getByRole('tab', { name: 'Audit log' }))
    await screen.findByText('Renamed a department')
    await userEvent.click(screen.getByRole('tab', { name: 'Alerts' }))
    await screen.findByText('Sudden rise in spending')
  })
})

describe('the usage report', () => {
  it('asks for the last 30 days by department, and reads usage as money', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByText('Sales')
    expect(lastCall(api.fetchOrgUsage)).toEqual({
      group_by: 'department',
      start_timestamp: daysAgo(29),
    })

    // What it adds up to, number first — and no token count anywhere (PRD D44).
    expect(screen.getByText('$10')).toBeInTheDocument()
    expect(screen.getByText('1,587')).toBeInTheDocument()
    expect(screen.queryByText(/27,000|tokens/i)).toBeNull()
    // The groupings, department and model first.
    const groupings = screen.getAllByRole('tablist')[1]
    expect(
      within(groupings)
        .getAllByRole('tab')
        .map((tab) => tab.textContent)
    ).toEqual(['Department', 'Model', 'Member', 'Key'])
    // The rows, in the order the backend sent them: the biggest spender first.
    expect(
      screen
        .getAllByRole('row')
        .slice(1)
        .map((row) => rowText(row))
    ).toEqual([
      'Sales | 900 | $6 | 60.0%',
      'Product | 600 | $3 | 30.0%',
      'General | 87 | $1 | 10.0%',
    ])
  })

  it('starts a department-scoped viewer on members, and says the report is a part', async () => {
    api.fetchOrgUsage.mockImplementation(
      async (params: { group_by: string }) => ({
        ...REPORTS[params.group_by],
        scope: 'dept',
        departments: [{ id: 11, name: 'Sales' }],
      })
    )
    await renderPage(membershipOf('manager'))
    await screen.findByText('sally')
    expect(lastCall(api.fetchOrgUsage)).toMatchObject({ group_by: 'member' })
    expect(
      screen.getByText('You see what was spent in the departments you manage.')
    ).toBeInTheDocument()
    // One department to choose from narrows nothing down: no filter.
    expect(screen.queryByRole('combobox', { name: 'Department' })).toBeNull()
  })

  it('does not say so to a viewer who is shown the whole company', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByText('Sales')
    expect(
      screen.queryByText(
        'You see what was spent in the departments you manage.'
      )
    ).toBeNull()
  })

  it('cuts the same usage by member, key and model', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByText('Sales')

    await userEvent.click(screen.getByRole('tab', { name: 'Member' }))
    await screen.findByText('CI Pipeline')
    expect(lastCall(api.fetchOrgUsage)).toMatchObject({ group_by: 'member' })
    // A service account and someone who was removed are both in the report,
    // marked as what they are.
    expect(rowText(rowOf('CI Pipeline'))).toContain('Service account')
    expect(rowText(rowOf('Gone Gary'))).toContain(
      'Removed from the organization'
    )
    expect(rowText(rowOf('sally'))).not.toMatch(/Service account|Removed/)

    await userEvent.click(screen.getByRole('tab', { name: 'Key' }))
    await screen.findByText('Claude Code')
    expect(lastCall(api.fetchOrgUsage)).toMatchObject({ group_by: 'key' })
    // Two keys can share a name; who used each tells them apart.
    expect(rowText(rowOf('Claude Code'))).toContain('Used by sally, bo')
    expect(rowText(rowOf('Old poster key'))).toContain('Deleted since')

    await userEvent.click(screen.getByRole('tab', { name: 'Model' }))
    await screen.findByText('claude-sonnet-5')
    expect(lastCall(api.fetchOrgUsage)).toMatchObject({ group_by: 'model' })
    expect(rowText(rowOf('gpt-4o-mini'))).toContain('$2')
  })

  it('asks again for the period that is chosen', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByText('Sales')

    await choose('Period', 'Today')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsage)).toEqual({
        group_by: 'department',
        start_timestamp: daysAgo(0),
      })
    )
    await choose('Period', 'Last calendar month')
    const lastMonth = dayjs().startOf('month').subtract(1, 'month')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsage)).toEqual({
        group_by: 'department',
        start_timestamp: lastMonth.unix(),
        end_timestamp: lastMonth.endOf('month').unix(),
      })
    )
    // Hand-picked dates start out unset: nothing bounds the report yet, and
    // the two date fields — which were not there until now — are there to
    // set them.
    expect(screen.queryByRole('group', { name: 'Custom dates' })).toBeNull()
    await choose('Period', 'Custom dates')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsage)).toEqual({ group_by: 'department' })
    )
    const dates = screen.getByRole('group', { name: 'Custom dates' })
    expect(within(dates).getByText('First day')).toBeInTheDocument()
    expect(within(dates).getByText('Last day')).toBeInTheDocument()
  })

  it('takes hand-picked dates from the calendar, and does not ask for a period that ends before it starts', async () => {
    // Mid-October 2026, so that the calendar opens on a month whose first
    // half is in the past. Only the clock is faked; timers run as they do.
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 15, 12))
    try {
      await renderPage(membershipOf('owner'))
      await screen.findByText('Sales')
      await choose('Period', 'Custom dates')
      const dates = screen.getByRole('group', { name: 'Custom dates' })
      /** Picks a day of October 2026 in the first or the second date field. */
      const pick = async (field: 0 | 1, day: number) => {
        await userEvent.click(within(dates).getAllByRole('button')[field])
        const grid = await screen.findByRole('grid')
        await userEvent.click(
          within(grid).getByRole('button', {
            name: new RegExp(`October ${day}(st|nd|rd|th), 2026`),
          })
        )
        await userEvent.keyboard('{Escape}')
        await waitFor(() => expect(screen.queryByRole('grid')).toBeNull())
      }
      const local = (day: number, endOfDay = false) =>
        Math.floor(
          (endOfDay
            ? new Date(2026, 9, day, 23, 59, 59)
            : new Date(2026, 9, day)
          ).getTime() / 1000
        )

      await pick(0, 10)
      await waitFor(() =>
        expect(lastCall(api.fetchOrgUsage)).toEqual({
          group_by: 'department',
          start_timestamp: local(10),
        })
      )
      // A last day before the first: the page says so and asks for nothing,
      // neither the report nor the chart above it.
      const asked = api.fetchOrgUsage.mock.calls.length
      const drawnTimes = api.fetchOrgUsageTrend.mock.calls.length
      await pick(1, 5)
      expect(
        await within(dates).findByText('The last day is before the first.')
      ).toBeVisible()
      expect(api.fetchOrgUsage.mock.calls.length).toBe(asked)
      expect(api.fetchOrgUsageTrend.mock.calls.length).toBe(drawnTimes)

      await pick(1, 12)
      await waitFor(() =>
        expect(lastCall(api.fetchOrgUsage)).toEqual({
          group_by: 'department',
          start_timestamp: local(10),
          end_timestamp: local(12, true),
        })
      )
      expect(
        within(dates).queryByText('The last day is before the first.')
      ).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  it('narrows the report to one department, whatever the grouping', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByText('Sales')

    await choose('Department', 'Product')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsage)).toMatchObject({
        group_by: 'department',
        department_id: 12,
      })
    )
    await userEvent.click(screen.getByRole('tab', { name: 'Member' }))
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsage)).toMatchObject({
        group_by: 'member',
        department_id: 12,
      })
    )
    await choose('Department', 'All departments')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsage)).toEqual({
        group_by: 'member',
        start_timestamp: daysAgo(29),
      })
    )
  })

  it('asks again when it is refreshed', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByText('Sales')
    const before = api.fetchOrgUsage.mock.calls.length
    await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() =>
      expect(api.fetchOrgUsage.mock.calls.length).toBe(before + 1)
    )
  })

  it('says so when nothing was used, and still shows the totals', async () => {
    api.fetchOrgUsage.mockResolvedValue({
      ...byDepartment,
      total: { requests: 0, quota: 0 },
      rows: [],
    })
    await renderPage(membershipOf('owner'))
    await screen.findByText('No usage in this period')
    expect(screen.getByText('$0')).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()
  })

  it('offers a retry when the report cannot be loaded', async () => {
    api.fetchOrgUsage.mockRejectedValueOnce(new Error('503'))
    await renderPage(membershipOf('owner'))
    await screen.findByText('Could not load the usage report.')
    await userEvent.click(screen.getByRole('button', { name: /retry/i }))
    await screen.findByText('Sales')
  })

  it('shows the hundred biggest rows of a long report, and the rest on request', async () => {
    const many = Array.from({ length: 130 }, (_, i) =>
      usageRow({ id: i + 1, name: `member-${i + 1}`, quota: 1000 * (130 - i) })
    )
    api.fetchOrgUsage.mockResolvedValue({
      ...byMember,
      group_by: 'member',
      rows: many,
    })
    await renderPage(membershipOf('manager'))
    await screen.findByText('member-1')
    expect(screen.getAllByRole('row')).toHaveLength(101)
    expect(screen.queryByText('member-101')).toBeNull()
    expect(
      screen.getByText('Showing the 100 that spent the most, of 130.')
    ).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Show all' }))
    expect(screen.getAllByRole('row')).toHaveLength(131)
    expect(screen.getByText('member-130')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Show all' })).toBeNull()
  })
})

// Enterprise Org P10 (PRD D46): the usage report drawn over time. Both
// acceptance items of the card are answered by the backend — which lines, and
// whose usage; these pin what the page asks it for and that it draws the answer.
describe('the usage trend', () => {
  it('draws the report over time above the table: by day, a line per department, in money', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByTestId('chart')
    // The report's own question, cut by day on the viewer's calendar.
    expect(lastCall(api.fetchOrgUsageTrend)).toEqual({
      group_by: 'department',
      bucket: 'day',
      timezone: viewerTimeZone(),
      start_timestamp: daysAgo(29),
    })
    expect(
      screen.getByRole('heading', { name: 'Spend over time' })
    ).toBeInTheDocument()
    expect(timeUnits()).toEqual({
      names: ['By day', 'By week', 'By month', 'By year'],
      on: 'By day',
    })
    expect(
      screen.getByRole('img', {
        name: 'Spend over time, a line per department',
      })
    ).toBeInTheDocument()
    // It comes before the table it is the other view of.
    const table = screen.getByRole('table')
    expect(
      screen.getByTestId('chart').compareDocumentPosition(table) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()

    // A line per department with a point per day, the biggest spender first.
    expect(drawn().lines).toEqual(['Sales', 'Product', 'General'])
    expect(drawn().points).toEqual([
      { bucket: '10-05', series: 'Sales', quota: 1000000 },
      { bucket: '10-05', series: 'Product', quota: 0 },
      { bucket: '10-05', series: 'General', quota: 500000 },
      { bucket: '10-06', series: 'Sales', quota: 500000 },
      { bucket: '10-06', series: 'Product', quota: 1500000 },
      { bucket: '10-06', series: 'General', quota: 0 },
      { bucket: '10-07', series: 'Sales', quota: 1500000 },
      { bucket: '10-07', series: 'Product', quota: 0 },
      { bucket: '10-07', series: 'General', quota: 0 },
    ])
    // Time runs along the bottom, and what was spent is read in money — on
    // the axis and when a point is pointed at — never shortened.
    const spec = chart.spec as {
      type: string
      xField: string
      yField: string
      seriesField: string
      axes: { label: { formatMethod?: (value: number) => string } }[]
      tooltip: {
        mark: { content: { value: (point: TrendPoint) => string }[] }
        dimension: { content: { value: (point: TrendPoint) => string }[] }
      }
    }
    expect(spec).toMatchObject({
      type: 'line',
      xField: 'bucket',
      yField: 'quota',
      seriesField: 'series',
    })
    expect(spec.axes[1].label.formatMethod?.(1500000)).toBe('$3')
    const pointed = { bucket: '10-05', series: 'Sales', quota: 6172750000 }
    expect(spec.tooltip.mark.content[0].value(pointed)).toBe('$12,345.5')
    expect(spec.tooltip.dimension.content[0].value(pointed)).toBe('$12,345.5')
  })

  it('is drawn by model too, with the line of the rest named', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByTestId('chart')
    await userEvent.click(screen.getByRole('tab', { name: 'Model' }))
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsageTrend)).toMatchObject({
        group_by: 'model',
        bucket: 'day',
      })
    )
    await waitFor(() =>
      expect(drawn().lines).toEqual(['claude-sonnet-5', 'Other'])
    )
    expect(
      screen.getByRole('img', { name: 'Spend over time, a line per model' })
    ).toBeInTheDocument()
  })

  it('is not drawn by member or by key', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByTestId('chart')
    const asked = api.fetchOrgUsageTrend.mock.calls.length
    for (const [grouping, row] of [
      ['Member', 'CI Pipeline'],
      ['Key', 'Claude Code'],
    ]) {
      await userEvent.click(screen.getByRole('tab', { name: grouping }))
      await screen.findByText(row)
      expect(screen.queryByTestId('chart'), grouping).toBeNull()
      expect(
        screen.queryByRole('heading', { name: 'Spend over time' }),
        grouping
      ).toBeNull()
    }
    expect(api.fetchOrgUsageTrend.mock.calls.length).toBe(asked)
    // Back on departments it is there again.
    await userEvent.click(screen.getByRole('tab', { name: 'Department' }))
    await screen.findByTestId('chart')
  })

  it('starts a department-scoped viewer without one, on members, and draws theirs when asked', async () => {
    api.fetchOrgUsage.mockImplementation(
      async (params: { group_by: string }) => ({
        ...REPORTS[params.group_by],
        scope: 'dept',
        departments: [{ id: 11, name: 'Sales' }],
      })
    )
    await renderPage(membershipOf('manager'))
    await screen.findByText('sally')
    expect(screen.queryByTestId('chart')).toBeNull()
    expect(api.fetchOrgUsageTrend).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('tab', { name: 'Model' }))
    await screen.findByTestId('chart')
    // What they are sent is the backend's to decide; the page names no
    // department for them.
    expect(lastCall(api.fetchOrgUsageTrend)).toEqual({
      group_by: 'model',
      bucket: 'day',
      timezone: viewerTimeZone(),
      start_timestamp: daysAgo(29),
    })
  })

  it('cuts time by the unit that is chosen', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByTestId('chart')
    for (const [name, bucket] of [
      ['By week', 'week'],
      ['By month', 'month'],
      ['By year', 'year'],
      ['By day', 'day'],
    ]) {
      await userEvent.click(screen.getByRole('tab', { name }))
      await waitFor(() =>
        expect(lastCall(api.fetchOrgUsageTrend)).toMatchObject({
          group_by: 'department',
          bucket,
        })
      )
      expect(timeUnits().on).toBe(name)
    }
    // Choosing a unit is the chart's business: the report was not asked again.
    expect(api.fetchOrgUsage).toHaveBeenCalledTimes(1)
  })

  it('names its buckets by the unit they are', async () => {
    api.fetchOrgUsageTrend.mockImplementation(
      async (params: { bucket: string }) => ({
        ...trendByDepartment,
        bucket: params.bucket,
        buckets: ['2026-10-05'],
        series: [{ ...trendByDepartment.series[0], points: [3000000] }],
      })
    )
    await renderPage(membershipOf('owner'))
    await screen.findByTestId('chart')
    expect(drawn().points.map((point) => point.bucket)).toEqual(['10-05'])
    await userEvent.click(screen.getByRole('tab', { name: 'By week' }))
    await waitFor(() =>
      expect(drawn().points.map((point) => point.bucket)).toEqual([
        '10-05 – 10-11',
      ])
    )
  })

  it('follows the period and the department of the report', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByTestId('chart')

    await choose('Period', 'Last calendar month')
    const lastMonth = dayjs().startOf('month').subtract(1, 'month')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsageTrend)).toEqual({
        group_by: 'department',
        bucket: 'day',
        timezone: viewerTimeZone(),
        start_timestamp: lastMonth.unix(),
        end_timestamp: lastMonth.endOf('month').unix(),
      })
    )
    await choose('Department', 'Product')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsageTrend)).toMatchObject({
        department_id: 12,
        start_timestamp: lastMonth.unix(),
      })
    )
    // Asked for the same things as the report, each time.
    expect(lastCall(api.fetchOrgUsage)).toEqual({
      group_by: 'department',
      start_timestamp: lastMonth.unix(),
      end_timestamp: lastMonth.endOf('month').unix(),
      department_id: 12,
    })
    await choose('Department', 'All departments')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgUsageTrend)).not.toHaveProperty(
        'department_id'
      )
    )
  })

  it('is asked for again together with the report when that is refreshed', async () => {
    await renderPage(membershipOf('owner'))
    await screen.findByTestId('chart')
    const reports = api.fetchOrgUsage.mock.calls.length
    const trends = api.fetchOrgUsageTrend.mock.calls.length
    await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => {
      expect(api.fetchOrgUsage.mock.calls.length).toBe(reports + 1)
      expect(api.fetchOrgUsageTrend.mock.calls.length).toBe(trends + 1)
    })
  })

  it('is left out when nothing was used', async () => {
    api.fetchOrgUsage.mockResolvedValue({
      ...byDepartment,
      total: { requests: 0, quota: 0 },
      rows: [],
    })
    await renderPage(membershipOf('owner'))
    await screen.findByText('No usage in this period')
    expect(
      screen.queryByRole('heading', { name: 'Spend over time' })
    ).toBeNull()
    expect(api.fetchOrgUsageTrend).not.toHaveBeenCalled()
  })

  it('offers a retry of its own when it cannot be loaded, and leaves the table standing', async () => {
    api.fetchOrgUsageTrend.mockRejectedValueOnce(new Error('503'))
    await renderPage(membershipOf('owner'))
    await screen.findByText('Could not load the trend.')
    expect(screen.queryByTestId('chart')).toBeNull()
    expect(rowText(rowOf('Sales'))).toContain('$6')
    await userEvent.click(screen.getByRole('button', { name: /retry/i }))
    await screen.findByTestId('chart')
    expect(screen.queryByText('Could not load the trend.')).toBeNull()
  })

  it('says so when the period turns out to hold no usage to draw', async () => {
    api.fetchOrgUsageTrend.mockResolvedValue({
      ...trendByDepartment,
      buckets: [],
      total: 0,
      series: [],
    })
    await renderPage(membershipOf('owner'))
    await screen.findByText('Sales')
    await screen.findByText('No usage in this period')
    expect(screen.queryByTestId('chart')).toBeNull()
  })
})

describe('the alert list', () => {
  it('asks for the alerts nobody has dealt with, and reads each one out', async () => {
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('Sudden rise in spending')
    expect(lastCall(api.fetchOrgAlerts)).toEqual({
      p: 1,
      page_size: 20,
      state: 'open',
    })
    expect(rowText(rowOf('Sudden rise in spending'))).toContain(
      '$5 in the last 24 hours, against $0.5 a day over the week before | Claude Code | bo · Sales | Unresolved'
    )
    expect(rowText(rowOf('80% of quota used'))).toContain(
      '$8 used of $10 | Design tools | sally · Sales | Unresolved'
    )
  })

  it('lists every alert on request, with who dealt with the ones that were', async () => {
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('Sudden rise in spending')
    api.fetchOrgAlerts.mockResolvedValue(alertPage([spike, handledAlert]))
    await userEvent.click(screen.getByRole('tab', { name: 'All alerts' }))
    await screen.findByText('Unfamiliar IP address')
    expect(lastCall(api.fetchOrgAlerts)).toEqual({ p: 1, page_size: 20 })
    const row = rowOf('Unfamiliar IP address')
    expect(rowText(row)).toContain('Handled | Ada Admin')
    // What is already "handled" can still be turned into a false alarm, and
    // nothing else.
    expect(within(row).queryByRole('button', { name: /as handled/ })).toBeNull()
    expect(
      within(row).getByRole('button', {
        name: 'Mark “Unfamiliar IP address” on Nightly build as a false alarm',
      })
    ).toBeInTheDocument()
  })

  it('lets the owner and the admins mark an alert, and asks for the list again', async () => {
    for (const role of ['owner', 'admin'] as const) {
      vi.clearAllMocks()
      api.fetchOrgAlerts.mockResolvedValue(alertPage([spike, quotaWarning]))
      const view = await renderPage(membershipOf(role), 'alerts')
      await screen.findByText('Sudden rise in spending')
      api.fetchOrgAlerts.mockResolvedValue(alertPage([quotaWarning]))
      await userEvent.click(
        screen.getByRole('button', {
          name: 'Mark “Sudden rise in spending” on Claude Code as handled',
        })
      )
      await waitFor(() =>
        expect(screen.queryByText('Sudden rise in spending')).toBeNull()
      )
      expect(api.handleOrgAlert).toHaveBeenCalledTimes(1)
      expect(api.handleOrgAlert).toHaveBeenCalledWith(8, 'handled')
      expect(mockToast.success).toHaveBeenCalledWith('Marked as handled')

      await userEvent.click(
        screen.getByRole('button', {
          name: 'Mark “80% of quota used” on Design tools as a false alarm',
        })
      )
      await waitFor(() =>
        expect(api.handleOrgAlert).toHaveBeenLastCalledWith(7, 'false_alarm')
      )
      expect(mockToast.success).toHaveBeenLastCalledWith(
        'Marked as a false alarm'
      )
      view.unmount()
    }
  })

  it('says nothing when the backend refuses the mark', async () => {
    api.handleOrgAlert.mockResolvedValue({ success: false })
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('Sudden rise in spending')
    const fetched = api.fetchOrgAlerts.mock.calls.length
    await userEvent.click(
      screen.getByRole('button', {
        name: 'Mark “Sudden rise in spending” on Claude Code as handled',
      })
    )
    await waitFor(() => expect(api.handleOrgAlert).toHaveBeenCalled())
    expect(mockToast.success).not.toHaveBeenCalled()
    expect(api.fetchOrgAlerts.mock.calls.length).toBe(fetched)
  })

  it('shows everyone else the list and nothing to press', async () => {
    for (const viewer of [
      membershipOf('manager'),
      membershipOf('readonly'),
      customRole('On call', ['alert.read']),
    ]) {
      const view = await renderPage(viewer, 'alerts')
      await screen.findByText('Sudden rise in spending')
      expect(
        screen.queryByRole('button', { name: /^Mark “/ }),
        viewer.role
      ).toBeNull()
      expect(
        screen.queryByRole('button', { name: 'Alert settings' }),
        viewer.role
      ).toBeNull()
      expect(screen.queryByText('Actions'), viewer.role).toBeNull()
      view.unmount()
    }
    expect(api.fetchOrgAlertSettings).not.toHaveBeenCalled()
  })

  it('tells a manager that the list is their departments’ part', async () => {
    const hint =
      'You see the alerts raised on keys held in the departments you manage.'
    const managers = await renderPage(membershipOf('manager'), 'alerts')
    await screen.findByText('Sudden rise in spending')
    expect(screen.getByText(hint)).toBeInTheDocument()
    managers.unmount()
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('Sudden rise in spending')
    expect(screen.queryByText(hint)).toBeNull()
  })

  it('tells whoever does not read alerts that the list is the warnings on their own keys', async () => {
    const own =
      'These are the warnings about the keys you hold: a key that has used most or all of what it was given, or was refused a request it could not pay for. An administrator of your organization can give a key more.'
    const everyones = /^A warning shows up within about a minute/
    api.fetchOrgAlerts.mockResolvedValue(alertPage([quotaWarning]))
    for (const viewer of [
      membershipOf('staff'),
      customRole('Finance Ops', ['usage.read']),
      // A role limited to departments that does not read alerts either.
      membershipOf('staff', {
        role: 'Key Desk',
        role_scope: 'dept',
        permissions: ['key.read', 'key.assign'],
      }),
    ]) {
      const view = await renderPage(viewer, 'alerts')
      await screen.findByText('80% of quota used')
      expect(screen.getByText(own), viewer.role).toBeInTheDocument()
      expect(screen.queryByText(everyones), viewer.role).toBeNull()
      expect(
        screen.queryByText(/departments you manage/),
        viewer.role
      ).toBeNull()
      // They read the list; marking and the settings are not theirs.
      expect(
        screen.queryByRole('button', { name: /^Mark “/ }),
        viewer.role
      ).toBeNull()
      expect(
        screen.queryByRole('button', { name: 'Alert settings' }),
        viewer.role
      ).toBeNull()
      view.unmount()
    }
    // Whoever reads alerts is told how the list comes about instead.
    await renderPage(membershipOf('readonly'), 'alerts')
    await screen.findByText('80% of quota used')
    expect(screen.getByText(everyones)).toBeInTheDocument()
    expect(screen.queryByText(own)).toBeNull()
  })

  it('tells a member with no warning of their own when one would show up', async () => {
    api.fetchOrgAlerts.mockResolvedValue(alertPage([]))
    await renderPage(membershipOf('staff'), 'alerts')
    await screen.findByText('No unresolved alerts')
    await userEvent.click(screen.getByRole('tab', { name: 'All alerts' }))
    await screen.findByText('No alerts yet')
    expect(
      screen.getByText(
        'A warning shows up here when one of your keys runs low on what it was given.'
      )
    ).toBeInTheDocument()
  })

  it('says there is nothing to look at, in the words of the list on show', async () => {
    api.fetchOrgAlerts.mockResolvedValue(alertPage([]))
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('No unresolved alerts')
    await userEvent.click(screen.getByRole('tab', { name: 'All alerts' }))
    await screen.findByText('No alerts yet')
  })

  it('pages through a long list', async () => {
    api.fetchOrgAlerts.mockResolvedValue(alertPage([spike, quotaWarning], 45))
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('1–20 of 45')
    expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('21–40 of 45')
    expect(lastCall(api.fetchOrgAlerts)).toEqual({
      p: 2,
      page_size: 20,
      state: 'open',
    })
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('41–45 of 45')
    expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()
    // Changing which alerts are listed starts over from the first page.
    await userEvent.click(screen.getByRole('tab', { name: 'All alerts' }))
    await waitFor(() =>
      expect(lastCall(api.fetchOrgAlerts)).toEqual({ p: 1, page_size: 20 })
    )
  })

  it('steps back when the page it is on empties', async () => {
    // Twenty-one unresolved alerts; the owner is on the second page, which
    // holds one, and deals with it.
    api.fetchOrgAlerts.mockImplementation(async ({ p }: { p: number }) =>
      p === 2
        ? { page: 2, page_size: 20, total: 21, items: [spike] }
        : alertPage([quotaWarning], 21)
    )
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('1–20 of 21')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('Sudden rise in spending')
    api.fetchOrgAlerts.mockImplementation(async ({ p }: { p: number }) =>
      p === 2
        ? { page: 2, page_size: 20, total: 20, items: [] }
        : alertPage([quotaWarning], 20)
    )
    await userEvent.click(
      screen.getByRole('button', {
        name: 'Mark “Sudden rise in spending” on Claude Code as handled',
      })
    )
    await screen.findByText('80% of quota used')
    expect(lastCall(api.fetchOrgAlerts)).toMatchObject({ p: 1 })
  })
})

describe('the alert settings', () => {
  /** Opens the settings dialog as the owner and returns it. */
  async function openSettings(): Promise<HTMLElement> {
    await renderPage(membershipOf('owner'), 'alerts')
    await screen.findByText('Sudden rise in spending')
    await userEvent.click(
      screen.getByRole('button', { name: 'Alert settings' })
    )
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText(/Warn at these shares/)
    return dialog
  }

  /** Types a value into a field of the dialog in place of what is there. */
  async function retype(dialog: HTMLElement, label: RegExp, value: string) {
    const field = within(dialog).getByLabelText(label)
    await userEvent.clear(field)
    if (value) await userEvent.type(field, value)
  }

  it('opens on what is stored, the floor in dollars', async () => {
    const dialog = await openSettings()
    expect(api.fetchOrgAlertSettings).toHaveBeenCalledTimes(1)
    expect(within(dialog).getByLabelText(/Warn at these shares/)).toHaveValue(
      '80, 100'
    )
    expect(within(dialog).getByLabelText(/Sudden rise/)).toHaveValue(5)
    expect(within(dialog).getByLabelText(/Smallest amount/)).toHaveValue(1)
    // No working hours yet: the rule is off, and says why.
    expect(
      within(dialog).getByRole('switch', {
        name: 'Report spending outside working hours',
      })
    ).not.toBeChecked()
    expect(within(dialog).getByText(/^Off: your organization/)).toBeVisible()
    expect(within(dialog).queryByLabelText('Day starts')).toBeNull()
  })

  it('saves what was typed, in the units the backend keeps', async () => {
    const dialog = await openSettings()
    await retype(dialog, /Warn at these shares/, '50, 90')
    await retype(dialog, /Sudden rise/, '8')
    await retype(dialog, /Smallest amount/, '2.5')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateOrgAlertSettings).toHaveBeenCalledWith({
        warn_at: [50, 90],
        spike_multiple: 8,
        off_hours_percent: 50,
        min_spend: 1250000,
        work_hours: null,
      })
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(mockToast.success).toHaveBeenCalledWith('Alert settings saved')
  })

  it('points at the field that is wrong, and sends nothing', async () => {
    const dialog = await openSettings()
    await retype(dialog, /Warn at these shares/, '80, 150')
    await retype(dialog, /Sudden rise/, '1')
    await retype(dialog, /Smallest amount/, '')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

    expect(
      await within(dialog).findByText(
        'Each level is a whole number from 1 to 100.'
      )
    ).toBeVisible()
    expect(
      within(dialog).getByText('A whole number from 2 to 1000.')
    ).toBeVisible()
    expect(within(dialog).getByText('An amount of 0 or more.')).toBeVisible()
    expect(within(dialog).getByLabelText(/Warn at these shares/)).toBeInvalid()
    expect(api.updateOrgAlertSettings).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    // Put right, it goes through, and the complaints are gone with it.
    await retype(dialog, /Warn at these shares/, '80')
    await retype(dialog, /Sudden rise/, '4')
    await retype(dialog, /Smallest amount/, '0')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.updateOrgAlertSettings).toHaveBeenCalledWith({
        warn_at: [80],
        spike_multiple: 4,
        off_hours_percent: 50,
        min_spend: 0,
        work_hours: null,
      })
    )
  })

  it('switches working hours on with the usual week, in the viewer’s own zone', async () => {
    const dialog = await openSettings()
    await userEvent.click(
      within(dialog).getByRole('switch', {
        name: 'Report spending outside working hours',
      })
    )
    expect(within(dialog).getByLabelText('Day starts')).toHaveValue('09:00')
    expect(within(dialog).getByLabelText('Day ends')).toHaveValue('18:00')
    for (const [day, working] of [
      ['Mon', true],
      ['Fri', true],
      ['Sat', false],
      ['Sun', false],
    ] as const) {
      const box = within(dialog).getByRole('checkbox', { name: day })
      if (working) expect(box, day).toBeChecked()
      else expect(box, day).not.toBeChecked()
    }
    await userEvent.click(within(dialog).getByRole('checkbox', { name: 'Sat' }))
    fireEvent.change(within(dialog).getByLabelText('Day ends'), {
      target: { value: '00:00' },
    })
    await retype(dialog, /Report from/, '75')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.updateOrgAlertSettings).toHaveBeenCalled())
    expect(lastCall(api.updateOrgAlertSettings)).toEqual({
      warn_at: [80, 100],
      spike_multiple: 5,
      off_hours_percent: 75,
      min_spend: 500000,
      work_hours: {
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
        days: [1, 2, 3, 4, 5, 6],
        start: 540,
        // 00:00 as the end of a day is midnight.
        end: 1440,
      },
    })
  })

  it('refuses hours that do not make a day, and a week with no day in it', async () => {
    const dialog = await openSettings()
    await userEvent.click(
      within(dialog).getByRole('switch', {
        name: 'Report spending outside working hours',
      })
    )
    for (const day of ['Mon', 'Tue', 'Wed', 'Thu', 'Fri']) {
      await userEvent.click(within(dialog).getByRole('checkbox', { name: day }))
    }
    fireEvent.change(within(dialog).getByLabelText('Day ends'), {
      target: { value: '08:00' },
    })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(
      await within(dialog).findByText('Choose at least one working day.')
    ).toBeVisible()
    expect(
      within(dialog).getByText(/^The day has to end later than it starts\./)
    ).toBeVisible()
    expect(api.updateOrgAlertSettings).not.toHaveBeenCalled()
  })

  it('opens on the working hours that are stored, and takes them away when switched off', async () => {
    api.fetchOrgAlertSettings.mockResolvedValue({
      ...SETTINGS,
      off_hours_percent: 120,
      work_hours: {
        timezone: 'Europe/Berlin',
        days: [1, 2, 3],
        start: 480,
        end: 1020,
      },
    })
    const dialog = await openSettings()
    const hours = within(dialog).getByRole('switch', {
      name: 'Report spending outside working hours',
    })
    expect(hours).toBeChecked()
    expect(within(dialog).getByLabelText('Day starts')).toHaveValue('08:00')
    expect(within(dialog).getByLabelText('Day ends')).toHaveValue('17:00')
    expect(within(dialog).getByLabelText(/Report from/)).toHaveValue(120)
    expect(
      within(dialog).getByRole('checkbox', { name: 'Thu' })
    ).not.toBeChecked()

    // Something unusable is typed into a field, which the switch then hides:
    // it must not be what stops the form from saving. What was typed into the
    // fields that stay on show is kept.
    await retype(dialog, /Sudden rise/, '7')
    await retype(dialog, /Report from/, '0')
    await userEvent.click(hours)
    expect(within(dialog).getByLabelText(/Sudden rise/)).toHaveValue(7)
    expect(within(dialog).queryByLabelText('Day starts')).toBeNull()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.updateOrgAlertSettings).toHaveBeenCalled())
    expect(lastCall(api.updateOrgAlertSettings)).toEqual({
      ...SETTINGS,
      spike_multiple: 7,
      off_hours_percent: 120,
      work_hours: null,
    })
  })

  it('stays open, with what was typed, when the backend refuses', async () => {
    api.updateOrgAlertSettings.mockResolvedValue({ success: false })
    const dialog = await openSettings()
    await retype(dialog, /Sudden rise/, '9')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(api.updateOrgAlertSettings).toHaveBeenCalled())
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(within(dialog).getByLabelText(/Sudden rise/)).toHaveValue(9)
    expect(mockToast.success).not.toHaveBeenCalled()
  })

  it('asks for the stored settings again every time it is opened', async () => {
    const dialog = await openSettings()
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Cancel' })
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    api.fetchOrgAlertSettings.mockResolvedValue({ ...SETTINGS, warn_at: [60] })
    await userEvent.click(
      screen.getByRole('button', { name: 'Alert settings' })
    )
    const reopened = await screen.findByRole('dialog')
    await waitFor(() =>
      expect(
        within(reopened).getByLabelText(/Warn at these shares/)
      ).toHaveValue('60')
    )
    expect(api.fetchOrgAlertSettings).toHaveBeenCalledTimes(2)
  })
})

describe('the audit log', () => {
  it('asks for the newest records and reads each one out in words', async () => {
    await renderPage(membershipOf('readonly'), 'audit')
    await screen.findByText('Renamed a department')
    expect(lastCall(api.fetchOrgAuditLogs)).toEqual({ p: 1, page_size: 20 })
    expect(rowText(rowOf('Renamed a department'))).toContain(
      'Fiona Founder | Renamed a department | Field Sales | Department | Name: Sales → Field Sales | 203.0.113.7'
    )
    // An invite link has no name: it is called by what it is, once.
    expect(rowText(rowOf('Created an invite link'))).toContain(
      'Ada Admin | Created an invite link | Invite link | Role: Staff | Department: Sales | —'
    )
  })

  it('names an account that is gone as such', async () => {
    api.fetchOrgAuditLogs.mockResolvedValue(
      auditPage([recordOf({ actor: '' })])
    )
    await renderPage(membershipOf('readonly'), 'audit')
    expect(
      await screen.findByText('An account that no longer exists')
    ).toBeInTheDocument()
  })

  it('searches by who did it once the typing pauses', async () => {
    await renderPage(membershipOf('readonly'), 'audit')
    await screen.findByText('Renamed a department')
    const calls = api.fetchOrgAuditLogs.mock.calls.length
    await userEvent.type(
      screen.getByRole('searchbox', { name: 'Search by who did it' }),
      '  ada '
    )
    await waitFor(() =>
      expect(lastCall(api.fetchOrgAuditLogs)).toEqual({
        p: 1,
        page_size: 20,
        actor: 'ada',
      })
    )
    // One request for the name, not one per keystroke.
    expect(api.fetchOrgAuditLogs.mock.calls.length).toBe(calls + 1)
  })

  it('narrows by what it was done to and by when', async () => {
    await renderPage(membershipOf('readonly'), 'audit')
    await screen.findByText('Renamed a department')

    await choose('Done to', 'Key')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgAuditLogs)).toEqual({
        p: 1,
        page_size: 20,
        target_type: 'key',
      })
    )
    await choose('Period', 'Last 7 days')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgAuditLogs)).toEqual({
        p: 1,
        page_size: 20,
        target_type: 'key',
        start_timestamp: daysAgo(6),
      })
    )
    await choose('Done to', 'Everything')
    await choose('Period', 'All time')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgAuditLogs)).toEqual({ p: 1, page_size: 20 })
    )
  })

  it('tells an empty log from a search that found nothing', async () => {
    api.fetchOrgAuditLogs.mockResolvedValue(auditPage([]))
    await renderPage(membershipOf('readonly'), 'audit')
    await screen.findByText('Nothing recorded yet')
    await choose('Done to', 'Role')
    await screen.findByText('No record matches')
  })

  it('pages through the log, and starts over when it is narrowed', async () => {
    api.fetchOrgAuditLogs.mockResolvedValue(auditPage([invited, renamed], 404))
    await renderPage(membershipOf('readonly'), 'audit')
    await screen.findByText('1–20 of 404')
    await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('21–40 of 404')
    expect(lastCall(api.fetchOrgAuditLogs)).toEqual({ p: 2, page_size: 20 })
    await choose('Done to', 'Member')
    await waitFor(() =>
      expect(lastCall(api.fetchOrgAuditLogs)).toEqual({
        p: 1,
        page_size: 20,
        target_type: 'member',
      })
    )
  })

  it('offers a retry when the log cannot be loaded', async () => {
    api.fetchOrgAuditLogs.mockRejectedValueOnce(new Error('503'))
    await renderPage(membershipOf('readonly'), 'audit')
    await screen.findByText('Could not load the audit log.')
    await userEvent.click(screen.getByRole('button', { name: /retry/i }))
    await screen.findByText('Renamed a department')
  })
})
