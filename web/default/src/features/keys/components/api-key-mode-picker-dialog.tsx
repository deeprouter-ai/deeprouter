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
import { ArrowRight, Settings2, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

type ApiKeyModePickerDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onPick: (mode: 'simple' | 'advanced') => void
}

/**
 * First step of the Create API Key flow — asks the user which mode to use
 * before opening the drawer (PRD §3.1, refined per UX feedback). Removes
 * the always-visible Simple/Advanced toggle from inside the drawer.
 */
export function ApiKeyModePickerDialog({
  open,
  onOpenChange,
  onPick,
}: ApiKeyModePickerDialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='!max-w-md sm:!max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Create API Key')}</DialogTitle>
          <DialogDescription>
            {t('Choose how you want to set this key up.')}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-3'>
          <ModeCard
            icon={<Sparkles className='h-5 w-5' />}
            badge={t('Recommended')}
            title={t('Simple')}
            description={t(
              'Pick what you will use the key for — chat, coding, image, video, voice, or auto. We route to the right model.'
            )}
            footnote={t('Best for most users. No model names to memorize.')}
            onClick={() => onPick('simple')}
          />
          <ModeCard
            icon={<Settings2 className='h-5 w-5' />}
            title={t('Advanced')}
            description={t(
              'Full control: model whitelist, channel group, per-key quota, expiration, IP allowlist, batch creation.'
            )}
            footnote={t('Best for developers and teams.')}
            onClick={() => onPick('advanced')}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}

function ModeCard({
  icon,
  title,
  description,
  footnote,
  badge,
  onClick,
}: {
  icon: React.ReactNode
  title: string
  description: string
  footnote?: string
  badge?: string
  onClick: () => void
}) {
  return (
    <button
      type='button'
      onClick={onClick}
      className={cn(
        'group border-border bg-background hover:border-foreground/40 relative flex w-full items-start gap-3 rounded-lg border p-4 text-left transition-all hover:shadow-sm',
        'focus-visible:border-ring focus-visible:ring-ring/30 focus-visible:ring-[3px] focus-visible:outline-none'
      )}
    >
      <span className='bg-muted text-muted-foreground group-hover:bg-foreground/10 group-hover:text-foreground flex h-10 w-10 shrink-0 items-center justify-center rounded-md border'>
        {icon}
      </span>
      <span className='flex min-w-0 flex-1 flex-col gap-1'>
        <span className='flex items-baseline justify-between gap-2'>
          <span className='flex items-center gap-2'>
            <span className='text-sm font-semibold'>{title}</span>
            {badge && (
              <span className='bg-foreground/10 text-foreground rounded-full px-2 py-0.5 text-[10px] font-medium'>
                {badge}
              </span>
            )}
          </span>
          <ArrowRight className='text-muted-foreground group-hover:text-foreground h-4 w-4 shrink-0 transition-transform group-hover:translate-x-0.5' />
        </span>
        <span className='text-muted-foreground text-xs leading-snug'>
          {description}
        </span>
        {footnote && (
          <span className='text-muted-foreground/80 mt-0.5 text-[11px]'>
            {footnote}
          </span>
        )}
      </span>
    </button>
  )
}
