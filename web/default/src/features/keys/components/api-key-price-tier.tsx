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
import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { cn } from '@/lib/utils'
import type { PriceTierSummary, SimplePriceTierId } from '../types'

const TIER_ICONS: Record<SimplePriceTierId, string> = {
  economy: '💰',
  standard: '🎯',
  premium: '🚀',
  ultra: '👑',
}

type ApiKeyPriceTierProps = {
  tiers: PriceTierSummary[]
  value?: SimplePriceTierId
  onValueChange: (value: SimplePriceTierId) => void
}

/**
 * Per-key price cap when purpose = 'all' (Auto).
 * Picking a tier flagged `requires_confirm` (Ultra) pops a confirm dialog.
 * PRD docs/tasks/api-key-simple-advanced-prd.md §3.5.
 */
export function ApiKeyPriceTier({
  tiers,
  value,
  onValueChange,
}: ApiKeyPriceTierProps) {
  const { t } = useTranslation()
  const [pendingTier, setPendingTier] = useState<SimplePriceTierId | null>(null)

  const handleSelect = (tier: PriceTierSummary) => {
    if (tier.requires_confirm && tier.id !== value) {
      setPendingTier(tier.id)
      return
    }
    onValueChange(tier.id)
  }

  const confirmingTier = tiers.find((t) => t.id === pendingTier)

  return (
    <>
      <div className='space-y-2'>
        <p className='text-foreground text-xs font-medium'>
          {t('Maximum cost tier')}
        </p>
        <div className='grid gap-2'>
          {tiers.map((tier) => {
            const selected = tier.id === value
            return (
              <button
                key={tier.id}
                type='button'
                onClick={() => handleSelect(tier)}
                aria-pressed={selected}
                className={cn(
                  'group flex items-start gap-3 rounded-lg border bg-background p-3 text-left transition-all',
                  'hover:border-foreground/40 hover:shadow-sm',
                  selected
                    ? 'border-foreground ring-foreground/15 bg-foreground/[0.025] ring-2'
                    : 'border-border'
                )}
              >
                <span
                  className={cn(
                    'mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-base leading-none',
                    selected
                      ? 'bg-foreground text-background'
                      : 'bg-muted text-muted-foreground'
                  )}
                  aria-hidden
                >
                  {selected ? (
                    <Check className='h-3 w-3' />
                  ) : (
                    <span className='text-xs'>{TIER_ICONS[tier.id]}</span>
                  )}
                </span>
                <span className='flex min-w-0 flex-1 flex-col gap-0.5'>
                  <span className='flex items-baseline justify-between gap-2'>
                    <span className='text-sm font-medium'>{tier.label}</span>
                    <span className='text-muted-foreground text-[11px]'>
                      {tier.price_range}
                    </span>
                  </span>
                  {tier.desc && (
                    <span className='text-muted-foreground text-xs leading-snug'>
                      {tier.desc}
                    </span>
                  )}
                </span>
              </button>
            )
          })}
        </div>
      </div>
      <AlertDialog
        open={!!confirmingTier}
        onOpenChange={(o) => {
          if (!o) setPendingTier(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Enable {{tier}} tier?', { tier: confirmingTier?.label })}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This tier removes the per-request price cap and may incur significantly higher charges. Make sure you monitor usage.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setPendingTier(null)}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (pendingTier) onValueChange(pendingTier)
                setPendingTier(null)
              }}
            >
              {t('Enable')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
