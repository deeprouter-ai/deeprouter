// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect } from 'react'
import { Link, Outlet, useRouterState } from '@tanstack/react-router'
import { Home, ReceiptText, UserRound, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { persistPersona } from '@/features/profile/lib/persist-persona'
import { readPersona } from '../lib/mode'

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

/**
 * The Simple console frame: a native-app shell rather than a dashboard. One
 * phone-width column (centred on desktop), a fixed bottom tab bar with the
 * safe-area inset, no sidebar and no top navigation. Pushed pages (a purpose's
 * "copy for AI" page) hide the tab bar, the way a native detail screen does.
 */
export function SimpleShell() {
  const { t } = useTranslation()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const user = useAuthStore((s) => s.auth.user)
  const pushed = pathname.startsWith('/simple/use/')

  // A brand-new account still carries the `unset` persona, which the full
  // console answers by bouncing to /welcome. Landing here *is* the choice
  // (new users default to Simple, PRD D5), so record it once, silently.
  useEffect(() => {
    if (user && readPersona(user.setting) === 'unset') {
      void persistPersona('casual')
    }
  }, [user])

  return (
    <div className='bg-background text-foreground min-h-dvh'>
      <div
        className={cn(
          'mx-auto flex min-h-dvh w-full max-w-[480px] flex-col pt-[env(safe-area-inset-top)]',
          !pushed && 'pb-[calc(4rem+env(safe-area-inset-bottom))]'
        )}
      >
        <Outlet />
      </div>

      {!pushed && (
        <nav
          aria-label={t('Main')}
          className='bg-card/95 border-border fixed inset-x-0 bottom-0 z-40 mx-auto w-full max-w-[480px] border-t pb-[env(safe-area-inset-bottom)] backdrop-blur'
        >
          <ul className='grid h-16 grid-cols-3'>
            {TABS.map((tab) => {
              const active =
                tab.to === '/simple'
                  ? pathname === '/simple' || pathname === '/simple/'
                  : pathname.startsWith(tab.to)
              const Icon = tab.icon
              return (
                <li key={tab.to}>
                  <Link
                    to={tab.to}
                    aria-current={active ? 'page' : undefined}
                    className={cn(
                      'flex h-full flex-col items-center justify-center gap-1 text-[11px] font-medium transition-transform active:scale-95',
                      active
                        ? 'text-accent dark:text-foreground'
                        : 'text-foreground/70'
                    )}
                  >
                    <Icon
                      className='size-6'
                      strokeWidth={active ? 2.25 : 1.75}
                    />
                    {t(tab.label)}
                  </Link>
                </li>
              )
            })}
          </ul>
        </nav>
      )}
    </div>
  )
}
