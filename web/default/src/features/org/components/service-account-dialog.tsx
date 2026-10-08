// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { createOrgServiceAccount } from '../api'
import type { OrgDepartment } from '../types'
import { OrgSelect } from './org-select'

/** Longest service account name the backend accepts, in characters. */
const SERVICE_ACCOUNT_NAME_MAX_LENGTH = 20

type ServiceAccountDialogProps = {
  open: boolean
  departments: OrgDepartment[]
  onClose: () => void
  onCreated: () => void
}

/** Adds a service account: a member for a CI pipeline or a bot. */
export function ServiceAccountDialog(props: ServiceAccountDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={(open) => !open && props.onClose()}>
      <DialogContent>
        {props.open && <ServiceAccountForm {...props} />}
      </DialogContent>
    </Dialog>
  )
}

/** The form inside the dialog, mounted fresh on every opening. */
function ServiceAccountForm({
  departments,
  onClose,
  onCreated,
}: ServiceAccountDialogProps) {
  const { t } = useTranslation()
  const defaultDepartment =
    departments.find((department) => department.is_default) ?? departments[0]
  const [name, setName] = useState('')
  const [departmentId, setDepartmentId] = useState(defaultDepartment?.id ?? 0)
  const [saving, setSaving] = useState(false)
  const trimmed = name.trim()

  const save = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!trimmed) return
    setSaving(true)
    try {
      const res = await createOrgServiceAccount({
        name: trimmed,
        department_id: departmentId,
      })
      if (res.success) {
        toast.success(t('Service account added'))
        onCreated()
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
        <DialogTitle>{t('Add service account')}</DialogTitle>
        <DialogDescription>
          {t(
            'For a CI pipeline or a bot. It holds keys and shows up in usage like any member, and it can never sign in.'
          )}
        </DialogDescription>
      </DialogHeader>

      <div className='grid gap-2'>
        <Label htmlFor='org-service-account-name'>{t('Name')}</Label>
        <Input
          id='org-service-account-name'
          value={name}
          maxLength={SERVICE_ACCOUNT_NAME_MAX_LENGTH}
          placeholder={t('e.g. CI pipeline')}
          autoFocus
          onChange={(e) => setName(e.target.value)}
        />
      </div>

      <div className='grid gap-2'>
        <Label htmlFor='org-service-account-department'>
          {t('Department')}
        </Label>
        <OrgSelect
          id='org-service-account-department'
          options={departments.map((department) => ({
            value: department.id,
            label: department.name,
          }))}
          value={departmentId}
          onChange={setDepartmentId}
        />
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
        <Button type='submit' disabled={saving || !trimmed}>
          {saving ? t('Saving...') : t('Add')}
        </Button>
      </DialogFooter>
    </form>
  )
}
