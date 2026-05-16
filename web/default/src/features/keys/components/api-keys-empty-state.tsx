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
import { ArrowRight, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'

type ApiKeysEmptyStateProps = {
  onCreate: () => void
}

/**
 * CTA-styled empty state shown when the user has zero API keys.
 * Drives the very first key-creation in the funnel (PRD §4.4).
 */
export function ApiKeysEmptyState({ onCreate }: ApiKeysEmptyStateProps) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col items-center justify-center gap-4 py-12 text-center'>
      <div className='bg-foreground/5 ring-foreground/10 flex h-14 w-14 items-center justify-center rounded-full ring-1'>
        <KeyRound className='text-foreground/80 h-6 w-6' />
      </div>
      <div className='space-y-1'>
        <h3 className='text-base font-semibold'>
          {t('Create your first API key')}
        </h3>
        <p className='text-muted-foreground max-w-sm text-sm'>
          {t(
            '30 seconds to get a key, then drop it into any AI client — no setup hassle.'
          )}
        </p>
      </div>
      <Button onClick={onCreate} className='gap-1.5'>
        {t('Get Started')}
        <ArrowRight className='h-4 w-4' />
      </Button>
      <p className='text-muted-foreground/80 text-xs'>
        {t("Don't know which model? We'll pick a good one for you.")}
      </p>
    </div>
  )
}
