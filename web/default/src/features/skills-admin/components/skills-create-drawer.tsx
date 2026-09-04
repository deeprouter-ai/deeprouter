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
import { ComboboxInput } from '@/components/ui/combobox-input'
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
import { getSkillCategoryOptions } from '../constants'
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
  const categoryOptions = getSkillCategoryOptions(t)

  const form = useForm<CreateSkillFormValues>({
    resolver: zodResolver(getCreateSkillFormSchema(t)),
    defaultValues: CREATE_SKILL_FORM_DEFAULT_VALUES,
  })

  const monetizationType = form.watch('monetization_type')

  const onSubmit = async (data: CreateSkillFormValues) => {
    setIsSubmitting(true)
    try {
      const result = await createSkill({
        slug: data.slug,
        name: data.name,
        description: data.description,
        category: data.category,
        tags: parseTagsInput(data.tags),
        monetization_type: data.monetization_type,
        price_usd: data.monetization_type === 'paid' ? data.price_usd : 0,
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
            {t(
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
              name='category'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Category')}</FormLabel>
                  <FormControl>
                    <ComboboxInput
                      options={categoryOptions}
                      value={field.value}
                      onValueChange={field.onChange}
                      allowCustomValue
                      placeholder={t('Select or type a category')}
                    />
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

            {monetizationType === 'paid' && (
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
