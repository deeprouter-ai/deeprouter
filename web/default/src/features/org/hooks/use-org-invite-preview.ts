// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useQuery } from '@tanstack/react-query'
import { fetchOrgInvitePreview, orgQueryKeys } from '../api'
import type { OrgInvitePreview } from '../types'

export type OrgInvitePreviewState =
  | { status: 'none' } // no invite code on the page
  | { status: 'loading' }
  | { status: 'valid'; preview: OrgInvitePreview }
  | { status: 'invalid' } // unknown, expired or revoked

/** Looks up where an invite code leads, for the sign-up page. */
export function useOrgInvitePreview(
  code: string | undefined
): OrgInvitePreviewState {
  const { data, isPending, isError } = useQuery({
    queryKey: orgQueryKeys.invitePreview(code ?? ''),
    queryFn: () => fetchOrgInvitePreview(code ?? ''),
    enabled: Boolean(code),
    retry: false,
    staleTime: 60 * 1000,
  })
  if (!code) return { status: 'none' }
  if (isError) return { status: 'invalid' }
  if (isPending) return { status: 'loading' }
  return data ? { status: 'valid', preview: data } : { status: 'invalid' }
}
