// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import {
  deleteOrgRole,
  fetchOrgMembers,
  fetchOrgPermissions,
  fetchOrgRoles,
  orgQueryKeys,
} from './api'
import { CustomRolesTable } from './components/custom-roles-table'
import { RoleDialog } from './components/role-dialog'
import { RoleMatrix } from './components/role-matrix'
import { RolePacks } from './components/role-packs'
import { canManageOrg, useOrgMembership } from './hooks/use-org-membership'
import type { OrgRole } from './types'

/** Which dialog of the page is open, and on what. */
type OpenDialog =
  | { type: 'role'; role?: OrgRole }
  | { type: 'delete-role'; role: OrgRole }

/**
 * "Roles & permissions" — the second page of the organization area
 * (Enterprise Org PRD §2, §6): the organization's own roles, the role packs
 * that are adopted into them and, last because they are fixed and only there
 * to be read, what each of the five preset roles may do.
 *
 * Anyone who may see members may read it; building, changing and deleting
 * roles is the owner's and the admins'. The backend checks every action again.
 */
export function OrgRolesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialog, setDialog] = useState<OpenDialog | null>(null)
  const [working, setWorking] = useState(false)
  const close = () => setDialog(null)

  const membershipQuery = useOrgMembership()
  const membership = membershipQuery.data
  const runs = canManageOrg(membership)
  // A head count per role is only true for a viewer who sees every member.
  const seesEveryone = membership?.role_scope === 'org'

  const rolesQuery = useQuery({
    queryKey: orgQueryKeys.roles(),
    queryFn: fetchOrgRoles,
  })
  const catalogQuery = useQuery({
    queryKey: orgQueryKeys.permissions(),
    queryFn: fetchOrgPermissions,
    // The vocabulary only changes with a release.
    staleTime: 5 * 60 * 1000,
  })
  const membersQuery = useQuery({
    queryKey: orgQueryKeys.members(),
    queryFn: fetchOrgMembers,
    enabled: seesEveryone,
  })

  const roles = rolesQuery.data ?? []
  const presets = roles.filter((role) => role.is_preset)
  const custom = roles.filter((role) => !role.is_preset)
  const catalog = catalogQuery.data
  const holders = new Map<number, number>()
  for (const member of membersQuery.data ?? []) {
    holders.set(member.role_id, (holders.get(member.role_id) ?? 0) + 1)
  }

  const queries = [membershipQuery, rolesQuery, catalogQuery, membersQuery]
  const failed = queries.some((query) => query.isError)
  const loading =
    !membership || !catalog || queries.some((query) => query.isLoading)

  // A role's name shows on the members page too, and deleting one changes who
  // holds what.
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: orgQueryKeys.roles() })
    void queryClient.invalidateQueries({ queryKey: orgQueryKeys.members() })
    void queryClient.invalidateQueries({ queryKey: orgQueryKeys.invites() })
  }

  const remove = async (role: OrgRole) => {
    setWorking(true)
    try {
      const res = await deleteOrgRole(role.id)
      if (res.success) {
        toast.success(t('Role deleted'))
        refresh()
        close()
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setWorking(false)
    }
  }

  const deleting = dialog?.type === 'delete-role' ? dialog.role : null
  const deletingHolders = deleting ? (holders.get(deleting.id) ?? 0) : 0

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Roles & permissions')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {runs && (
            <Button
              size='sm'
              disabled={loading || failed}
              onClick={() => setDialog({ type: 'role' })}
            >
              <Plus aria-hidden='true' />
              {t('New role')}
            </Button>
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {failed ? (
            <ErrorState
              description={t('Could not load your organization.')}
              onRetry={() => queries.forEach((query) => void query.refetch())}
            />
          ) : loading || !catalog ? (
            <div className='space-y-3'>
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className='h-12 rounded-xl' />
              ))}
            </div>
          ) : (
            <div className='grid gap-8'>
              <section
                aria-labelledby='org-custom-roles'
                className='grid gap-3'
              >
                <div className='grid gap-1'>
                  <h2 id='org-custom-roles' className='text-base font-semibold'>
                    {t('Custom roles')}
                  </h2>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Your own roles, built from the list of permissions. Running the organization itself cannot be put into one.'
                    )}
                  </p>
                </div>
                {custom.length === 0 ? (
                  <EmptyState
                    icon={ShieldCheck}
                    title={t('No custom roles yet')}
                    description={t(
                      'Build one from the permissions, or start from a role pack below.'
                    )}
                    action={
                      runs ? (
                        <Button
                          variant='outline'
                          onClick={() => setDialog({ type: 'role' })}
                        >
                          {t('New role')}
                        </Button>
                      ) : undefined
                    }
                    className='min-h-[200px]'
                    bordered
                  />
                ) : (
                  <CustomRolesTable
                    roles={custom}
                    holders={seesEveryone ? holders : undefined}
                    onEdit={
                      runs
                        ? (role) => setDialog({ type: 'role', role })
                        : undefined
                    }
                    onDelete={
                      runs
                        ? (role) => setDialog({ type: 'delete-role', role })
                        : undefined
                    }
                  />
                )}
              </section>

              <section aria-labelledby='org-role-packs' className='grid gap-3'>
                <div className='grid gap-1'>
                  <h2 id='org-role-packs' className='text-base font-semibold'>
                    {t('Role packs')}
                  </h2>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Ready-made roles for common jobs. Adopt one and it becomes a custom role of yours, to change as you like.'
                    )}
                  </p>
                </div>
                <RolePacks
                  packs={catalog.role_packs}
                  roles={custom}
                  canAdopt={runs}
                  onAdopted={refresh}
                />
              </section>

              <section
                aria-labelledby='org-preset-roles'
                className='grid gap-3'
              >
                <div className='grid gap-1'>
                  <h2 id='org-preset-roles' className='text-base font-semibold'>
                    {t('Preset roles')}
                  </h2>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Every organization has these five. What they may do is fixed. Whatever their role, a member can always use the keys assigned to them and see their own usage.'
                    )}
                  </p>
                </div>
                <RoleMatrix roles={presets} catalog={catalog} />
              </section>
            </div>
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* Outside the layout: it renders its named slots and nothing else. */}
      {runs && catalog && (
        <>
          <RoleDialog
            open={dialog?.type === 'role'}
            role={dialog?.type === 'role' ? dialog.role : undefined}
            catalog={catalog}
            onClose={close}
            onSaved={refresh}
          />
          <ConfirmDialog
            open={deleting !== null}
            onOpenChange={(open) => !open && close()}
            title={t('Delete this role?')}
            desc={
              deleting === null
                ? ''
                : deletingHolders > 0
                  ? t(
                      '“{{name}}” will be deleted. The members who hold it ({{holders}}) become Staff, and its unused invite links will bring in Staff.',
                      { name: deleting.name, holders: deletingHolders }
                    )
                  : t(
                      '“{{name}}” will be deleted. Nobody holds it; its unused invite links will bring in Staff.',
                      { name: deleting.name }
                    )
            }
            destructive
            confirmText={t('Delete')}
            isLoading={working}
            handleConfirm={() => {
              if (deleting) void remove(deleting)
            }}
          />
        </>
      )}
    </>
  )
}
