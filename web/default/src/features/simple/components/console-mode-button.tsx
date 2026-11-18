// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { ArrowRightLeft, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useConsoleModeSwitch } from '../hooks/use-console-mode-switch'
import type { ConsoleMode } from '../lib/mode'

export function ConsoleModeButton(props: { target: ConsoleMode }) {
  const { t } = useTranslation()
  const { switchMode, switching } = useConsoleModeSwitch(props.target)
  const simple = props.target === 'simple'
  const label = simple
    ? t('Switch to simple mode')
    : t('Switch to professional mode')

  return (
    <Button
      type='button'
      variant='outline'
      size='sm'
      className='h-8 gap-1.5 rounded-[7px] max-[359px]:px-1.5 max-[359px]:text-xs max-[359px]:[&_svg]:hidden'
      aria-label={label}
      aria-busy={switching}
      title={label}
      disabled={switching}
      onClick={switchMode}
    >
      {switching ? (
        <Loader2 className='animate-spin' aria-hidden='true' />
      ) : (
        <ArrowRightLeft aria-hidden='true' />
      )}
      {simple ? t('Simple mode') : t('Professional mode')}
    </Button>
  )
}
