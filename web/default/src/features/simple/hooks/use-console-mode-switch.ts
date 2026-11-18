// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { persistPersona } from '@/features/profile/lib/persist-persona'
import { ADVANCED_HOME, SIMPLE_HOME, type ConsoleMode } from '../lib/mode'

export function useConsoleModeSwitch(target: ConsoleMode) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const inFlight = useRef(false)
  const [switching, setSwitching] = useState(false)

  const switchMode = async () => {
    if (inFlight.current) return
    inFlight.current = true
    setSwitching(true)

    try {
      const simple = target === 'simple'
      const error = await persistPersona(simple ? 'casual' : 'dev')
      if (error) {
        toast.error(t('Could not switch. Please try again.'))
        return
      }
      await navigate({ to: simple ? SIMPLE_HOME : ADVANCED_HOME })
    } catch {
      toast.error(t('Could not switch. Please try again.'))
    } finally {
      inFlight.current = false
      setSwitching(false)
    }
  }

  return { switchMode, switching }
}
