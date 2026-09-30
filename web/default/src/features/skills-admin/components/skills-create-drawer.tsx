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
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
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
import { createSkill } from '../api'
import {
  CREATE_SKILL_FORM_DEFAULT_VALUES,
  type CreateSkillFormValues,
  getCreateSkillFormSchema,
  parseTagsInput,
} from '../lib/skill-form'
import { useSkills } from './skills-provider'

type SkillsCreateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SkillsCreateDrawer({
  open,
  onOpenChange,
}: SkillsCreateDrawerProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { triggerRefresh } = useSkills()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<CreateSkillFormValues>({
    resolver: zodResolver(getCreateSkillFormSchema(t)),
    defaultValues: CREATE_SKILL_FORM_DEFAULT_VALUES,
  })

  const monetizationType = form.watch('monetization_type')
  const listingType = form.watch('listing_type')
  const isReference = listingType === 'reference'

  const onSubmit = async (data: CreateSkillFormValues) => {
    setIsSubmitting(true)
    try {
      const result = await createSkill({
        slug: data.slug,
        name: data.name,
        description: data.description,
        tags: parseTagsInput(data.tags),
        // A reference listing is free-only (backend also enforces this via
        // ErrReferenceMustBeFree) — the schema's refine already blocks
        // submitting monetization_type:"paid" for one, so this is just
        // being explicit about what actually goes over the wire.
        monetization_type: data.monetization_type,
        price_usd: data.monetization_type === 'paid' ? data.price_usd : 0,
        listing_type: data.listing_type,
        source_url:
          data.listing_type === 'reference' ? data.source_url : undefined,
      })
      if (result.success && result.data) {
        toast.success(t('Skill created as draft'))
        onOpenChange(false)
        triggerRefresh()
        navigate({
          to: '/admin/skills/$id/edit',
          params: { id: String(result.data.id) },
        })
      }
    } catch (_error) {
      // Errors are handled by the global interceptor (toast + reject)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) form.reset(CREATE_SKILL_FORM_DEFAULT_VALUES)
      }}
    >
      <SheetContent className='flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-[600px]'>
        <SheetHeader className='border-b px-4 py-3 text-start sm:px-6 sm:py-4'>
          <SheetTitle>{t('Create Skill')}</SheetTitle>
          <SheetDescription>
            {isReference
              ? t(
                  'Creates a draft skill that links out to an external repo — no package, no version to upload.'
                )
              : t(
                  'Creates a draft skill. Upload and activate a version before publishing.'
                )}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='skill-create-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex-1 space-y-4 overflow-y-auto px-3 py-3 pb-4 sm:space-y-6 sm:px-4'
          >
            <FormField
              control={form.control}
              name='listing_type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Listing Type')}</FormLabel>
                  <FormControl>
                    <NativeSelect
                      value={field.value}
                      onChange={(e) => {
                        const next = e.target
                          .value as CreateSkillFormValues['listing_type']
                        field.onChange(next)
                        // A reference listing is free-only (backend
                        // enforces this too) — reset so switching types
                        // never leaves a stale paid+reference combination.
                        if (next === 'reference') {
                          form.setValue('monetization_type', 'free')
                        }
                      }}
                    >
                      <NativeSelectOption value='hosted'>
                        {t('Hosted (upload & package)')}
                      </NativeSelectOption>
                      <NativeSelectOption value='reference'>
                        {t('Reference (link to external repo)')}
                      </NativeSelectOption>
                    </NativeSelect>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {isReference && (
              <FormField
                control={form.control}
                name='source_url'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Source URL')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder='https://github.com/owner/repo'
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Where the "View on GitHub" button on the skill page links to.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={t('Code Review Expert')} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='slug'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Slug')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='code-review-expert' />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Locked once the skill is published — used in the download URL and package.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Description')}</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={3} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='tags'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Tags')}</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='code, review' />
                  </FormControl>
                  <FormDescription>
                    {t('Comma-separated, optional')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {!isReference && (
              <FormField
                control={form.control}
                name='monetization_type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Monetization')}</FormLabel>
                    <FormControl>
                      <NativeSelect
                        value={field.value}
                        onChange={(e) =>
                          field.onChange(
                            e.target
                              .value as CreateSkillFormValues['monetization_type']
                          )
                        }
                      >
                        <NativeSelectOption value='free'>
                          {t('Free')}
                        </NativeSelectOption>
                        <NativeSelectOption value='paid'>
                          {t('Paid')}
                        </NativeSelectOption>
                      </NativeSelect>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            {!isReference && monetizationType === 'paid' && (
              <FormField
                control={form.control}
                name='price_usd'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Price (USD)')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        step={0.01}
                        min={0}
                        onChange={(e) =>
                          field.onChange(parseFloat(e.target.value) || 0)
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
          </form>
        </Form>
        <SheetFooter className='grid grid-cols-2 gap-2 border-t px-4 py-3 sm:flex sm:px-6 sm:py-4'>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button
            form='skill-create-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Creating...') : t('Create')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
