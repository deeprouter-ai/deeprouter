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
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getSkillLogs } from '../api'
import type { SkillAdminLog } from '../types'

export function SkillActivityLog({ skillId }: { skillId: number }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [selectedLog, setSelectedLog] = useState<SkillAdminLog | null>(null)

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
          <div className='rounded-xl border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Time')}</TableHead>
                  <TableHead>{t('Admin')}</TableHead>
                  <TableHead>{t('Action')}</TableHead>
                  <TableHead className='text-right'>{t('Details')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => {
                  const hasDetails = Object.keys(log.details ?? {}).length > 0
                  return (
                    <TableRow key={log.id}>
                      <TableCell className='text-muted-foreground text-xs'>
                        {formatDateTimeStr(new Date(log.created_at))}
                      </TableCell>
                      <TableCell className='text-xs'>#{log.admin_id}</TableCell>
                      <TableCell className='text-xs font-medium'>
                        {log.action}
                      </TableCell>
                      <TableCell className='text-right'>
                        <Button
                          variant='ghost'
                          size='sm'
                          disabled={!hasDetails}
                          onClick={() => setSelectedLog(log)}
                        >
                          {t('View')}
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </CollapsibleContent>

      <Dialog
        open={selectedLog !== null}
        onOpenChange={(v) => !v && setSelectedLog(null)}
      >
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Log Details')}</DialogTitle>
            <DialogDescription className='sr-only'>
              {t('View the complete details for this log entry')}
            </DialogDescription>
          </DialogHeader>
          {selectedLog && (
            <pre className='bg-muted/50 max-h-96 overflow-auto rounded p-3 font-mono text-xs'>
              {JSON.stringify(selectedLog.details, null, 2)}
            </pre>
          )}
        </DialogContent>
      </Dialog>
    </Collapsible>
  )
}
