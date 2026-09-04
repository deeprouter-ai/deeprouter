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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { updateFeatured } from '../api'
import type { SkillSummary } from '../types'
import { useSkills } from './skills-provider'

export function FeaturedCell({ skill }: { skill: SkillSummary }) {
  const { t } = useTranslation()
  const { triggerRefresh } = useSkills()
  const [isSaving, setIsSaving] = useState(false)
  const canFeature = skill.status === 'published'

  const handleToggle = async (checked: boolean) => {
    setIsSaving(true)
    try {
      const result = await updateFeatured(skill.id, {
        featured_flag: checked,
        featured_rank: skill.featured_rank,
      })
      if (result.success) {
        triggerRefresh()
      }
    } finally {
      setIsSaving(false)
    }
  }

  const handleRankBlur = async (value: string) => {
    const rank = Number.parseInt(value, 10)
    if (Number.isNaN(rank) || rank === skill.featured_rank) return
    setIsSaving(true)
    try {
      const result = await updateFeatured(skill.id, {
        featured_flag: skill.featured_flag,
        featured_rank: rank,
      })
      if (result.success) {
        toast.success(t('Featured rank updated'))
        triggerRefresh()
      }
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className='flex items-center gap-2'>
      <Switch
        checked={skill.featured_flag}
        onCheckedChange={handleToggle}
        disabled={isSaving || !canFeature}
        aria-label={t('Featured')}
      />
      <Input
        type='number'
        defaultValue={skill.featured_rank}
        onBlur={(e) => handleRankBlur(e.target.value)}
        disabled={isSaving || !canFeature || !skill.featured_flag}
        className='h-8 w-16'
        aria-label={t('Featured rank')}
      />
    </div>
  )
}
