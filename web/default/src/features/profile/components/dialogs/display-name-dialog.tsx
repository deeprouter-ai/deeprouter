// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { updateUserProfile } from '../../api'

/** How long a display name may be, in characters — the backend's bound. */
export const DISPLAY_NAME_MAX_LENGTH = 20

type DisplayNameDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The name as it is now, which the field starts out holding. */
  current: string
  username: string
  /** Called once the new name is saved, so the page can read it back. */
  onRenamed: () => void
}

/**
 * Changes the signed-in user's own display name (Enterprise Org D48): the
 * name the member list, the reports and the alerts call them by. It takes no
 * password — the backend asks for none when the request carries the name
 * alone — and the username is not touched.
 */
export function DisplayNameDialog({
  open,
  onOpenChange,
  current,
  username,
  onRenamed,
}: DisplayNameDialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Change display name')}</DialogTitle>
          <DialogDescription>
            {t(
              'This is the name others see: in the member list, the reports and the alerts. Your username stays {{username}}.',
              { username }
            )}
          </DialogDescription>
        </DialogHeader>
        {/* The form lives inside the content, which is mounted only while
            the dialog is open: each opening starts from the current name. */}
        <DisplayNameForm
          current={current}
          onCancel={() => onOpenChange(false)}
          onSaved={() => {
            onRenamed()
            onOpenChange(false)
          }}
        />
      </DialogContent>
    </Dialog>
  )
}

type DisplayNameFormProps = {
  current: string
  onCancel: () => void
  onSaved: () => void
}

/** The field and the two buttons: what can be saved, and saving it. */
function DisplayNameForm({ current, onCancel, onSaved }: DisplayNameFormProps) {
  const { t } = useTranslation()
  const [name, setName] = useState(current)
  const [saving, setSaving] = useState(false)
  const trimmed = name.trim()
  const tooLong = [...trimmed].length > DISPLAY_NAME_MAX_LENGTH
  const savable = trimmed !== '' && !tooLong && trimmed !== current

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!savable) return
    setSaving(true)
    try {
      const response = await updateUserProfile({ display_name: trimmed })
      if (!response.success) {
        toast.error(response.message || t('Could not change the display name.'))
        return
      }
      // The top-right menu reads the stored sign-in; keep it in step.
      const { user, setUser } = useAuthStore.getState().auth
      if (user) setUser({ ...user, display_name: trimmed })
      toast.success(t('Display name changed'))
      onSaved()
    } catch (_error) {
      toast.error(t('Could not change the display name.'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className='my-6 space-y-2'>
        <Label htmlFor='display-name'>{t('Display Name')}</Label>
        <Input
          id='display-name'
          value={name}
          onChange={(event) => setName(event.target.value)}
          disabled={saving}
          required
          autoFocus
          autoComplete='nickname'
          maxLength={DISPLAY_NAME_MAX_LENGTH * 2}
        />
        <p
          className={
            tooLong
              ? 'text-destructive text-xs'
              : 'text-muted-foreground text-xs'
          }
        >
          {t('Up to {{max}} characters.', { max: DISPLAY_NAME_MAX_LENGTH })}
        </p>
      </div>

      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          onClick={onCancel}
          disabled={saving}
        >
          {t('Cancel')}
        </Button>
        <Button type='submit' disabled={saving || !savable}>
          {saving && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
          {t('Save')}
        </Button>
      </DialogFooter>
    </form>
  )
}
