// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useTranslation } from 'react-i18next'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { AlertsPanel } from './components/alerts-panel'
import { AuditLog } from './components/audit-log'
import { UsageReport } from './components/usage-report'
import { useOrgMembership } from './hooks/use-org-membership'
import { type OrgReportsSection, reportSections } from './lib/reports'

type OrgReportsPageProps = {
  /**
   * The section the address asks for. Left out, or one the viewer may not
   * read, it is the first one they may.
   */
  section?: OrgReportsSection
  onSectionChange: (section: OrgReportsSection) => void
}

/**
 * "Reports & alerts" — the fourth page of the organization area (Enterprise
 * Org PRD §6): what the company's keys spent, the alerts raised about them,
 * and the audit log of everything that was changed.
 *
 * Its three sections take three different permissions, so each viewer gets the
 * tabs their role reads: a finance role the usage alone, a manager usage and
 * alerts for their departments, a read-only member all three. The backend
 * sends each of them only what they may see and checks every request again.
 */
export function OrgReportsPage({
  section,
  onSectionChange,
}: OrgReportsPageProps) {
  const { t } = useTranslation()
  const membershipQuery = useOrgMembership()
  const membership = membershipQuery.data
  const sections = reportSections(membership)
  const shown = section && sections.includes(section) ? section : sections[0]
  const label: Record<OrgReportsSection, string> = {
    usage: t('Usage'),
    alerts: t('Alerts'),
    audit: t('Audit log'),
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Reports & alerts')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {membershipQuery.isError ? (
          <ErrorState
            description={t('Could not load your organization.')}
            onRetry={() => void membershipQuery.refetch()}
          />
        ) : !membership || !shown ? (
          <div className='space-y-3'>
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className='h-12 rounded-xl' />
            ))}
          </div>
        ) : (
          <Tabs
            value={shown}
            onValueChange={(next) => onSectionChange(next as OrgReportsSection)}
          >
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <TabsList>
                {sections.map((one) => (
                  <TabsTrigger key={one} value={one}>
                    {label[one]}
                  </TabsTrigger>
                ))}
              </TabsList>
              <span className='text-muted-foreground truncate text-sm'>
                {membership.org_name}
              </span>
            </div>
            {sections.includes('usage') && (
              <TabsContent value='usage' className='pt-2'>
                <UsageReport membership={membership} />
              </TabsContent>
            )}
            {sections.includes('alerts') && (
              <TabsContent value='alerts' className='pt-2'>
                <AlertsPanel membership={membership} />
              </TabsContent>
            )}
            {sections.includes('audit') && (
              <TabsContent value='audit' className='pt-2'>
                <AuditLog />
              </TabsContent>
            )}
          </Tabs>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
