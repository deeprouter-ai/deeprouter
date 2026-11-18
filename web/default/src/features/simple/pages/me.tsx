// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  Briefcase,
  ChevronRight,
  Languages,
  LogOut,
  Wallet,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { getUserAvatarFallback } from '@/lib/avatar'
import { SignOutDialog } from '@/components/sign-out-dialog'
import { useWalletView } from '@/features/org/hooks/use-wallet-view'
import { updateUserLanguage } from '@/features/profile/api'
import { useConsoleModeSwitch } from '../hooks/use-console-mode-switch'

function Row(props: {
  icon: LucideIcon
  label: string
  detail?: string
  /** Second line under the label, for rows that need explaining. */
  subtitle?: string
  onClick: () => void
  destructive?: boolean
  disabled?: boolean
}) {
  const Icon = props.icon
  return (
    <li>
      <button
        type='button'
        onClick={props.onClick}
        disabled={props.disabled}
        className={`active:bg-muted focus-visible:ring-ring flex min-h-12 w-full items-center gap-3 px-4 py-3 text-left text-[15px] outline-none focus-visible:ring-2 focus-visible:ring-inset disabled:opacity-45 ${props.destructive ? 'text-destructive' : ''}`}
      >
        <Icon className='size-5 shrink-0' />
        <span className='flex-1'>
          {props.label}
          {props.subtitle && (
            <span className='text-muted-foreground block text-xs'>
              {props.subtitle}
            </span>
          )}
        </span>
        {props.detail && (
          <span className='text-muted-foreground text-sm'>{props.detail}</span>
        )}
        {!props.destructive && (
          <ChevronRight className='text-muted-foreground size-4' />
        )}
      </button>
    </li>
  )
}

/** Account, language, the way into the professional console, sign out. */
export function SimpleMe() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.auth.user)
  const [signOutOpen, setSignOutOpen] = useState(false)
  const { switchMode, switching } = useConsoleModeSwitch('advanced')

  const name = user?.display_name || user?.username || ''
  const zh = i18n.language?.startsWith('zh')
  // Enterprise Org: a member's keys spend the company wallet; there is
  // nothing for them to add credit to.
  const walletView = useWalletView()

  const toggleLanguage = async () => {
    const next = zh ? 'en' : 'zh'
    await i18n.changeLanguage(next)
    void updateUserLanguage(next).catch(() => undefined)
  }

  return (
    <main className='flex max-w-3xl flex-col gap-4 px-4 pt-4 md:px-0 md:pt-0'>
      <h1 className='px-1 text-2xl font-semibold'>{t('Me')}</h1>

      <section className='bg-card border-border flex items-center gap-3 rounded-xl border p-4'>
        <span className='bg-muted inline-flex size-12 items-center justify-center rounded-full text-base font-semibold'>
          {getUserAvatarFallback(name)}
        </span>
        <span className='min-w-0'>
          <span className='block truncate text-base font-semibold'>{name}</span>
          {user?.email && (
            <span className='text-muted-foreground block truncate text-sm'>
              {user.email}
            </span>
          )}
        </span>
      </section>

      <ul className='bg-card border-border divide-border divide-y overflow-hidden rounded-xl border'>
        {walletView === 'own' && (
          <Row
            icon={Wallet}
            label={t('Add credit')}
            onClick={() => navigate({ to: '/simple', search: { topup: true } })}
          />
        )}
        <Row
          icon={Languages}
          label={t('Language')}
          detail={zh ? '中文' : 'English'}
          onClick={toggleLanguage}
        />
      </ul>

      <ul className='bg-card border-border divide-border divide-y overflow-hidden rounded-xl border'>
        <Row
          icon={Briefcase}
          label={t('Switch to professional mode')}
          subtitle={t('For teams & developers')}
          onClick={switchMode}
          disabled={switching}
        />
      </ul>

      <ul className='bg-card border-border overflow-hidden rounded-xl border'>
        <Row
          icon={LogOut}
          label={t('Sign out')}
          onClick={() => setSignOutOpen(true)}
          destructive
        />
      </ul>

      <SignOutDialog open={signOutOpen} onOpenChange={setSignOutOpen} />
    </main>
  )
}
