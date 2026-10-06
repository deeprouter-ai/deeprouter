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
import { Label } from '@/components/ui/label'
import { updateOrgMember } from '../api'
import {
  assignableRoles,
  orgRoleLabel,
  roleLockReason,
  sitsInDefaultDepartment,
} from '../lib/roles'
import type { OrgDepartment, OrgMember, OrgMembership, OrgRole } from '../types'
import { OrgSelect } from './org-select'

type MemberEditDialogProps = {
  /** The member being edited; null keeps the dialog closed. */
  member: OrgMember | null
  roles: OrgRole[]
  departments: OrgDepartment[]
  actor: OrgMembership
  onClose: () => void
  onSaved: () => void
}

/** Changes one member's role and department. */
export function MemberEditDialog(props: MemberEditDialogProps) {
  const { member, onClose } = props
  return (
    <Dialog open={member !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        {/* Keyed by member so the selects start from that member's values. */}
        {member && (
          <MemberEditForm key={member.id} {...props} member={member} />
        )}
      </DialogContent>
    </Dialog>
  )
}

/** The form inside the dialog, for one member. */
function MemberEditForm({
  member,
  roles,
  departments,
  actor,
  onClose,
  onSaved,
}: MemberEditDialogProps & { member: OrgMember }) {
  const { t } = useTranslation()
  const [roleId, setRoleId] = useState(member.role_id)
  const [departmentId, setDepartmentId] = useState(member.department_id)
  const [saving, setSaving] = useState(false)

  const lockReason = roleLockReason(t, member, roles, actor)
  // A locked role still has to show what it is, so it is the one option.
  const roleOptions = lockReason
    ? roles.filter((role) => role.id === member.role_id)
    : assignableRoles(roles, actor)

  // The owner and admins sit in the default department. Choosing such a role
  // shows where the member will be; the backend does the moving.
  const defaultDepartment = departments.find((d) => d.is_default)
  const pinnedTo = sitsInDefaultDepartment(roles.find((r) => r.id === roleId))
    ? defaultDepartment
    : undefined
  const shownDepartmentId = pinnedTo ? pinnedTo.id : departmentId

  const save = async () => {
    const patch: { role_id?: number; department_id?: number } = {}
    if (!lockReason && roleId !== member.role_id) patch.role_id = roleId
    if (!pinnedTo && departmentId !== member.department_id) {
      patch.department_id = departmentId
    }
    if (Object.keys(patch).length === 0) {
      onClose()
      return
    }
    setSaving(true)
    try {
      const res = await updateOrgMember(member.id, patch)
      if (res.success) {
        toast.success(t('Member updated'))
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
    <>
      <DialogHeader>
        <DialogTitle>{t('Edit member')}</DialogTitle>
        <DialogDescription>
          {member.display_name || member.username}
        </DialogDescription>
      </DialogHeader>

      <div className='grid gap-4'>
        <div className='grid gap-2'>
          <Label htmlFor='org-member-role'>{t('Role')}</Label>
          <OrgSelect
            id='org-member-role'
            options={roleOptions.map((role) => ({
              value: role.id,
              label: orgRoleLabel(t, role.name),
            }))}
            value={roleId}
            disabled={lockReason !== null}
            onChange={setRoleId}
          />
          {lockReason && (
            <p className='text-muted-foreground text-xs'>{lockReason}</p>
          )}
        </div>

        <div className='grid gap-2'>
          <Label htmlFor='org-member-department'>{t('Department')}</Label>
          <OrgSelect
            id='org-member-department'
            options={departments.map((department) => ({
              value: department.id,
              label: department.name,
            }))}
            value={shownDepartmentId}
            disabled={pinnedTo !== undefined}
            onChange={setDepartmentId}
          />
          {pinnedTo && (
            <p className='text-muted-foreground text-xs'>
              {t('The owner and admins always belong to “{{name}}”.', {
                name: pinnedTo.name,
              })}
            </p>
          )}
        </div>
      </div>

      <DialogFooter>
        <Button variant='outline' onClick={onClose} disabled={saving}>
          {t('Cancel')}
        </Button>
        <Button onClick={save} disabled={saving}>
          {saving ? t('Saving...') : t('Save')}
        </Button>
      </DialogFooter>
    </>
  )
}
