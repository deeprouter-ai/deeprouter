/*
Copyright (C) 2026 DeepRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { downloadMarketplaceSkill, marketplaceQueryKeys } from '../api'
import { DownloadSkillError } from '../download-utils'

/**
 * Shared download action for the detail page and My Skills. Maps the three
 * backend outcomes to user-facing copy; a successful (possibly paying)
 * download invalidates every marketplace query so Downloaded badges, the
 * purchase list and the sidebar entry all refresh.
 */
export function useSkillDownload() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [pendingSlug, setPendingSlug] = useState<string | null>(null)

  const download = async (slug: string): Promise<boolean> => {
    setPendingSlug(slug)
    try {
      await downloadMarketplaceSkill(slug)
      toast.success(t('Download started. Unzip it into .claude/skills/ to use it.'))
      queryClient.invalidateQueries({ queryKey: marketplaceQueryKeys.all })
      return true
    } catch (error) {
      if (error instanceof DownloadSkillError) {
        if (error.code === 'INSUFFICIENT_BALANCE') {
          // PRD §8.2: “余额不足，前往 Wallet 充值”.
          toast.error(t('Insufficient balance — top up in Wallet to continue.'), {
            action: {
              label: t('Go to Wallet'),
              onClick: () => navigate({ to: '/wallet' }),
            },
          })
        } else if (error.code === 'NOT_FOUND') {
          toast.error(t('This skill is no longer available for download.'))
          queryClient.invalidateQueries({ queryKey: marketplaceQueryKeys.all })
        } else {
          toast.error(error.message || t('Download failed, please try again.'))
        }
      } else {
        toast.error(t('Download failed, please try again.'))
      }
      return false
    } finally {
      setPendingSlug(null)
    }
  }

  return { download, pendingSlug }
}
