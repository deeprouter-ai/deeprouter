// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect } from 'react'
import { Link, Outlet, useRouterState } from '@tanstack/react-router'
import { Home, ReceiptText, UserRound, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { SystemBrand } from '@/components/layout/components/system-brand'
import { persistPersona } from '@/features/profile/lib/persist-persona'
import { readPersona } from '../lib/mode'
import { ConsoleModeButton } from './console-mode-button'

type Tab = {
  to: '/simple' | '/simple/records' | '/simple/me'
  label: string
  icon: LucideIcon
}

const TABS: Tab[] = [
  { to: '/simple', label: 'Home', icon: Home },
  { to: '/simple/records', label: 'Records', icon: ReceiptText },
  { to: '/simple/me', label: 'Me', icon: UserRound },
]

export function SimpleShell() {
  const { t } = useTranslation()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const user = useAuthStore((s) => s.auth.user)
  const pushed = pathname.startsWith('/simple/use/')

  // A brand-new account still carries the `unset` persona, which the full
  // console answers by bouncing to /welcome. Landing here *is* the choice
  // (new users default to Simple, PRD D5), so record it once, silently.
  // Not for a member of an organization: their default is the Advanced
  // console (Enterprise Org D42), so nothing sends them here unasked, and a
  // visit is not taken for a choice — that they make on the welcome page.
  useEffect(() => {
    if (user && !user.org_id && readPersona(user.setting) === 'unset') {
      void persistPersona('casual')
    }
  }, [user])

  return (
    <div className='bg-background text-foreground min-h-dvh'>
      <header className='bg-background border-border sticky top-0 z-30 border-b pt-[env(safe-area-inset-top)]'>
        <div className='mx-auto flex h-14 max-w-[34rem] items-center justify-between gap-3 px-4 md:max-w-6xl md:px-6'>
          <SystemBrand variant='inline' />
          <ConsoleModeButton target='advanced' />
        </div>
      </header>

      <div className='mx-auto max-w-[34rem] md:grid md:max-w-6xl md:grid-cols-[10rem_minmax(0,1fr)] md:gap-6 md:px-6 md:py-8 lg:grid-cols-[12rem_minmax(0,1fr)]'>
        <nav
          aria-label={t('Main')}
          className={cn(
            'bg-card border-border fixed inset-x-0 bottom-0 z-20 mx-auto w-full max-w-[34rem] border-t pb-[env(safe-area-inset-bottom)] md:sticky md:top-24 md:z-auto md:self-start md:rounded-xl md:border md:p-2',
            pushed && 'hidden md:block'
          )}
        >
          <ul className='grid h-16 grid-cols-3 md:h-auto md:grid-cols-1 md:gap-1'>
            {TABS.map((tab) => {
              const active =
                tab.to === '/simple'
                  ? pathname === '/simple' || pathname === '/simple/' || pushed
                  : pathname.startsWith(tab.to)
              const Icon = tab.icon
              return (
                <li key={tab.to}>
                  <Link
                    to={tab.to}
                    aria-current={
                      active ? (pushed ? 'location' : 'page') : undefined
                    }
                    className={cn(
                      'focus-visible:ring-ring flex h-full flex-col items-center justify-center gap-1 rounded-[7px] text-[11px] font-semibold transition-colors outline-none focus-visible:ring-2 focus-visible:ring-inset md:min-h-10 md:flex-row md:justify-start md:gap-2.5 md:px-3 md:text-sm',
                      active
                        ? 'text-accent dark:text-foreground md:bg-accent/10'
                        : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                    )}
                  >
                    <Icon className='size-5' aria-hidden='true' />
                    {t(tab.label)}
                  </Link>
                </li>
              )
            })}
          </ul>
        </nav>
        <div
          className={cn(
            'min-w-0 pb-6',
            !pushed && 'pb-[calc(5rem+env(safe-area-inset-bottom))] md:pb-8'
          )}
        >
          <Outlet />
        </div>
      </div>
    </div>
  )
}
