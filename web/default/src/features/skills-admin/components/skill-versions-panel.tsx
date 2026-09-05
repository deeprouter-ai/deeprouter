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
import {
  Plus,
  Rocket,
  Edit,
  Trash2,
  MoreHorizontal as DotsHorizontalIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatDateTimeStr } from '@/lib/format'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StatusBadge } from '@/components/status-badge'
import { activateVersion } from '../api'
import { SKILL_VERSION_STATUS_VARIANTS } from '../constants'
import type { SkillSummary, SkillVersion } from '../types'
import { SkillVersionDeleteDialog } from './skill-version-delete-dialog'
import { SkillVersionEditDrawer } from './skill-version-edit-drawer'
import { SkillVersionUploadDrawer } from './skill-version-upload-drawer'

export function SkillVersionsPanel({
  skill,
  versions,
  onChanged,
}: {
  skill: SkillSummary
  versions: SkillVersion[]
  onChanged: () => void
}) {
  const { t } = useTranslation()
  const [uploadOpen, setUploadOpen] = useState(false)
  const [activatingId, setActivatingId] = useState<number | null>(null)
  const [activationError, setActivationError] = useState<string | null>(null)
  const [editingVersion, setEditingVersion] = useState<SkillVersion | null>(
    null
  )
  const [deletingVersion, setDeletingVersion] = useState<SkillVersion | null>(
    null
  )

  const handleActivate = async (versionId: number) => {
    setActivatingId(versionId)
    setActivationError(null)
    try {
      const result = await activateVersion(skill.id, versionId)
      if (result.success) {
        toast.success(t('Version activated'))
        onChanged()
      } else {
        // PRD §9: activation failure must show the security-guard error
        // detail to the Admin, not just a generic toast — they need it to
        // fix the package. A toast disappears; this stays until dismissed.
        setActivationError(result.message ?? t('Activation failed'))
      }
    } catch (error) {
      // The security-guard rejections this Alert exists for (PRD §9) come
      // back as a real HTTP 400, not a `{success:false}` body — so they
      // land here, not in the `else` above. Read the message straight off
      // the axios error the same way the global interceptor does.
      const message =
        (error as { response?: { data?: { message?: string } } })?.response
          ?.data?.message ?? t('Activation failed')
      setActivationError(message)
    } finally {
      setActivatingId(null)
    }
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between'>
        <CardTitle>{t('Versions')}</CardTitle>
        <Button size='sm' onClick={() => setUploadOpen(true)}>
          <Plus className='h-4 w-4' />
          {t('Upload New Version')}
        </Button>
      </CardHeader>
      <CardContent className='space-y-4'>
        {activationError && (
          <Alert variant='destructive'>
            <AlertTitle>{t('Activation failed')}</AlertTitle>
            <AlertDescription className='font-mono text-xs break-words'>
              {activationError}
            </AlertDescription>
          </Alert>
        )}

        {versions.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('No versions yet — upload one to get started.')}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Version')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Created')}</TableHead>
                <TableHead>{t('Changelog')}</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {versions.map((version) => (
                <TableRow key={version.id}>
                  <TableCell className='font-mono text-sm'>
                    {version.version}
                  </TableCell>
                  <TableCell>
                    <StatusBadge
                      label={version.status}
                      variant={SKILL_VERSION_STATUS_VARIANTS[version.status]}
                      showDot
                      copyable={false}
                    />
                  </TableCell>
                  <TableCell className='text-muted-foreground text-sm'>
                    {formatDateTimeStr(new Date(version.created_at))}
                  </TableCell>
                  <TableCell className='max-w-[200px] truncate text-sm'>
                    {version.changelog || '—'}
                  </TableCell>
                  <TableCell className='text-right'>
                    <div className='flex items-center justify-end gap-1'>
                      {version.status !== 'active' && (
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={activatingId === version.id}
                          onClick={() => handleActivate(version.id)}
                        >
                          <Rocket className='h-3.5 w-3.5' />
                          {activatingId === version.id
                            ? t('Activating...')
                            : version.status === 'archived'
                              ? t('Reactivate')
                              : t('Activate')}
                        </Button>
                      )}
                      {/* Only draft versions can be edited/deleted — active
                          and archived both return 409 on the backend. */}
                      {version.status === 'draft' && (
                        <DropdownMenu modal={false}>
                          <DropdownMenuTrigger
                            render={
                              <Button
                                variant='ghost'
                                size='sm'
                                className='h-8 w-8 p-0'
                              />
                            }
                          >
                            <DotsHorizontalIcon className='h-4 w-4' />
                            <span className='sr-only'>{t('Open menu')}</span>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align='end'>
                            <DropdownMenuItem
                              onClick={() => setEditingVersion(version)}
                            >
                              <Edit className='h-4 w-4' />
                              {t('Edit')}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => setDeletingVersion(version)}
                              className='text-destructive focus:text-destructive'
                            >
                              <Trash2 className='h-4 w-4' />
                              {t('Delete')}
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>

      <SkillVersionUploadDrawer
        open={uploadOpen}
        onOpenChange={setUploadOpen}
        skillId={skill.id}
        skillSlug={skill.slug}
        onUploaded={onChanged}
      />

      <SkillVersionEditDrawer
        open={editingVersion !== null}
        onOpenChange={(v) => !v && setEditingVersion(null)}
        skillId={skill.id}
        version={editingVersion}
        onUpdated={onChanged}
      />

      <SkillVersionDeleteDialog
        open={deletingVersion !== null}
        onOpenChange={(v) => !v && setDeletingVersion(null)}
        skillId={skill.id}
        version={deletingVersion}
        onDeleted={onChanged}
      />
    </Card>
  )
}
