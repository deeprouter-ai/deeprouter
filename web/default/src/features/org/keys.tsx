// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import {
  deleteOrgKey,
  fetchOrgKeyHolders,
  fetchOrgKeyTemplates,
  fetchOrgKeys,
  freezeOrgKey,
  orgQueryKeys,
  rotateOrgKey,
  unfreezeOrgKey,
} from './api'
import { KeyDialog } from './components/key-dialog'
import { KeyReadyDialog } from './components/key-ready-dialog'
import { KeysTable } from './components/keys-table'
import { useOrgMembership } from './hooks/use-org-membership'
import { holds } from './lib/permissions'
import type { OrgKey, OrgKeyGrant } from './types'

/** Which dialog of the page is open, and on what. */
type OpenDialog =
  | { type: 'key'; key?: OrgKey }
  | { type: 'rotate'; key: OrgKey }
  | { type: 'delete'; key: OrgKey }

/**
 * "Organization keys" — the third page of the organization area (Enterprise
 * Org PRD §3, §6): the keys the company has made, who holds each, what each
 * may call and spend, and the ways to change, replace, freeze and delete them.
 *
 * No key's value is ever on this page. A person's key reaches its holder
 * through one-click setup on their own keys page; a service account's is shown
 * once, in a dialog, at the moment it is created or replaced (PRD D15).
 *
 * Anyone who may see keys opens it and is offered what their role allows; the
 * backend sends only the keys within their reach and checks every action again.
 */
export function OrgKeysPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialog, setDialog] = useState<OpenDialog | null>(null)
  const [ready, setReady] = useState<{
    grant: OrgKeyGrant
    rotated: boolean
  } | null>(null)
  const [busyKeyId, setBusyKeyId] = useState<number | null>(null)
  const close = () => setDialog(null)

  const membershipQuery = useOrgMembership()
  const membership = membershipQuery.data
  const creates = holds(membership, 'key.create')

  const keysQuery = useQuery({
    queryKey: orgQueryKeys.keys(),
    queryFn: fetchOrgKeys,
  })
  const templatesQuery = useQuery({
    queryKey: orgQueryKeys.keyTemplates(),
    queryFn: fetchOrgKeyTemplates,
    // The templates only change with a release.
    staleTime: 5 * 60 * 1000,
  })
  // Whom a key can be made out to is only asked by whoever may make one.
  const holdersQuery = useQuery({
    queryKey: orgQueryKeys.keyHolders(),
    queryFn: fetchOrgKeyHolders,
    enabled: creates,
  })

  const keys = keysQuery.data ?? []
  const queries = [membershipQuery, keysQuery, templatesQuery, holdersQuery]
  const failed = queries.some((query) => query.isError)
  const loading = !membership || queries.some((query) => query.isLoading)
  const refresh = () =>
    void queryClient.invalidateQueries({ queryKey: orgQueryKeys.keys() })

  // Runs one action on one key and says so when it worked.
  const act = async (
    key: OrgKey,
    action: () => Promise<{ success: boolean }>,
    done: string
  ) => {
    setBusyKeyId(key.id)
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
      setBusyKeyId(null)
    }
  }

  const rotate = async (key: OrgKey) => {
    setBusyKeyId(key.id)
    try {
      const res = await rotateOrgKey(key.id)
      if (res.success && res.data) {
        refresh()
        close()
        // A service account's new value has to be seen now or never; a
        // person's is not shown, so a line of confirmation is all there is.
        if (res.data.value) {
          setReady({ grant: res.data, rotated: true })
        } else {
          toast.success(t('The key has a new value'))
        }
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setBusyKeyId(null)
    }
  }

  const rotating = dialog?.type === 'rotate' ? dialog.key : null
  const deleting = dialog?.type === 'delete' ? dialog.key : null

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Organization keys')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {creates && (
            <Button
              size='sm'
              disabled={loading || failed}
              onClick={() => setDialog({ type: 'key' })}
            >
              <Plus aria-hidden='true' />
              {t('New key')}
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
            <div className='grid gap-3'>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'A key’s value never appears on this page. A member installs their key into their tools with one-click setup, from their own API keys page; a service account’s key is shown once, when it is created or given a new value.'
                )}
              </p>
              {/* Said out loud, so a short list reads as a scope and not as a
                  company with three keys in it. */}
              {membership?.role_scope === 'dept' && (
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'You see the keys held by members of the departments you manage.'
                  )}
                </p>
              )}
              {keys.length === 0 ? (
                <EmptyState
                  icon={KeyRound}
                  title={t('No keys yet')}
                  description={
                    creates
                      ? t(
                          'Create one for a member or a service account, with the models it may call and what it may spend.'
                        )
                      : t(
                          'Keys appear here once someone who may create them has made one.'
                        )
                  }
                  action={
                    creates ? (
                      <Button
                        variant='outline'
                        onClick={() => setDialog({ type: 'key' })}
                      >
                        {t('New key')}
                      </Button>
                    ) : undefined
                  }
                  bordered
                />
              ) : (
                <KeysTable
                  keys={keys}
                  busyKeyId={busyKeyId}
                  onEdit={
                    holds(membership, 'key.update')
                      ? (key) => setDialog({ type: 'key', key })
                      : undefined
                  }
                  onRotate={
                    holds(membership, 'key.rotate')
                      ? (key) => setDialog({ type: 'rotate', key })
                      : undefined
                  }
                  // Freezing is the first thing to do with a key that may have
                  // leaked: one click, no question asked. It can be undone.
                  onFreeze={
                    holds(membership, 'key.freeze')
                      ? (key) =>
                          void act(
                            key,
                            () => freezeOrgKey(key.id),
                            t('Key frozen')
                          )
                      : undefined
                  }
                  onUnfreeze={
                    holds(membership, 'key.freeze')
                      ? (key) =>
                          void act(
                            key,
                            () => unfreezeOrgKey(key.id),
                            t('Key unfrozen')
                          )
                      : undefined
                  }
                  onDelete={
                    holds(membership, 'key.delete')
                      ? (key) => setDialog({ type: 'delete', key })
                      : undefined
                  }
                />
              )}
            </div>
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* Outside the layout: it renders its named slots and nothing else. */}
      <KeyDialog
        open={dialog?.type === 'key'}
        editing={dialog?.type === 'key' ? dialog.key : undefined}
        holders={holdersQuery.data ?? []}
        templates={templatesQuery.data ?? []}
        onClose={close}
        onCreated={(grant) => {
          refresh()
          setReady({ grant, rotated: false })
        }}
        onUpdated={refresh}
      />
      <KeyReadyDialog
        grant={ready?.grant ?? null}
        rotated={ready?.rotated ?? false}
        onClose={() => setReady(null)}
      />
      <ConfirmDialog
        open={rotating !== null}
        onOpenChange={(open) => !open && close()}
        title={t('Give this key a new value?')}
        desc={
          rotating === null
            ? ''
            : rotating.holder_is_service
              ? t(
                  'The current value of “{{name}}” stops working at once. The new one is shown to you once — have the place that uses it ready to be updated.',
                  { name: rotating.name }
                )
              : t(
                  'The current value of “{{name}}” stops working at once. The new one is not shown: {{holder}} runs one-click setup again to go on using it.',
                  { name: rotating.name, holder: rotating.holder }
                )
        }
        destructive
        confirmText={t('Replace the value')}
        isLoading={rotating !== null && busyKeyId === rotating.id}
        handleConfirm={() => {
          if (rotating) void rotate(rotating)
        }}
      />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => !open && close()}
        title={t('Delete this key?')}
        desc={
          deleting === null
            ? ''
            : t(
                '“{{name}}” stops working at once and cannot be brought back. What it has spent stays in the usage records.',
                { name: deleting.name }
              )
        }
        destructive
        confirmText={t('Delete')}
        isLoading={deleting !== null && busyKeyId === deleting.id}
        handleConfirm={() => {
          if (deleting) {
            void act(
              deleting,
              () => deleteOrgKey(deleting.id),
              t('Key deleted')
            )
          }
        }}
      />
    </>
  )
}
