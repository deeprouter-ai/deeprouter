// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, Link2, Plus, UserPlus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
  revokeOrgInvite,
} from './api'
import { DepartmentDialog } from './components/department-dialog'
import { DepartmentsTable } from './components/departments-table'
import { InviteCreateDialog } from './components/invite-create-dialog'
import { InvitesTable } from './components/invites-table'
import { MemberEditDialog } from './components/member-edit-dialog'
import { MembersTable } from './components/members-table'
import { ServiceAccountDialog } from './components/service-account-dialog'
import { useOrgMembership } from './hooks/use-org-membership'
import type { OrgDepartment, OrgInvite, OrgMember } from './types'

/** Which dialog of the page is open, and on what. */
type OpenDialog =
  | { type: 'invite' }
  | { type: 'service-account' }
  | { type: 'department'; department?: OrgDepartment }
  | { type: 'delete-department'; department: OrgDepartment }
  | { type: 'member'; member: OrgMember }
  | { type: 'revoke-invite'; invite: OrgInvite }

/**
 * "Members & departments" — the first page of the organization area
 * (Enterprise Org PRD §6): who is in the company, in which department and
 * role, plus the invite links that bring new people in.
 */
export function OrgMembersPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialog, setDialog] = useState<OpenDialog | null>(null)
  const [working, setWorking] = useState(false)
  const close = () => setDialog(null)

  const membershipQuery = useOrgMembership()
  const membership = membershipQuery.data
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
  const invitesQuery = useQuery({
    queryKey: orgQueryKeys.invites(),
    queryFn: fetchOrgInvites,
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

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Members & departments')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            size='sm'
            disabled={loading || failed}
            onClick={() => setDialog({ type: 'service-account' })}
          >
            <Bot aria-hidden='true' />
            {t('Add service account')}
          </Button>
          <Button
            size='sm'
            disabled={loading || failed}
            onClick={() => setDialog({ type: 'invite' })}
          >
            <UserPlus aria-hidden='true' />
            {t('Invite members')}
          </Button>
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
                  <TabsTrigger value='invites'>
                    {t('Invite links')}
                    <span className='text-muted-foreground tabular-nums'>
                      {invites.length}
                    </span>
                  </TabsTrigger>
                </TabsList>
                <span className='text-muted-foreground truncate text-sm'>
                  {membership?.org_name}
                </span>
              </div>

              <TabsContent value='members' className='pt-2'>
                <MembersTable
                  members={members}
                  departments={departments}
                  onEdit={(member) => setDialog({ type: 'member', member })}
                />
              </TabsContent>

              <TabsContent value='departments' className='space-y-3 pt-2'>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Departments decide who belongs where and how reports are grouped. They carry no permissions — those come from roles.'
                    )}
                  </p>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => setDialog({ type: 'department' })}
                  >
                    <Plus aria-hidden='true' />
                    {t('New department')}
                  </Button>
                </div>
                <DepartmentsTable
                  departments={departments}
                  onRename={(department) =>
                    setDialog({ type: 'department', department })
                  }
                  onDelete={(department) =>
                    setDialog({ type: 'delete-department', department })
                  }
                />
              </TabsContent>

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
            </Tabs>
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* Outside the layout: it renders its named slots and nothing else. */}
      {membership && (
        <>
          <MemberEditDialog
            member={dialog?.type === 'member' ? dialog.member : null}
            roles={roles}
            departments={departments}
            actor={membership}
            onClose={close}
            onSaved={refresh}
          />
          <InviteCreateDialog
            open={dialog?.type === 'invite'}
            roles={roles}
            departments={departments}
            actor={membership}
            onClose={close}
            onCreated={refresh}
          />
        </>
      )}
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
          void confirm(() => deleteOrgDepartment(id), t('Department deleted'))
        }}
      />
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
    </>
  )
}
