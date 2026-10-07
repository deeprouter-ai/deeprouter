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
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getUserAvatarFallback } from '@/lib/avatar'
import { SignOutDialog } from '@/components/sign-out-dialog'
import { useWalletView } from '@/features/org/hooks/use-wallet-view'
import { updateUserLanguage } from '@/features/profile/api'
import { persistPersona } from '@/features/profile/lib/persist-persona'
import { ADVANCED_HOME } from '../lib/mode'

function Row(props: {
  icon: LucideIcon
  label: string
  detail?: string
  /** Second line under the label, for rows that need explaining. */
  subtitle?: string
  onClick: () => void
  destructive?: boolean
}) {
  const Icon = props.icon
  return (
    <li>
      <button
        type='button'
        onClick={props.onClick}
        className={`active:bg-muted flex min-h-12 w-full items-center gap-3 px-4 py-3 text-left text-[15px] ${props.destructive ? 'text-destructive' : ''}`}
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
  const [switching, setSwitching] = useState(false)

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

  const goProfessional = async () => {
    if (switching) return
    setSwitching(true)
    const error = await persistPersona('dev').catch(() => 'save failed')
    setSwitching(false)
    if (error) {
      toast.error(t('Could not switch. Please try again.'))
      return
    }
    void navigate({ to: ADVANCED_HOME })
  }

  return (
    <div className='flex flex-col gap-4 px-4 pt-4'>
      <h1 className='px-1 text-2xl font-semibold'>{t('Me')}</h1>

      <section className='bg-card border-border flex items-center gap-3 rounded-2xl border p-4'>
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

      <ul className='bg-card border-border divide-border divide-y overflow-hidden rounded-2xl border'>
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

      <ul className='bg-card border-border divide-border divide-y overflow-hidden rounded-2xl border'>
        <Row
          icon={Briefcase}
          label={t('Switch to professional mode')}
          subtitle={t('For teams & developers')}
          onClick={goProfessional}
        />
      </ul>

      <ul className='bg-card border-border overflow-hidden rounded-2xl border'>
        <Row
          icon={LogOut}
          label={t('Sign out')}
          onClick={() => setSignOutOpen(true)}
          destructive
        />
      </ul>

      <SignOutDialog open={signOutOpen} onOpenChange={setSignOutOpen} />
    </div>
  )
}
