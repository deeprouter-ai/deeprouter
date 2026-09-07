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
import { Receipt } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { SectionPageLayout } from '@/components/layout'
import { fetchMyPurchases, marketplaceQueryKeys } from './api'

function formatDate(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

export function PurchasesPage() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: marketplaceQueryKeys.myPurchases(),
    queryFn: () => fetchMyPurchases({ limit: 100 }),
  })

  const purchases = data?.purchases ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Purchase History')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {isLoading ? (
          <div className='space-y-3'>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className='h-16 rounded-xl' />
            ))}
          </div>
        ) : purchases.length === 0 ? (
          <EmptyState
            icon={Receipt}
            title={t('No purchases yet')}
            description={t('Paid skills you buy will show up here.')}
            action={
              <Button variant='outline' render={<Link to='/marketplace' />}>
                {t('Browse marketplace')}
              </Button>
            }
            bordered
          />
        ) : (
          <div className='space-y-3'>
            {purchases.map((p) => (
              <div
                key={p.skill_id}
                className='border-border bg-card flex items-center justify-between gap-3 rounded-xl border p-4'
              >
                <div className='min-w-0'>
                  <Link
                    to='/marketplace/$slug'
                    params={{ slug: p.slug }}
                    className='hover:text-accent font-medium transition-colors'
                  >
                    {p.name}
                  </Link>
                  <p className='text-muted-foreground mt-1 text-sm'>
                    {formatDate(p.purchased_at)}
                  </p>
                </div>
                {/* PRD §5.4: purchase amounts are always USD. */}
                <span className='font-medium tabular-nums'>
                  ${p.price_usd.toFixed(2)}
                </span>
              </div>
            ))}
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
