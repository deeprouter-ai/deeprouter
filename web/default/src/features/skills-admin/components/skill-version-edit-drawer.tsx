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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { updateVersion } from '../api'
import {
  getUpdateVersionFormSchema,
  type UpdateVersionFormValues,
} from '../lib/skill-form'
import type { SkillVersion } from '../types'

type SkillVersionEditDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  skillId: number
  version: SkillVersion | null
  onUpdated: () => void
}

export function SkillVersionEditDrawer({
  open,
  onOpenChange,
  skillId,
  version,
  onUpdated,
}: SkillVersionEditDrawerProps) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<UpdateVersionFormValues>({
    resolver: zodResolver(getUpdateVersionFormSchema(t)),
    defaultValues: {
      skill_md_content: '',
      manifest_json: '{}',
      changelog: '',
    },
  })

  // Re-seed the form every time a different version is opened — this drawer
  // is a single instance reused across rows, not remounted per-row.
  useEffect(() => {
    if (open && version) {
      form.reset({
        skill_md_content: version.skill_md_content,
        manifest_json: JSON.stringify(version.manifest_json, null, 2),
        changelog: version.changelog ?? '',
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, version])

  const onSubmit = async (values: UpdateVersionFormValues) => {
    if (!version) return
    setIsSubmitting(true)
    try {
      const result = await updateVersion(skillId, version.id, {
        skill_md_content: values.skill_md_content,
        manifest_json: JSON.parse(values.manifest_json),
        changelog: values.changelog ?? '',
      })
      if (result.success) {
        toast.success(t('Version updated'))
        onOpenChange(false)
        onUpdated()
      } else {
        toast.error(result.message ?? t('Update failed'))
      }
    } catch (_error) {
      // Errors are handled by the global interceptor (toast + reject)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-[700px]'>
        <SheetHeader className='border-b px-4 py-3 text-start sm:px-6 sm:py-4'>
          <SheetTitle>{t('Edit Version')}</SheetTitle>
          <SheetDescription>
            {t(
              'Only draft versions can be edited. The version number itself cannot change — upload a new version to bump it.'
            )}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='version-edit-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex-1 space-y-4 overflow-y-auto px-3 py-3 pb-4 sm:space-y-6 sm:px-4'
          >
            <FormItem>
              <FormLabel>{t('Version')}</FormLabel>
              <FormControl>
                <Input value={version?.version ?? ''} disabled />
              </FormControl>
            </FormItem>

            <FormField
              control={form.control}
              name='skill_md_content'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('SKILL.md')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={14}
                      className='font-mono text-xs'
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='manifest_json'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('manifest.json')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={8}
                      className='font-mono text-xs'
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'requires_deeprouter_key must stay true — activation is rejected otherwise.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='changelog'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Changelog')}</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={2} />
                  </FormControl>
                </FormItem>
              )}
            />
          </form>
        </Form>
        <SheetFooter className='grid grid-cols-2 gap-2 border-t px-4 py-3 sm:flex sm:px-6 sm:py-4'>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button
            form='version-edit-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
