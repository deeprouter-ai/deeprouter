// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, Link2, Plus, UserPlus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import {
  deleteOrgDepartment,
  fetchOrgDepartments,
  fetchOrgInvites,
  fetchOrgMembers,
  fetchOrgRoles,
  orgQueryKeys,
  removeOrgMember,
  revokeOrgInvite,
} from './api'
import { DepartmentDialog } from './components/department-dialog'
import { DepartmentsTable } from './components/departments-table'
import { InviteCreateDialog } from './components/invite-create-dialog'
import { InvitesTable } from './components/invites-table'
import { MemberEditDialog } from './components/member-edit-dialog'
import { MembersDirectory } from './components/members-directory'
import { ServiceAccountDialog } from './components/service-account-dialog'
import { canManageOrg, useOrgMembership } from './hooks/use-org-membership'
import { holds } from './lib/permissions'
import type { OrgDepartment, OrgInvite, OrgMember } from './types'

/** Which dialog of the page is open, and on what. */
type OpenDialog =
  | { type: 'invite' }
  | { type: 'service-account' }
  | { type: 'department'; department?: OrgDepartment }
  | { type: 'delete-department'; department: OrgDepartment }
  | { type: 'member'; member: OrgMember }
  | { type: 'remove-member'; member: OrgMember }
  | { type: 'revoke-invite'; invite: OrgInvite }

/**
 * "Members & departments" — the first page of the organization area
 * (Enterprise Org PRD §6): who is in the company, in which department and
 * role, plus the invite links that bring new people in.
 *
 * Anyone who may see members opens it, and it offers each of them what their
 * role allows: the owner and admins everything, a manager the people of their
 * departments and invite links into them, a role that may remove members the
 * button for that, a read-only member no button at all.
 * The backend sends only what the caller may see and checks every action
 * again.
 */
export function OrgMembersPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialog, setDialog] = useState<OpenDialog | null>(null)
  const [working, setWorking] = useState(false)
  const close = () => setDialog(null)

  const membershipQuery = useOrgMembership()
  const membership = membershipQuery.data
  // Roles, departments and service accounts are the owner's and the admins' to
  // manage; inviting takes a permission a role can carry.
  const runs = canManageOrg(membership)
  const invitesPeople = holds(membership, 'member.invite')
  const removesPeople = holds(membership, 'member.remove')
  const selfId = useAuthStore((s) => s.auth.user?.id)
  // Whom the remove button is offered on. Nobody removes the owner or
  // themselves, and dismissing an admin is the owner's alone; the backend
  // holds the same three lines.
  const removable = (member: OrgMember) =>
    !member.is_owner &&
    member.id !== selfId &&
    (member.role !== 'admin' || Boolean(membership?.is_owner))

  const membersQuery = useQuery({
    queryKey: orgQueryKeys.members(),
    queryFn: fetchOrgMembers,
  })
  const departmentsQuery = useQuery({
    queryKey: orgQueryKeys.departments(),
    queryFn: fetchOrgDepartments,
  })
  const rolesQuery = useQuery({
    queryKey: orgQueryKeys.roles(),
    queryFn: fetchOrgRoles,
  })
  // A link is a way into the organization, so the backend shows them only to
  // whoever may invite; asking without that would be a refusal on every load.
  const invitesQuery = useQuery({
    queryKey: orgQueryKeys.invites(),
    queryFn: fetchOrgInvites,
    enabled: invitesPeople,
  })

  const members = membersQuery.data ?? []
  const departments = departmentsQuery.data ?? []
  const roles = rolesQuery.data ?? []
  const invites = invitesQuery.data ?? []
  // Who is acting comes first: the dialogs need it to know which roles to
  // offer. A failure of any of the five is one failure of the page.
  const queries = [
    membershipQuery,
    membersQuery,
    departmentsQuery,
    rolesQuery,
    invitesQuery,
  ]
  const failed = queries.some((query) => query.isError)
  const loading = !membership || queries.some((query) => query.isLoading)

  // Members carry their department and departments their head count, so a
  // change to either list can show up in the other.
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: orgQueryKeys.members() })
    void queryClient.invalidateQueries({ queryKey: orgQueryKeys.departments() })
    void queryClient.invalidateQueries({ queryKey: orgQueryKeys.invites() })
  }

  // Takes a member out of the organization. Their keys go back under the
  // owner and they stop being someone a key can be made out to, so the keys
  // page has to ask again too.
  const remove = async (member: OrgMember) => {
    setWorking(true)
    try {
      const res = await removeOrgMember(member.id)
      if (res.success) {
        const reclaimed = res.data?.reclaimed_keys ?? 0
        toast.success(
          t('{{name}} is no longer in the organization', {
            name: member.display_name || member.username,
          }),
          reclaimed > 0
            ? {
                description: t('{{count}} key(s) were taken back.', {
                  count: reclaimed,
                }),
              }
            : undefined
        )
        refresh()
        void queryClient.invalidateQueries({ queryKey: orgQueryKeys.keys() })
        void queryClient.invalidateQueries({
          queryKey: orgQueryKeys.keyHolders(),
        })
        void queryClient.invalidateQueries({
          queryKey: orgQueryKeys.keyAssignees(),
        })
        close()
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setWorking(false)
    }
  }

  // Runs a confirmed, destructive action and closes its dialog on success.
  const confirm = async (
    action: () => Promise<{ success: boolean }>,
    done: string
  ) => {
    setWorking(true)
    try {
      const res = await action()
      if (res.success) {
        toast.success(done)
        refresh()
        close()
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setWorking(false)
    }
  }

  const defaultDepartment = departments.find((d) => d.is_default)
  const removing = dialog?.type === 'remove-member' ? dialog.member : null

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Members & departments')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {runs && (
            <Button
              variant='outline'
              size='sm'
              disabled={loading || failed}
              onClick={() => setDialog({ type: 'service-account' })}
            >
              <Bot aria-hidden='true' />
              {t('Add service account')}
            </Button>
          )}
          {invitesPeople && (
            <Button
              size='sm'
              disabled={loading || failed}
              onClick={() => setDialog({ type: 'invite' })}
            >
              <UserPlus aria-hidden='true' />
              {t('Invite members')}
            </Button>
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {failed ? (
            <ErrorState
              description={t('Could not load your organization.')}
              onRetry={() => queries.forEach((query) => void query.refetch())}
            />
          ) : loading ? (
            <div className='space-y-3'>
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className='h-12 rounded-xl' />
              ))}
            </div>
          ) : (
            <Tabs defaultValue='members'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <TabsList>
                  <TabsTrigger value='members'>
                    {t('Members')}
                    <span className='text-muted-foreground tabular-nums'>
                      {members.length}
                    </span>
                  </TabsTrigger>
                  <TabsTrigger value='departments'>
                    {t('Departments')}
                    <span className='text-muted-foreground tabular-nums'>
                      {departments.length}
                    </span>
                  </TabsTrigger>
                  {invitesPeople && (
                    <TabsTrigger value='invites'>
                      {t('Invite links')}
                      <span className='text-muted-foreground tabular-nums'>
                        {invites.length}
                      </span>
                    </TabsTrigger>
                  )}
                </TabsList>
                <span className='text-muted-foreground truncate text-sm'>
                  {membership?.org_name}
                </span>
              </div>

              <TabsContent value='members' className='space-y-3 pt-2'>
                {/* Said out loud, so a short list reads as a scope and not as
                    a company with three people in it. */}
                {membership?.role_scope === 'dept' && (
                  <p className='text-muted-foreground text-sm'>
                    {t('You see the members of the departments you manage.')}
                  </p>
                )}
                <MembersDirectory
                  members={members}
                  departments={departments}
                  onEdit={
                    runs
                      ? (member) => setDialog({ type: 'member', member })
                      : undefined
                  }
                  onRemove={
                    removesPeople
                      ? (member) => setDialog({ type: 'remove-member', member })
                      : undefined
                  }
                  removable={removable}
                />
              </TabsContent>

              <TabsContent value='departments' className='space-y-3 pt-2'>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Departments decide who belongs where and how reports are grouped. They carry no permissions — those come from roles.'
                    )}
                  </p>
                  {runs && (
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => setDialog({ type: 'department' })}
                    >
                      <Plus aria-hidden='true' />
                      {t('New department')}
                    </Button>
                  )}
                </div>
                <DepartmentsTable
                  departments={departments}
                  onRename={
                    runs
                      ? (department) =>
                          setDialog({ type: 'department', department })
                      : undefined
                  }
                  onDelete={
                    runs
                      ? (department) =>
                          setDialog({ type: 'delete-department', department })
                      : undefined
                  }
                />
              </TabsContent>

              {invitesPeople && (
                <TabsContent value='invites' className='pt-2'>
                  {invites.length === 0 ? (
                    <EmptyState
                      icon={Link2}
                      title={t('No invite links yet')}
                      description={t(
                        'Create a link for a role and a department, and send it to the people who should join.'
                      )}
                      action={
                        <Button
                          variant='outline'
                          onClick={() => setDialog({ type: 'invite' })}
                        >
                          {t('Invite members')}
                        </Button>
                      }
                      bordered
                    />
                  ) : (
                    <InvitesTable
                      invites={invites}
                      departments={departments}
                      onRevoke={(invite) =>
                        setDialog({ type: 'revoke-invite', invite })
                      }
                    />
                  )}
                </TabsContent>
              )}
            </Tabs>
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* Outside the layout: it renders its named slots and nothing else. */}
      {membership && runs && (
        <MemberEditDialog
          member={dialog?.type === 'member' ? dialog.member : null}
          roles={roles}
          departments={departments}
          actor={membership}
          onClose={close}
          onSaved={refresh}
        />
      )}
      {membership && invitesPeople && (
        <InviteCreateDialog
          open={dialog?.type === 'invite'}
          roles={roles}
          departments={departments}
          actor={membership}
          onClose={close}
          onCreated={refresh}
        />
      )}
      {runs && (
        <>
          <ServiceAccountDialog
            open={dialog?.type === 'service-account'}
            departments={departments}
            onClose={close}
            onCreated={refresh}
          />
          <DepartmentDialog
            open={dialog?.type === 'department'}
            department={
              dialog?.type === 'department' ? dialog.department : undefined
            }
            onClose={close}
            onSaved={refresh}
          />
          <ConfirmDialog
            open={dialog?.type === 'delete-department'}
            onOpenChange={(open) => !open && close()}
            title={t('Delete this department?')}
            desc={
              dialog?.type === 'delete-department'
                ? t(
                    '“{{name}}” will be deleted. Its members and unused invite links move to “{{fallback}}”.',
                    {
                      name: dialog.department.name,
                      fallback: defaultDepartment?.name ?? '',
                    }
                  )
                : ''
            }
            destructive
            confirmText={t('Delete')}
            isLoading={working}
            handleConfirm={() => {
              if (dialog?.type !== 'delete-department') return
              const { id } = dialog.department
              void confirm(
                () => deleteOrgDepartment(id),
                t('Department deleted')
              )
            }}
          />
        </>
      )}
      {removesPeople && (
        <ConfirmDialog
          open={removing !== null}
          onOpenChange={(open) => !open && close()}
          title={
            removing === null
              ? ''
              : removing.is_service
                ? t('Delete the service account {{name}}?', {
                    name: removing.display_name || removing.username,
                  })
                : t('Remove {{name}} from the organization?', {
                    name: removing.display_name || removing.username,
                  })
          }
          desc={
            removing === null ? (
              ''
            ) : (
              <div className='grid gap-2'>
                <p>
                  {removing.is_service
                    ? t('The service account is deleted.')
                    : t(
                        'Their account is deleted: they can no longer sign in, and nobody can sign up with the same username or email again.'
                      )}
                </p>
                <p>
                  {removing.key_count > 0
                    ? t(
                        'The {{count}} key(s) held under this account are taken back: they stop working at once, get a new value and go back under the owner, frozen.',
                        { count: removing.key_count }
                      )
                    : t('This account holds no keys.')}
                </p>
                <p>
                  {t(
                    'What it has spent stays in the usage records. This cannot be undone.'
                  )}
                </p>
              </div>
            )
          }
          destructive
          confirmText={
            removing?.is_service ? t('Delete') : t('Remove from organization')
          }
          isLoading={working}
          handleConfirm={() => {
            if (removing) void remove(removing)
          }}
        />
      )}
      {invitesPeople && (
        <ConfirmDialog
          open={dialog?.type === 'revoke-invite'}
          onOpenChange={(open) => !open && close()}
          title={t('Revoke this invite link?')}
          desc={t(
            'The link stops working at once. People who already joined through it stay in the organization.'
          )}
          destructive
          confirmText={t('Revoke')}
          isLoading={working}
          handleConfirm={() => {
            if (dialog?.type !== 'revoke-invite') return
            const { id } = dialog.invite
            void confirm(() => revokeOrgInvite(id), t('Invite link revoked'))
          }}
        />
      )}
    </>
  )
}
