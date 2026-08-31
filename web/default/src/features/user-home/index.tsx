/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  CreditCard,
  RefreshCcw,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import { getUserHome } from './api'
import type { UserHomeData } from './types'

export function UserHome() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['user-home'],
    queryFn: getUserHome,
    retry: false,
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Home')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Your balance and plan.')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        {query.isError ? (
          <Alert variant='destructive' aria-live='polite'>
            <AlertTriangle aria-hidden='true' />
            <AlertTitle>{t('Request failed')}</AlertTitle>
            <AlertDescription>
              {t('Unable to load your home dashboard.')}
            </AlertDescription>
            <AlertAction>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() => void query.refetch()}
              >
                <RefreshCcw data-icon='inline-start' />
                {t('Retry')}
              </Button>
            </AlertAction>
          </Alert>
        ) : query.isLoading || !query.data ? (
          <UserHomeSkeleton />
        ) : (
          <StatusGrid data={query.data} />
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function StatusGrid(props: { data: UserHomeData }) {
  const { t } = useTranslation()
  const activePlan = props.data.active_plan
  const cards = [
    {
      label: t('Balance'),
      value: formatQuota(props.data.account.balance_quota),
      detail: t('{{count}} top-ups total', {
        count: props.data.account.topups_count,
      }),
      icon: WalletCards,
    },
    {
      label: t('Plan'),
      value: activePlan?.title || t('No active plan'),
      detail:
        activePlan?.status === 'active'
          ? t('Active until {{date}}', {
              date: new Date(activePlan.end_time * 1000).toLocaleDateString(),
            })
          : t('No active subscription'),
      icon: CreditCard,
    },
  ]

  return (
    <div className='grid grid-cols-1 gap-3 lg:grid-cols-2'>
      {cards.map((item) => (
        <Card key={item.label} size='sm'>
          <CardHeader>
            <div className='flex items-center justify-between gap-3'>
              <CardTitle className='text-sm font-semibold'>
                {item.label}
              </CardTitle>
              <item.icon className='text-muted-foreground size-4' />
            </div>
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-semibold tabular-nums'>
              {item.value}
            </div>
            <p className='text-muted-foreground mt-1 text-sm'>{item.detail}</p>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function UserHomeSkeleton() {
  return (
    <div className='grid grid-cols-1 gap-3 lg:grid-cols-2'>
      {Array.from({ length: 2 }).map((_, index) => (
        <Skeleton key={index} className='h-32 w-full' />
      ))}
    </div>
  )
}
