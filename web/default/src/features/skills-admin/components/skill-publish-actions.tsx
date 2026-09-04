/*
Copyright (C) 2023-2026 QuantumNous

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

For commercial licensing, please contact support@quantumnous.com
*/
import { ArchiveX, Rocket } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { deprecateSkill, publishSkill } from '../api'
import type { SkillSummary } from '../types'

export function SkillPublishActions({
  skill,
  onChanged,
}: {
  skill: SkillSummary
  onChanged: () => void
}) {
  const { t } = useTranslation()

  const canPublish =
    (skill.status === 'draft' || skill.status === 'deprecated') &&
    !!skill.active_version_id

  const handlePublish = async () => {
    const result = await publishSkill(skill.id)
    if (result.success) {
      toast.success(
        skill.status === 'deprecated'
          ? t('Skill republished')
          : t('Skill published')
      )
      onChanged()
    } else {
      toast.error(result.message ?? t('Publish failed'))
    }
  }

  const handleDeprecate = async () => {
    const result = await deprecateSkill(skill.id)
    if (result.success) {
      toast.success(t('Skill deprecated'))
      onChanged()
    }
  }

  if (skill.status === 'published') {
    return (
      <Button variant='outline' onClick={handleDeprecate}>
        <ArchiveX className='h-4 w-4' />
        {t('Deprecate')}
      </Button>
    )
  }

  const publishButton = (
    <Button onClick={handlePublish} disabled={!canPublish}>
      <Rocket className='h-4 w-4' />
      {skill.status === 'deprecated' ? t('Republish') : t('Publish')}
    </Button>
  )

  if (canPublish) return publishButton

  return (
    <Tooltip>
      <TooltipTrigger render={<div />}>{publishButton}</TooltipTrigger>
      <TooltipContent>
        {t('Upload and activate a version before publishing')}
      </TooltipContent>
    </Tooltip>
  )
}
