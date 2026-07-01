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
import { useCallback, useRef, useState } from 'react'
import { ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  getTelemetryConsent,
  updateTelemetryConsent,
} from '@/features/profile/api'

const PROMPT_SEEN_KEY = 'deeprouter.skillTelemetryConsentPromptSeen'

function hasSeenPrompt() {
  if (typeof window === 'undefined') return true
  return window.localStorage.getItem(PROMPT_SEEN_KEY) === 'true'
}

function markPromptSeen() {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(PROMPT_SEEN_KEY, 'true')
}

type PendingAction = () => Promise<void> | void

export function useSkillTelemetryConsentPrompt() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const pendingActionRef = useRef<PendingAction | null>(null)

  const runPendingAction = useCallback(async () => {
    const action = pendingActionRef.current
    pendingActionRef.current = null
    if (action) {
      await action()
    }
  }, [])

  const runWithConsentPrompt = useCallback(async (action: PendingAction) => {
    if (hasSeenPrompt()) {
      await action()
      return
    }

    try {
      const response = await getTelemetryConsent()
      if (response.data?.tier2_telemetry_consent === true) {
        markPromptSeen()
        await action()
        return
      }
    } catch (_error) {
      await action()
      return
    }

    pendingActionRef.current = action
    setOpen(true)
  }, [])

  const continueWithoutEnabling = useCallback(async () => {
    markPromptSeen()
    setOpen(false)
    await runPendingAction()
  }, [runPendingAction])

  const enableAndContinue = useCallback(async () => {
    setSaving(true)
    try {
      const response = await updateTelemetryConsent({
        tier2_telemetry_consent: true,
      })
      if (!response.success) {
        throw new Error(
          response.message || 'Failed to update telemetry consent'
        )
      }
      markPromptSeen()
      setOpen(false)
      toast.success(t('Tier 2 telemetry consent enabled'))
      await runPendingAction()
    } catch (_error) {
      toast.error(t('Failed to update telemetry consent'))
    } finally {
      setSaving(false)
    }
  }, [runPendingAction, t])

  const prompt = (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogContent className='w-[calc(100vw-2rem)] max-w-md'>
        <AlertDialogHeader>
          <AlertDialogMedia>
            <ShieldCheck className='size-5' aria-hidden='true' />
          </AlertDialogMedia>
          <AlertDialogTitle>
            {t('Enable Skill and Runner usage details?')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'DeepRouter can store Skill and Runner usage metadata such as downloaded Skills, token counts, cost, model tier, event time, and success status. This helps you and Super Admins review usage, but raw prompts and provider payloads are never stored.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel
            disabled={saving}
            onClick={() => void continueWithoutEnabling()}
          >
            {t('Download without usage details')}
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={saving}
            onClick={() => void enableAndContinue()}
          >
            {saving ? t('Saving...') : t('Enable and download')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )

  return { prompt, runWithConsentPrompt }
}
