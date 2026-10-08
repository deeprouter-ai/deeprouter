// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { createOrgDepartment, renameOrgDepartment } from '../api'
import type { OrgDepartment } from '../types'

/** Longest department name the backend accepts, in characters. */
const DEPARTMENT_NAME_MAX_LENGTH = 64

type DepartmentDialogProps = {
  open: boolean
  /** Set to rename this department; leave out to create a new one. */
  department?: OrgDepartment
  onClose: () => void
  onSaved: () => void
}

/** Creates a department, or renames an existing one. */
export function DepartmentDialog(props: DepartmentDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={(open) => !open && props.onClose()}>
      <DialogContent>
        {/* Keyed so each opening starts from that department's name. */}
        {props.open && (
          <DepartmentForm key={props.department?.id ?? 'new'} {...props} />
        )}
      </DialogContent>
    </Dialog>
  )
}

/** The form inside the dialog, mounted fresh on every opening. */
function DepartmentForm({
  department,
  onClose,
  onSaved,
}: DepartmentDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState(department?.name ?? '')
  const [saving, setSaving] = useState(false)
  const trimmed = name.trim()

  const save = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!trimmed) return
    setSaving(true)
    try {
      const res = department
        ? await renameOrgDepartment(department.id, trimmed)
        : await createOrgDepartment(trimmed)
      if (res.success) {
        toast.success(
          department ? t('Department renamed') : t('Department created')
        )
        onSaved()
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
          {department ? t('Rename department') : t('New department')}
        </DialogTitle>
      </DialogHeader>

      <div className='grid gap-2'>
        <Label htmlFor='org-department-name'>{t('Department name')}</Label>
        <Input
          id='org-department-name'
          value={name}
          maxLength={DEPARTMENT_NAME_MAX_LENGTH}
          autoFocus
          onChange={(e) => setName(e.target.value)}
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
          {saving ? t('Saving...') : t('Save')}
        </Button>
      </DialogFooter>
    </form>
  )
}
