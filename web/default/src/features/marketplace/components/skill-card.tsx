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
import { Link } from '@tanstack/react-router'
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { skillPriceLabel } from '../lib/price'
import type { MarketplaceSkill } from '../types'

export function SkillCard({ skill }: { skill: MarketplaceSkill }) {
  const { t } = useTranslation()
  const free = skill.monetization_type === 'free'

  return (
    <Link
      to='/marketplace/$slug'
      params={{ slug: skill.slug }}
      className='group border-border bg-card hover:border-primary/50 flex h-full flex-col rounded-xl border p-4 transition-colors'
    >
      <div className='flex items-start justify-between gap-2'>
        <span className='font-medium'>{skill.name}</span>
        {skill.featured_flag && (
          <span className='text-accent flex shrink-0 items-center gap-1 text-xs font-medium'>
            <Sparkles className='size-3.5' />
            {t('Featured')}
          </span>
        )}
      </div>
      <p className='text-muted-foreground mt-1 line-clamp-2 flex-1 text-sm'>
        {skill.description}
      </p>
      <div className='mt-3 flex items-center gap-2'>
        <Badge variant='secondary'>{skill.category}</Badge>
        <Badge variant={free ? 'ghost' : 'outline'} className='tabular-nums'>
          {skillPriceLabel(skill, t('Free'))}
        </Badge>
      </div>
    </Link>
  )
}
