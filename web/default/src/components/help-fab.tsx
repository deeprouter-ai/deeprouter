/*
Copyright (C) 2023-2026 QuantumNous

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

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import {
  HelpCircle,
  Mail,
  MessageCircle,
  PlayCircle,
  ScrollText,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useIsCasual } from '@/hooks/use-casual'
import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

const NEW_DOT_STORAGE_KEY = 'dr_help_seen'

/**
 * Floating action button anchored bottom-right of every authenticated
 * page. Opens a popover with workshop video / WeChat group / docs /
 * email links. URLs are admin-configurable via System Settings →
 * Operations (controller.GetStatus exposes them on the status payload).
 *
 * Casual persona users get a red NEW dot on the icon until first
 * interaction, signalling that this is the path for "I'm stuck".
 *
 * See docs/tasks/casual-ux-prd.md §2.3.
 */
export function HelpFab() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const casual = useIsCasual()
  const [open, setOpen] = useState(false)
  // Casual users get a red dot until they've opened the menu at least
  // once. Persisted across sessions in localStorage. Initialized
  // synchronously from localStorage in useState so we avoid the
  // "setState inside useEffect" lint rule and the initial-flash render.
  const [showNewDot, setShowNewDot] = useState<boolean>(() => {
    if (typeof window === 'undefined') return false
    try {
      return (
        casual && window.localStorage.getItem(NEW_DOT_STORAGE_KEY) !== '1'
      )
    } catch {
      return false
    }
  })

  const handleToggle = () => {
    setOpen((v) => !v)
    if (showNewDot) {
      setShowNewDot(false)
      try {
        window.localStorage.setItem(NEW_DOT_STORAGE_KEY, '1')
      } catch {
        /* ignore */
      }
    }
  }

  // Read admin-configured URLs from /api/status. Empty string → render
  // a disabled placeholder so the user sees "coming soon" instead of
  // an invisible dead link.
  const videoUrl = (status?.help_video_url as string) || ''
  const wechatQR = (status?.help_wechat_qr as string) || ''
  const wechatId = (status?.help_wechat_id as string) || ''
  const supportEmail =
    (status?.help_support_email as string) || 'support@deeprouter.ai'

  return (
    <div className='fixed right-4 bottom-4 z-40 sm:right-6 sm:bottom-6'>
      {open && (
        <div className='bg-card animate-in fade-in zoom-in-95 absolute right-0 bottom-14 w-72 rounded-xl border p-3 shadow-lg'>
          <div className='mb-2 flex items-center justify-between'>
            <span className='text-sm font-semibold'>
              {t("Stuck? We're here to help.")}
            </span>
            <button
              type='button'
              onClick={() => setOpen(false)}
              className='text-muted-foreground hover:text-foreground -mr-1 rounded p-1'
              aria-label={t('Close')}
            >
              <X className='h-3.5 w-3.5' />
            </button>
          </div>
          <div className='space-y-2'>
            <HelpRow
              icon={<PlayCircle className='h-4 w-4' />}
              title={t('Watch the 3-minute tutorial')}
              subtitle={
                videoUrl
                  ? t('Open video in a new tab')
                  : t('Video coming soon')
              }
              href={videoUrl || undefined}
              disabled={!videoUrl}
            />
            <HelpRow
              icon={<MessageCircle className='h-4 w-4' />}
              title={t('Join our WeChat group')}
              subtitle={
                wechatQR || wechatId
                  ? t('Scan QR or add the group')
                  : t('WeChat group coming soon')
              }
              onClick={
                wechatQR || wechatId
                  ? () => alert(wechatQR || wechatId)
                  : undefined
              }
              disabled={!wechatQR && !wechatId}
            />
            <HelpRow
              icon={<ScrollText className='h-4 w-4' />}
              title={t('Read the setup guides')}
              subtitle={t('Cherry Studio, Cursor, Code, and more')}
              to='/onboarding/cherry-studio'
            />
            <HelpRow
              icon={<Mail className='h-4 w-4' />}
              title={t('Email support')}
              subtitle={supportEmail}
              href={`mailto:${supportEmail}`}
            />
          </div>
        </div>
      )}
      <Button
        type='button'
        onClick={handleToggle}
        size='sm'
        className={cn(
          'relative h-12 w-12 rounded-full p-0 shadow-md hover:shadow-lg',
          casual
            ? 'bg-foreground text-background hover:bg-foreground/90'
            : 'bg-muted text-foreground border hover:bg-muted/80'
        )}
        aria-label={t('Help')}
      >
        <HelpCircle className='h-5 w-5' />
        {showNewDot && (
          <span
            aria-hidden
            className='animate-pulse absolute -top-0.5 -right-0.5 inline-flex h-3 w-3 items-center justify-center rounded-full bg-rose-500 ring-2 ring-background'
          />
        )}
      </Button>
    </div>
  )
}

function HelpRow({
  icon,
  title,
  subtitle,
  href,
  to,
  onClick,
  disabled,
}: {
  icon: React.ReactNode
  title: string
  subtitle: string
  href?: string
  to?: string
  onClick?: () => void
  disabled?: boolean
}) {
  const cls = cn(
    'flex items-start gap-2.5 rounded-lg border p-2.5 text-left transition-colors',
    disabled
      ? 'border-dashed border-border bg-muted/30 cursor-not-allowed opacity-60'
      : 'bg-background hover:border-foreground/40 hover:shadow-sm'
  )
  const body = (
    <>
      <span className='bg-muted text-muted-foreground flex h-7 w-7 shrink-0 items-center justify-center rounded-md'>
        {icon}
      </span>
      <span className='min-w-0 flex-1'>
        <span className='block text-xs font-medium'>{title}</span>
        <span className='text-muted-foreground line-clamp-1 block text-[11px]'>
          {subtitle}
        </span>
      </span>
    </>
  )
  if (disabled) {
    return (
      <div className={cls} aria-disabled>
        {body}
      </div>
    )
  }
  if (to) {
    return (
      <Link to={to as never} className={cls}>
        {body}
      </Link>
    )
  }
  if (href) {
    return (
      <a
        href={href}
        target='_blank'
        rel='noreferrer'
        className={cls}
      >
        {body}
      </a>
    )
  }
  return (
    <button type='button' onClick={onClick} className={cls}>
      {body}
    </button>
  )
}
