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
import { useQuery } from '@tanstack/react-query'
import { ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatDateTimeStr } from '@/lib/format'
import { cn } from '@/lib/utils'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { getSkillLogs } from '../api'

export function SkillActivityLog({ skillId }: { skillId: number }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-skill-logs', skillId],
    queryFn: () => getSkillLogs(skillId),
    enabled: open,
  })

  const logs = data?.data ?? []

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger
        render={
          <button
            type='button'
            className='bg-card hover:bg-accent/50 flex w-full items-center justify-between rounded-xl border px-5 py-4 text-left transition-colors'
          />
        }
      >
        <div className='text-[13px] font-semibold'>{t('Activity Log')}</div>
        <ChevronDown
          className={cn(
            'text-muted-foreground h-4 w-4 shrink-0 transition-transform',
            open && 'rotate-180'
          )}
        />
      </CollapsibleTrigger>

      <CollapsibleContent className='mt-3'>
        {isLoading ? (
          <p className='text-muted-foreground px-1 text-sm'>
            {t('Loading...')}
          </p>
        ) : logs.length === 0 ? (
          <p className='text-muted-foreground px-1 text-sm'>
            {t('No activity yet')}
          </p>
        ) : (
          <ul className='divide-y rounded-xl border'>
            {logs.map((log) => (
              <li
                key={log.id}
                className='flex flex-col gap-1 px-4 py-3 text-sm'
              >
                <div className='flex items-center justify-between'>
                  <span className='font-medium'>{log.action}</span>
                  <span className='text-muted-foreground text-xs'>
                    {formatDateTimeStr(new Date(log.created_at))}
                  </span>
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('Admin')} #{log.admin_id}
                </div>
                {Object.keys(log.details ?? {}).length > 0 && (
                  <pre className='bg-muted/50 mt-1 overflow-x-auto rounded p-2 font-mono text-xs'>
                    {JSON.stringify(log.details, null, 2)}
                  </pre>
                )}
              </li>
            ))}
          </ul>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}
