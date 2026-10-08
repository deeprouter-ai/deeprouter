// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Check, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { adoptOrgRolePack } from '../api'
import {
  permissionLabel,
  rolePackHint,
  rolePackLabel,
  scopeLabel,
} from '../lib/permissions'
import type { OrgRole, OrgRolePack } from '../types'

type RolePacksProps = {
  packs: OrgRolePack[]
  /** The organization's roles, to tell which packs it already has. */
  roles: OrgRole[]
  /** Whether the viewer may adopt a pack; without it the packs are only shown. */
  canAdopt: boolean
  onAdopted: () => void
}

/**
 * The role packs the platform offers (Enterprise Org PRD §2): ready-made
 * custom roles for common jobs. Adopting one copies it into the organization
 * in one click, under the name shown here; from then on it is the
 * organization's own role, changed and deleted like any other.
 */
export function RolePacks({
  packs,
  roles,
  canAdopt,
  onAdopted,
}: RolePacksProps) {
  const { t } = useTranslation()
  const [adopting, setAdopting] = useState<string | null>(null)

  const adopt = async (pack: OrgRolePack) => {
    setAdopting(pack.key)
    try {
      const res = await adoptOrgRolePack(pack.key, rolePackLabel(t, pack))
      if (res.success) {
        toast.success(
          t('“{{name}}” added to your roles', { name: rolePackLabel(t, pack) })
        )
        onAdopted()
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setAdopting(null)
    }
  }

  return (
    <div className='grid gap-3 md:grid-cols-3'>
      {packs.map((pack) => {
        const name = rolePackLabel(t, pack)
        // A role is its own once adopted and may be renamed, so this only
        // knows the pack is there while the role still carries its name.
        const adopted = roles.some((role) => role.name === name)
        return (
          <section
            key={pack.key}
            aria-label={name}
            className='bg-card flex flex-col gap-3 rounded-xl border p-4'
          >
            <div className='grid gap-1'>
              <h3 className='font-medium'>{name}</h3>
              <p className='text-muted-foreground text-sm'>
                {rolePackHint(t, pack)}
              </p>
            </div>
            <div className='text-muted-foreground text-xs'>
              {t('Reach')}: {scopeLabel(t, pack.scope)}
            </div>
            <div className='flex flex-wrap gap-1'>
              {pack.permissions.map((primitive) => (
                <Badge key={primitive} variant='outline'>
                  {permissionLabel(t, primitive)}
                </Badge>
              ))}
            </div>
            {canAdopt && (
              <div className='mt-auto pt-1'>
                {adopted ? (
                  <Button variant='outline' size='sm' disabled>
                    <Check aria-hidden='true' />
                    {t('Adopted')}
                  </Button>
                ) : (
                  <Button
                    variant='outline'
                    size='sm'
                    disabled={adopting !== null}
                    onClick={() => void adopt(pack)}
                    aria-label={t('Adopt {{name}}', { name })}
                  >
                    <Plus aria-hidden='true' />
                    {adopting === pack.key ? t('Adopting...') : t('Adopt')}
                  </Button>
                )}
              </div>
            )}
          </section>
        )
      })}
    </div>
  )
}
