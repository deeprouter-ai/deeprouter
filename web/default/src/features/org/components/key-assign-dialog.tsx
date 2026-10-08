// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { assignOrgKey } from '../api'
import type { OrgKey, OrgKeyGrant, OrgKeyHolder } from '../types'
import { HolderPicker } from './holder-picker'

type KeyAssignDialogProps = {
  /** The key to hand over; null keeps the dialog closed. */
  assigning: OrgKey | null
  /** The members the viewer may hand a key to. */
  assignees: OrgKeyHolder[]
  onClose: () => void
  /** The key changed hands; the grant carries its value if it went to a service account. */
  onAssigned: (grant: OrgKeyGrant) => void
}

/**
 * Hands a key to another member (Enterprise Org PRD §3). The key gets a new
 * value on the way — whatever its last holder has in their tools stops working
 * — and the dialog says so before it happens, along with how the new holder
 * comes by the new one.
 */
export function KeyAssignDialog(props: KeyAssignDialogProps) {
  return (
    <Dialog
      open={props.assigning !== null}
      onOpenChange={(open) => !open && props.onClose()}
    >
      <DialogContent>
        {/* Keyed so each opening starts with nobody chosen. */}
        {props.assigning && (
          <KeyAssignForm
            key={props.assigning.id}
            {...props}
            assigning={props.assigning}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

/** The form inside the dialog, mounted fresh on every opening. */
function KeyAssignForm({
  assigning,
  assignees,
  onClose,
  onAssigned,
}: KeyAssignDialogProps & { assigning: OrgKey }) {
  const { t } = useTranslation()
  // Nobody is chosen to begin with: handing a key over is not something to do
  // by pressing Enter on whoever happened to be first.
  const [holderId, setHolderId] = useState(0)
  const [saving, setSaving] = useState(false)
  // Whoever holds the key already is not someone to hand it to.
  const candidates = assignees.filter(
    (member) => member.id !== assigning.holder_id
  )
  const chosen = candidates.find((member) => member.id === holderId)

  const save = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!chosen) return
    setSaving(true)
    try {
      const res = await assignOrgKey(assigning.id, chosen.id)
      if (res.success && res.data) {
        onAssigned(res.data)
        onClose()
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={save} className='grid gap-4'>
      <DialogHeader>
        <DialogTitle>
          {t('Assign “{{name}}”', { name: assigning.name })}
        </DialogTitle>
        <DialogDescription>
          {assigning.holder_is_owner
            ? t(
                'It has not been handed to anyone yet. It gets a new value when it is.'
              )
            : t(
                'It is held by {{holder}} now. It gets a new value on the way, so the one {{holder}} has stops working at once.',
                { holder: assigning.holder || t('Someone who has left') }
              )}
        </DialogDescription>
      </DialogHeader>

      <div className='grid gap-2'>
        <Label htmlFor='org-key-assignee'>{t('Assign to')}</Label>
        {candidates.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('There is nobody else you can assign this key to.')}
          </p>
        ) : (
          <HolderPicker
            id='org-key-assignee'
            holders={candidates}
            value={holderId}
            onChange={setHolderId}
            placeholder={t('Choose a member')}
          />
        )}
        {chosen && (
          <p className='text-muted-foreground text-xs'>
            {chosen.is_service
              ? t(
                  'A service account cannot sign in, so the new value is shown to you once, right after this. Save it then.'
                )
              : t(
                  'The new value is not shown to you. {{name}} installs it into their own tools with one-click setup, from their API keys page.',
                  { name: chosen.name }
                )}
          </p>
        )}
      </div>

      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          onClick={onClose}
          disabled={saving}
        >
          {t('Cancel')}
        </Button>
        <Button type='submit' disabled={saving || !chosen}>
          {saving ? t('Saving...') : t('Assign')}
        </Button>
      </DialogFooter>
    </form>
  )
}
