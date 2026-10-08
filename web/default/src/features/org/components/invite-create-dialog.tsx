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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { CopyButton } from '@/components/copy-button'
import { createOrgInvite } from '../api'
import {
  assignableRoles,
  orgInviteLink,
  orgRoleLabel,
  sitsInDefaultDepartment,
} from '../lib/roles'
import type { OrgDepartment, OrgInvite, OrgMembership, OrgRole } from '../types'
import { OrgSelect } from './org-select'

type InviteCreateDialogProps = {
  open: boolean
  roles: OrgRole[]
  departments: OrgDepartment[]
  actor: OrgMembership
  onClose: () => void
  onCreated: () => void
}

/**
 * Issues an invite link. One link fixes a role and a department and admits
 * everyone who opens it until it expires or is revoked.
 */
export function InviteCreateDialog(props: InviteCreateDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={(open) => !open && props.onClose()}>
      <DialogContent className='sm:max-w-md'>
        {props.open && <InviteCreateForm {...props} />}
      </DialogContent>
    </Dialog>
  )
}

/** The two steps inside the dialog: choose role and department, then copy the link. */
function InviteCreateForm({
  roles,
  departments,
  actor,
  onClose,
  onCreated,
}: InviteCreateDialogProps) {
  const { t } = useTranslation()
  const roleOptions = assignableRoles(roles, actor)
  // Most invites are for ordinary staff, so that is where the form starts.
  const defaultRole =
    roleOptions.find((role) => role.is_preset && role.name === 'staff') ??
    roleOptions[0]
  const defaultDepartment =
    departments.find((department) => department.is_default) ?? departments[0]
  const [roleId, setRoleId] = useState(defaultRole?.id ?? 0)
  const [departmentId, setDepartmentId] = useState(defaultDepartment?.id ?? 0)
  const [saving, setSaving] = useState(false)
  const [created, setCreated] = useState<OrgInvite | null>(null)

  // Admins sit in the default department, so a link that makes admins leads
  // nowhere else.
  const pinnedTo = sitsInDefaultDepartment(
    roleOptions.find((role) => role.id === roleId)
  )
    ? departments.find((department) => department.is_default)
    : undefined
  const shownDepartmentId = pinnedTo ? pinnedTo.id : departmentId

  const create = async (event: React.FormEvent) => {
    event.preventDefault()
    setSaving(true)
    try {
      const res = await createOrgInvite({
        role_id: roleId,
        department_id: shownDepartmentId,
      })
      if (res.success && res.data) {
        setCreated(res.data)
        onCreated()
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setSaving(false)
    }
  }

  if (created) {
    const link = orgInviteLink(created.code)
    return (
      <div className='grid gap-4'>
        <DialogHeader>
          <DialogTitle>{t('Invite link ready')}</DialogTitle>
          <DialogDescription>
            {t(
              'Send this link to the people you are inviting. Everyone who signs up through it joins your organization with the role and department you chose.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='flex items-center gap-2'>
          <Input
            readOnly
            value={link}
            aria-label={t('Invite link')}
            className='font-mono text-xs'
            onFocus={(e) => e.currentTarget.select()}
          />
          <CopyButton
            value={link}
            variant='outline'
            tooltip={t('Copy link')}
            aria-label={t('Copy link')}
          />
        </div>
        <p className='text-muted-foreground text-xs'>
          {t(
            'The link works for 7 days and for any number of people. You can revoke it at any time under “Invite links”.'
          )}
        </p>
        <DialogFooter>
          <Button onClick={onClose}>{t('Done')}</Button>
        </DialogFooter>
      </div>
    )
  }

  return (
    <form onSubmit={create} className='grid gap-4'>
      <DialogHeader>
        <DialogTitle>{t('Invite members')}</DialogTitle>
        <DialogDescription>
          {t(
            'Choose the role and department new members get, then share the link.'
          )}
        </DialogDescription>
      </DialogHeader>

      <div className='grid gap-2'>
        <Label htmlFor='org-invite-role'>{t('Role')}</Label>
        <OrgSelect
          id='org-invite-role'
          options={roleOptions.map((role) => ({
            value: role.id,
            label: orgRoleLabel(t, role.name),
          }))}
          value={roleId}
          onChange={setRoleId}
        />
      </div>

      <div className='grid gap-2'>
        <Label htmlFor='org-invite-department'>{t('Department')}</Label>
        <OrgSelect
          id='org-invite-department'
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

      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          onClick={onClose}
          disabled={saving}
        >
          {t('Cancel')}
        </Button>
        <Button type='submit' disabled={saving || !roleId}>
          {saving ? t('Creating...') : t('Create link')}
        </Button>
      </DialogFooter>
    </form>
  )
}
