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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { getSkill, listVersions } from '../api'
import { SKILL_STATUS_VARIANTS } from '../constants'
import { SkillActivityLog } from './skill-activity-log'
import { SkillMetadataForm } from './skill-metadata-form'
import { SkillPublishActions } from './skill-publish-actions'
import { SkillVersionsPanel } from './skill-versions-panel'

export function SkillEditPage({ skillId }: { skillId: number }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const skillQuery = useQuery({
    queryKey: ['admin-skill', skillId],
    queryFn: () => getSkill(skillId),
  })
  const versionsQuery = useQuery({
    queryKey: ['admin-skill-versions', skillId],
    queryFn: () => listVersions(skillId),
  })

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-skill', skillId] })
    queryClient.invalidateQueries({
      queryKey: ['admin-skill-versions', skillId],
    })
  }

  if (skillQuery.isLoading) {
    return <div className='text-muted-foreground p-6'>{t('Loading...')}</div>
  }

  const skill = skillQuery.data?.data
  if (!skill) {
    return (
      <div className='text-muted-foreground p-6'>{t('Skill not found')}</div>
    )
  }

  const versions = versionsQuery.data?.data ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <div className='flex items-center gap-2'>
          {skill.name}
          <StatusBadge
            label={skill.status}
            variant={SKILL_STATUS_VARIANTS[skill.status]}
            showDot
            copyable={false}
          />
        </div>
      </SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Edit skill metadata, manage versions, and publish.')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <SkillPublishActions skill={skill} onChanged={refresh} />
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-6'>
          <SkillMetadataForm skill={skill} onSaved={refresh} />
          <SkillVersionsPanel
            skill={skill}
            versions={versions}
            onChanged={refresh}
          />
          <SkillActivityLog skillId={skillId} />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
