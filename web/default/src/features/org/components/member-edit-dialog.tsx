// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { type OrgMemberPatch, updateOrgMember } from '../api'
import {
  assignableRoles,
  managesDepartments,
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

/** Changes one member's role, department and the departments they manage. */
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

/** Whether two lists hold the same ids, in any order. */
function sameIds(a: number[], b: number[]): boolean {
  return a.length === b.length && a.every((id) => b.includes(id))
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
  // The departments ticked besides the member's own. Their own is not kept
  // here: it follows the department field.
  const alreadyAdded = member.managed_department_ids.filter(
    (id) => id !== member.department_id
  )
  const [added, setAdded] = useState(alreadyAdded)
  const [saving, setSaving] = useState(false)

  const lockReason = roleLockReason(t, member, roles, actor)
  // A locked role still has to show what it is, so it is the one option.
  const roleOptions = lockReason
    ? roles.filter((role) => role.id === member.role_id)
    : assignableRoles(roles, actor)
  const role = roles.find((r) => r.id === roleId)

  // The owner and admins sit in the default department. Choosing such a role
  // shows where the member will be; the backend does the moving.
  const defaultDepartment = departments.find((d) => d.is_default)
  const pinnedTo = sitsInDefaultDepartment(role) ? defaultDepartment : undefined
  const shownDepartmentId = pinnedTo ? pinnedTo.id : departmentId

  // A department-scoped role manages the member's own department, and any
  // others ticked here (PRD D28).
  const manages = managesDepartments(role)
  const nowAdded = added.filter((id) => id !== shownDepartmentId)

  const save = async () => {
    const patch: OrgMemberPatch = {}
    if (!lockReason && roleId !== member.role_id) patch.role_id = roleId
    if (!pinnedTo && departmentId !== member.department_id) {
      patch.department_id = departmentId
    }
    // Sent only when the ticks changed: giving a member such a role, or
    // moving them, already makes them manage their own department.
    if (manages && !sameIds(nowAdded, alreadyAdded)) {
      patch.managed_department_ids = [shownDepartmentId, ...nowAdded]
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
            options={roleOptions.map((option) => ({
              value: option.id,
              label: orgRoleLabel(t, option.name),
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

        {manages && (
          <fieldset className='grid gap-2'>
            <legend className='mb-2 text-sm leading-none font-medium'>
              {t('Departments they manage')}
            </legend>
            <p className='text-muted-foreground text-xs'>
              {t(
                'This role reaches the departments its holder manages: always their own, plus any you tick here.'
              )}
            </p>
            <div className='grid gap-2 sm:grid-cols-2'>
              {departments.map((department) => {
                const own = department.id === shownDepartmentId
                const id = `org-member-manages-${department.id}`
                return (
                  <div key={department.id} className='flex items-center gap-2'>
                    <Checkbox
                      id={id}
                      className='data-disabled:cursor-not-allowed data-disabled:opacity-50'
                      checked={own || added.includes(department.id)}
                      disabled={own}
                      onCheckedChange={(checked) =>
                        setAdded((ids) =>
                          checked === true
                            ? [...ids, department.id]
                            : ids.filter((other) => other !== department.id)
                        )
                      }
                    />
                    <Label htmlFor={id} className='font-normal'>
                      {department.name}
                    </Label>
                    {/* Why this one cannot be unticked. */}
                    {own && (
                      <span className='text-muted-foreground text-xs'>
                        {t('their department')}
                      </span>
                    )}
                  </div>
                )
              })}
            </div>
          </fieldset>
        )}
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
