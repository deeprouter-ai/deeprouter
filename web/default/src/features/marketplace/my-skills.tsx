/*
Copyright (C) 2026 DeepRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Download, Package } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { SectionPageLayout } from '@/components/layout'
import { fetchMySkills, marketplaceQueryKeys } from './api'
import { useSkillDownload } from './hooks/use-skill-download'

function formatDate(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

export function MySkillsPage() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: marketplaceQueryKeys.mySkills(),
    queryFn: () => fetchMySkills({ limit: 100 }),
  })
  const { download, pendingSlug } = useSkillDownload()

  const skills = data?.skills ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('My Skills')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {isLoading ? (
          <div className='space-y-3'>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className='h-20 rounded-xl' />
            ))}
          </div>
        ) : skills.length === 0 ? (
          <EmptyState
            icon={Package}
            title={t('No skills downloaded yet')}
            description={t('Browse the marketplace and download your first skill.')}
            action={
              <Button variant='outline' render={<Link to='/marketplace' />}>
                {t('Browse marketplace')}
              </Button>
            }
            bordered
          />
        ) : (
          <div className='space-y-3'>
            {skills.map((entry) => {
              const delisted = entry.skill_status === 'deprecated'
              return (
                <div
                  key={entry.skill_id}
                  className='border-border bg-card flex flex-col gap-3 rounded-xl border p-4 sm:flex-row sm:items-center sm:justify-between'
                >
                  <div className='min-w-0'>
                    <div className='flex flex-wrap items-center gap-2'>
                      <Link
                        to='/marketplace/$slug'
                        params={{ slug: entry.slug }}
                        className='hover:text-accent font-medium transition-colors'
                      >
                        {entry.name}
                      </Link>
                      {entry.version && (
                        <Badge variant='ghost' className='tabular-nums'>
                          v{entry.version}
                        </Badge>
                      )}
                      {delisted && (
                        <Badge variant='destructive'>{t('Delisted')}</Badge>
                      )}
                    </div>
                    <p className='text-muted-foreground mt-1 text-sm'>
                      {t('Downloaded {{date}}', {
                        date: formatDate(entry.enabled_at),
                      })}
                    </p>
                    {delisted && (
                      // PRD §8.3 copy, verbatim in zh.
                      <p className='text-muted-foreground mt-1 text-sm'>
                        {t(
                          'This skill was delisted by the admin and is no longer maintained; the version you downloaded still works.'
                        )}
                      </p>
                    )}
                  </div>
                  {/* PRD §8.3: no re-download for delisted skills — there is
                      no active version left to download. */}
                  {!delisted && (
                    <Button
                      variant='outline'
                      onClick={() => void download(entry.slug)}
                      disabled={pendingSlug === entry.slug}
                      className='shrink-0'
                    >
                      <Download className='size-4' />
                      {t('Download latest')}
                    </Button>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
