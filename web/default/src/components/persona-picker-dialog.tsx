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
import { ArrowRight, MessageSquare, Terminal, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import type { Persona } from '@/features/profile/types'

type PersonaPickerDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onPick: (persona: Persona) => void
  /**
   * Hide the dialog's close affordance so the picker is effectively
   * mandatory on first-login. Profile-page invocation should leave this
   * default (closable).
   */
  dismissible?: boolean
}

/**
 * One-shot persona picker shown to a fresh account on first authenticated
 * load (when `setting.persona === 'unset'`). 3 cards — Casual / Dev / Team —
 * each describes who it's for, what UI they'll get, and a footnote example.
 * Selection is persisted by the parent via updateUserSettings, which then
 * writes the matching sidebar_modules preset too.
 */
export function PersonaPickerDialog({
  open,
  onOpenChange,
  onPick,
  dismissible = true,
}: PersonaPickerDialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!dismissible && !next) return
        onOpenChange(next)
      }}
    >
      <DialogContent
        className='!max-w-md sm:!max-w-2xl'
        showCloseButton={dismissible}
      >
        <DialogHeader>
          <DialogTitle>{t('Welcome to DeepRouter')}</DialogTitle>
          <DialogDescription>
            {t(
              'How do you plan to use DeepRouter? You can change this anytime in your profile.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='grid gap-3 sm:grid-cols-3'>
          <PersonaCard
            icon={<MessageSquare className='h-5 w-5' />}
            title={t('Casual')}
            description={t(
              'Chat, write, translate, generate images — use AI clients like Cherry Studio.'
            )}
            footnote={t('No code required.')}
            onClick={() => onPick('casual')}
          />
          <PersonaCard
            icon={<Terminal className='h-5 w-5' />}
            badge={t('Most users')}
            title={t('Developer')}
            description={t(
              'Build apps, run scripts, call the API from your own code. Full Key controls.'
            )}
            footnote={t('Coding, Cursor, Claude Code, custom scripts.')}
            onClick={() => onPick('dev')}
          />
          <PersonaCard
            icon={<Users className='h-5 w-5' />}
            title={t('Team / Enterprise')}
            description={t(
              'Shared keys, audit trail, integration into team workflows.'
            )}
            footnote={t('Same UI as Developer for now; team-only tools coming soon.')}
            onClick={() => onPick('team')}
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}

function PersonaCard({
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
        'group border-border bg-background hover:border-foreground/40 relative flex h-full w-full flex-col items-start gap-2 rounded-lg border p-4 text-left transition-all hover:shadow-sm',
        'focus-visible:border-ring focus-visible:ring-ring/30 focus-visible:ring-[3px] focus-visible:outline-none'
      )}
    >
      <span className='flex w-full items-start justify-between gap-2'>
        <span className='bg-muted text-muted-foreground group-hover:bg-foreground/10 group-hover:text-foreground flex h-10 w-10 shrink-0 items-center justify-center rounded-md border'>
          {icon}
        </span>
        <ArrowRight className='text-muted-foreground group-hover:text-foreground mt-3 h-4 w-4 shrink-0 transition-transform group-hover:translate-x-0.5' />
      </span>
      <span className='flex items-center gap-2'>
        <span className='text-sm font-semibold'>{title}</span>
        {badge && (
          <span className='bg-foreground/10 text-foreground rounded-full px-2 py-0.5 text-[10px] font-medium'>
            {badge}
          </span>
        )}
      </span>
      <span className='text-muted-foreground text-xs leading-snug'>
        {description}
      </span>
      {footnote && (
        <span className='text-muted-foreground/80 mt-auto pt-1 text-[11px]'>
          {footnote}
        </span>
      )}
    </button>
  )
}
