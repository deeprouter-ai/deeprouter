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
import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useIsCasual } from '@/hooks/use-casual'
import { cn } from '@/lib/utils'
import type { PurposeSummary, SimplePurposeId } from '../types'

/**
 * Casual-persona-only extra hint per purpose. Tells the user which
 * client this purpose pairs with — answers the "but how do I actually
 * use this key?" question casual users have at this step.
 */
const CASUAL_PURPOSE_HINT: Record<SimplePurposeId, string> = {
  chat: 'Pair with Cherry Studio / Chatbox for chat, writing, translation.',
  coding: 'Pair with Cursor / Claude Code for AI coding assistance.',
  image: 'Use DALL·E or Flux models for text-to-image.',
  video: 'Generate short video clips from a text prompt.',
  voice: 'Speech-to-text (Whisper) or text-to-speech (TTS).',
  all: 'Lets the key call any model. Watch the cost.',
}

type ApiKeyPurposePickerProps = {
  options: PurposeSummary[]
  value?: SimplePurposeId
  onValueChange: (value: SimplePurposeId) => void
  isLoading?: boolean
}

/**
 * The 6-card grid that drives Simple-mode key creation.
 * PRD docs/tasks/api-key-simple-advanced-prd.md §4.1.
 */
export function ApiKeyPurposePicker({
  options,
  value,
  onValueChange,
  isLoading,
}: ApiKeyPurposePickerProps) {
  const { t } = useTranslation()
  const casual = useIsCasual()
  if (isLoading) {
    return (
      <div className='grid grid-cols-2 gap-3 lg:grid-cols-3'>
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className='bg-muted/30 h-32 animate-pulse rounded-lg'
          />
        ))}
      </div>
    )
  }

  return (
    <div className='grid grid-cols-2 gap-3 lg:grid-cols-3'>
      {options.map((option) => {
        const selected = option.id === value
        return (
          <button
            key={option.id}
            type='button'
            onClick={() => onValueChange(option.id)}
            className={cn(
              'group relative flex h-full min-h-[7.5rem] flex-col items-start gap-1.5 rounded-lg border bg-background p-3 text-left transition-all',
              'hover:border-foreground/40 hover:shadow-sm',
              selected
                ? 'border-foreground ring-foreground/15 bg-foreground/[0.025] ring-2'
                : 'border-border'
            )}
            aria-pressed={selected}
          >
            {selected && (
              <span className='bg-foreground text-background absolute end-2 top-2 inline-flex h-5 w-5 items-center justify-center rounded-full'>
                <Check className='h-3 w-3' />
              </span>
            )}
            <span className='text-2xl leading-none' aria-hidden>
              {option.icon}
            </span>
            <span className='text-sm font-medium leading-tight'>
              {option.label}
            </span>
            {option.desc && (
              <span className='text-muted-foreground line-clamp-2 text-xs leading-snug'>
                {option.desc}
              </span>
            )}
            {option.human_estimate && (
              <span className='text-muted-foreground/90 mt-auto text-[11px] leading-snug'>
                {option.human_estimate}
              </span>
            )}
            {casual && CASUAL_PURPOSE_HINT[option.id] && (
              <span className='text-muted-foreground/70 text-[11px] leading-snug'>
                💡 {t(CASUAL_PURPOSE_HINT[option.id])}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
