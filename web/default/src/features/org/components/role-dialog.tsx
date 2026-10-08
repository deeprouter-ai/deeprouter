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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { createOrgRole, updateOrgRole } from '../api'
import {
  groupByResource,
  impliedReads,
  permissionHint,
  permissionLabel,
  resourceLabel,
  scopeLabel,
} from '../lib/permissions'
import type { OrgPermissionCatalog, OrgRole, OrgRoleScope } from '../types'
import { OrgSelect } from './org-select'

/** Longest role name the backend accepts, in characters. */
const ROLE_NAME_MAX_LENGTH = 64

/** The one primitive that only a role reaching the whole organization can hold. */
const WHOLE_ORGANIZATION_ONLY = 'audit.read'

type RoleDialogProps = {
  open: boolean
  /** Set to change this custom role; leave out to build a new one. */
  role?: OrgRole
  catalog: OrgPermissionCatalog
  onClose: () => void
  onSaved: () => void
}

/** Builds a custom role from the permission primitives, or changes one. */
export function RoleDialog(props: RoleDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={(open) => !open && props.onClose()}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        {/* Keyed so each opening starts from that role's values. */}
        {props.open && <RoleForm key={props.role?.id ?? 'new'} {...props} />}
      </DialogContent>
    </Dialog>
  )
}

/** The form inside the dialog, mounted fresh on every opening. */
function RoleForm({ role, catalog, onClose, onSaved }: RoleDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState(role?.name ?? '')
  const [scope, setScope] = useState<OrgRoleScope>(role?.scope ?? 'org')
  const [selected, setSelected] = useState<string[]>(role?.permissions ?? [])
  const [saving, setSaving] = useState(false)
  const trimmed = name.trim()

  // The audit log belongs to the organization as a whole, so a role that
  // reaches only departments cannot read it; the tick is dropped with the
  // scope rather than left for the backend to refuse.
  const allowed = (primitive: string) =>
    scope === 'org' || primitive !== WHOLE_ORGANIZATION_ONLY
  // A write includes the read of the same resource (PRD §2): shown ticked and
  // locked, so the form says what the role will really grant.
  const chosen = selected.filter(allowed)
  const implied = impliedReads(chosen)
  const granted = catalog.primitives.filter(
    (primitive) => chosen.includes(primitive) || implied.has(primitive)
  )

  const toggle = (primitive: string, on: boolean) =>
    setSelected((current) =>
      on
        ? [...current, primitive]
        : current.filter((other) => other !== primitive)
    )

  const save = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!trimmed || granted.length === 0) return
    setSaving(true)
    try {
      const input = { name: trimmed, scope, permissions: granted }
      const res = role
        ? await updateOrgRole(role.id, input)
        : await createOrgRole(input)
      if (res.success) {
        toast.success(role ? t('Role updated') : t('Role created'))
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
        <DialogTitle>{role ? t('Edit role') : t('New role')}</DialogTitle>
        <DialogDescription>
          {role
            ? t(
                'Changes apply at once to everyone who holds this role — they do not need to sign in again.'
              )
            : t(
                'Pick what holders of this role may do, and how far that reaches.'
              )}
        </DialogDescription>
      </DialogHeader>

      <div className='grid gap-4 sm:grid-cols-2'>
        <div className='grid gap-2'>
          <Label htmlFor='org-role-name'>{t('Role name')}</Label>
          <Input
            id='org-role-name'
            value={name}
            maxLength={ROLE_NAME_MAX_LENGTH}
            autoFocus
            onChange={(e) => setName(e.target.value)}
          />
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='org-role-scope'>{t('Reach')}</Label>
          <OrgSelect<OrgRoleScope>
            id='org-role-scope'
            options={[
              { value: 'org', label: scopeLabel(t, 'org') },
              { value: 'dept', label: scopeLabel(t, 'dept') },
            ]}
            value={scope}
            onChange={setScope}
          />
        </div>
      </div>
      <p className='text-muted-foreground -mt-2 text-xs'>
        {scope === 'dept'
          ? t(
              'Holders act only in the departments they manage: their own, plus any you add for them on the members page.'
            )
          : t('Holders act across every department.')}
      </p>

      <div className='grid gap-4'>
        {groupByResource(catalog.primitives).map((group) => (
          <fieldset key={group.resource} className='grid gap-2'>
            <legend className='text-muted-foreground mb-1 text-xs font-semibold'>
              {resourceLabel(t, group.resource)}
            </legend>
            <div className='grid gap-x-4 gap-y-3 sm:grid-cols-2'>
              {group.primitives.map((primitive) => {
                const id = `org-role-permission-${primitive}`
                const locked = implied.has(primitive) || !allowed(primitive)
                return (
                  <div key={primitive} className='flex items-start gap-2'>
                    <Checkbox
                      id={id}
                      className='mt-0.5 data-disabled:cursor-not-allowed data-disabled:opacity-50'
                      checked={granted.includes(primitive)}
                      disabled={locked}
                      onCheckedChange={(checked) =>
                        toggle(primitive, checked === true)
                      }
                    />
                    <Label
                      htmlFor={id}
                      className='flex-col items-start gap-0.5 text-left leading-5 font-normal'
                    >
                      <span>{permissionLabel(t, primitive)}</span>
                      <span className='text-muted-foreground text-xs'>
                        {!allowed(primitive)
                          ? t(
                              'Only for a role that reaches the whole organization.'
                            )
                          : implied.has(primitive)
                            ? t(
                                'Included with the other permissions you ticked.'
                              )
                            : permissionHint(t, primitive)}
                      </span>
                    </Label>
                  </div>
                )
              })}
            </div>
          </fieldset>
        ))}
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
        <Button
          type='submit'
          disabled={saving || !trimmed || granted.length === 0}
        >
          {saving ? t('Saving...') : t('Save')}
        </Button>
      </DialogFooter>
    </form>
  )
}
