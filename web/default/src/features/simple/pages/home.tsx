// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronRight, Loader2, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { CompanyWalletNotice } from '@/features/org/components/company-wallet-notice'
import { useWalletView } from '@/features/org/hooks/use-wallet-view'
import { TopupSheet } from '../components/topup-sheet'
import { usePullToRefresh } from '../hooks/use-pull-to-refresh'
import { clipsAffordable } from '../lib/balance'
import { SIMPLE_PURPOSES } from '../lib/purposes'

/**
 * Simple home: the balance card (with top-up) and the "what do you want to
 * do" grid. That is the whole product for a non-technical user — put money
 * in, pick what to do, hand it to their AI.
 */
export function SimpleHome(props: { openTopup?: boolean }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.auth.user)
  const setUser = useAuthStore((s) => s.auth.setUser)
  const [topupOpen, setTopupOpen] = useState(!!props.openTopup)

  const refresh = useCallback(async () => {
    const res = await getSelf().catch(() => null)
    if (res?.success && res.data) setUser(res.data)
  }, [setUser])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const { pull, refreshing, ready } = usePullToRefresh(refresh)

  const handleTopupChange = (open: boolean) => {
    setTopupOpen(open)
    // Drop `?topup=1` so a reload does not reopen the sheet.
    if (!open && props.openTopup) {
      void navigate({ to: '/simple', search: {}, replace: true })
    }
  }

  const clips = clipsAffordable(user?.quota)
  const name = user?.display_name || user?.username || ''
  // Enterprise Org: a member's keys spend the company wallet. They are told so
  // where a personal account sees its balance, and are offered no top-up —
  // not by the card, and not by a link that carries `?topup=1` either.
  const walletView = useWalletView()

  return (
    <main className='px-4 pt-4 md:px-0 md:pt-0'>
      <div
        className='text-muted-foreground flex items-center justify-center overflow-hidden text-xs transition-[height]'
        style={{ height: refreshing ? 28 : pull * 0.4 }}
        aria-hidden={!refreshing && pull === 0}
      >
        {refreshing ? (
          <Loader2 className='size-4 animate-spin' />
        ) : ready ? (
          t('Release to refresh')
        ) : pull > 0 ? (
          t('Pull to refresh')
        ) : null}
      </div>

      <header className='mb-4 px-1 md:mb-6 md:px-0'>
        <h1 className='text-xl font-semibold md:text-2xl'>
          {name ? t('Hi, {{name}}', { name }) : t('Hi there')}
        </h1>
      </header>

      <div
        className={cn(
          'grid items-start gap-5 lg:gap-6',
          walletView !== 'pending' &&
            'lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]'
        )}
      >
        {walletView === 'company' && <CompanyWalletNotice />}
        {walletView === 'own' && (
          <section className='bg-card border-border rounded-xl border p-5'>
            <p className='text-muted-foreground text-sm'>{t('Balance')}</p>
            <div className='mt-1 flex flex-wrap items-end justify-between gap-3 lg:flex-col lg:items-start'>
              <p className='text-3xl font-semibold tabular-nums'>
                {formatQuota(user?.quota ?? 0)}
              </p>
              <Button type='button' onClick={() => setTopupOpen(true)}>
                <Plus aria-hidden='true' />
                {t('Add credit')}
              </Button>
            </div>
            <p className='text-muted-foreground mt-2 text-sm'>
              {clips
                ? t('Enough for about {{count}} short video clips', {
                    count: clips,
                  })
                : t('Add credit to start making things with AI')}
            </p>
          </section>
        )}

        <section>
          <h2 className='mb-3 px-1 text-base font-semibold md:px-0 md:text-lg'>
            {t('What do you want to do?')}
          </h2>
          <ul className='grid grid-cols-2 gap-3 md:grid-cols-3'>
            {SIMPLE_PURPOSES.map((purpose) => {
              const Icon = purpose.icon
              return (
                <li key={purpose.id}>
                  <Link
                    to='/simple/use/$purpose'
                    params={{ purpose: purpose.id }}
                    className='bg-card border-border hover:border-foreground/25 focus-visible:ring-ring flex h-full min-h-32 flex-col justify-between gap-3 rounded-xl border p-4 transition-colors outline-none focus-visible:ring-2'
                  >
                    <span className='bg-accent/10 text-accent dark:bg-muted dark:text-foreground inline-flex size-11 items-center justify-center rounded-xl'>
                      <Icon className='size-6' />
                    </span>
                    <span>
                      <span className='flex items-center justify-between text-[15px] font-semibold'>
                        {t(purpose.title)}
                        <ChevronRight className='text-muted-foreground size-4' />
                      </span>
                      <span className='text-muted-foreground mt-0.5 block text-xs leading-snug'>
                        {t(purpose.blurb)}
                      </span>
                    </span>
                  </Link>
                </li>
              )
            })}
          </ul>
        </section>
      </div>

      {walletView === 'own' && (
        <TopupSheet
          open={topupOpen}
          onOpenChange={handleTopupChange}
          onBalanceChange={() => void refresh()}
        />
      )}
    </main>
  )
}
