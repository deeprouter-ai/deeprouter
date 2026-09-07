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
import { uploadVersion } from '../api'
import {
  getUploadVersionFormSchema,
  type UploadVersionFormValues,
} from '../lib/skill-form'
import { manifestTemplate, SKILL_MD_TEMPLATE } from '../lib/skill-md-template'

type SkillVersionUploadDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  skillId: number
  skillSlug: string
  onUploaded: () => void
}

export function SkillVersionUploadDrawer({
  open,
  onOpenChange,
  skillId,
  skillSlug,
  onUploaded,
}: SkillVersionUploadDrawerProps) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const defaultValues: UploadVersionFormValues = {
    version: '',
    skill_md_content: SKILL_MD_TEMPLATE,
    manifest_json: manifestTemplate(skillSlug, ''),
    changelog: '',
  }

  const form = useForm<UploadVersionFormValues>({
    resolver: zodResolver(getUploadVersionFormSchema(t)),
    defaultValues,
  })

  useEffect(() => {
    if (open) form.reset(defaultValues)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  // Keep the manifest template's version field in sync as the admin types
  // the version number, as long as they haven't hand-edited the JSON yet.
  const version = form.watch('version')
  useEffect(() => {
    if (!form.formState.dirtyFields.manifest_json) {
      form.setValue('manifest_json', manifestTemplate(skillSlug, version))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version, skillSlug])

  const onSubmit = async (values: UploadVersionFormValues) => {
    setIsSubmitting(true)
    try {
      const result = await uploadVersion(skillId, {
        version: values.version,
        skill_md_content: values.skill_md_content,
        manifest_json: JSON.parse(values.manifest_json),
        changelog: values.changelog ?? '',
      })
      if (result.success) {
        toast.success(t('Version uploaded as draft'))
        onOpenChange(false)
        onUploaded()
      } else {
        toast.error(result.message ?? t('Upload failed'))
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
          <SheetTitle>{t('Upload New Version')}</SheetTitle>
          <SheetDescription>
            {t(
              'Creates a draft version. It stays draft — and un-downloadable — until activated.'
            )}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='version-upload-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex-1 space-y-4 overflow-y-auto px-3 py-3 pb-4 sm:space-y-6 sm:px-4'
          >
            <FormField
              control={form.control}
              name='version'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Version')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='1.0.0' />
                  </FormControl>
                  <FormDescription>{t('Semver: X.Y.Z')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

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
                  <FormMessage />
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
            form='version-upload-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Uploading...') : t('Upload')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
