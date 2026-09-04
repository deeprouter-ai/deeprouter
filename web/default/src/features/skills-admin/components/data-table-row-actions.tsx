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
import { useNavigate } from '@tanstack/react-router'
import { type Row } from '@tanstack/react-table'
import {
  Trash2,
  Edit,
  Rocket,
  ArchiveX,
  MoreHorizontal as DotsHorizontalIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { deprecateSkill, publishSkill } from '../api'
import type { SkillSummary } from '../types'
import { useSkills } from './skills-provider'

interface DataTableRowActionsProps<TData> {
  row: Row<TData>
}

export function DataTableRowActions<TData>({
  row,
}: DataTableRowActionsProps<TData>) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const skill = row.original as SkillSummary
  const { setOpen, setCurrentRow, triggerRefresh } = useSkills()

  const canPublish =
    (skill.status === 'draft' || skill.status === 'deprecated') &&
    !!skill.active_version_id
  const canDeprecate = skill.status === 'published'
  const canDelete = skill.status === 'draft'

  const handlePublish = async () => {
    const result = await publishSkill(skill.id)
    if (result.success) {
      toast.success(
        skill.status === 'deprecated'
          ? t('Skill republished')
          : t('Skill published')
      )
      triggerRefresh()
    }
  }

  const handleDeprecate = async () => {
    const result = await deprecateSkill(skill.id)
    if (result.success) {
      toast.success(t('Skill deprecated'))
      triggerRefresh()
    }
  }

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        render={
          <Button
            variant='ghost'
            className='data-popup-open:bg-muted flex h-8 w-8 p-0'
          />
        }
      >
        <DotsHorizontalIcon className='h-4 w-4' />
        <span className='sr-only'>{t('Open menu')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[180px]'>
        <DropdownMenuItem
          onClick={() =>
            navigate({
              to: '/admin/skills/$id/edit',
              params: { id: String(skill.id) },
            })
          }
        >
          {t('Edit')}
          <DropdownMenuShortcut>
            <Edit size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        {(skill.status === 'draft' || skill.status === 'deprecated') && (
          <Tooltip>
            <TooltipTrigger
              render={
                <DropdownMenuItem
                  onClick={handlePublish}
                  disabled={!canPublish}
                >
                  {skill.status === 'deprecated'
                    ? t('Republish')
                    : t('Publish')}
                  <DropdownMenuShortcut>
                    <Rocket size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
              }
            />
            {!canPublish && (
              <TooltipContent>
                {t('Set an active version before publishing')}
              </TooltipContent>
            )}
          </Tooltip>
        )}
        {canDeprecate && (
          <DropdownMenuItem onClick={handleDeprecate}>
            {t('Deprecate')}
            <DropdownMenuShortcut>
              <ArchiveX size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(skill)
            setOpen('delete')
          }}
          disabled={!canDelete}
          className='text-destructive focus:text-destructive'
        >
          {t('Delete')}
          <DropdownMenuShortcut>
            <Trash2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
