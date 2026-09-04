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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
import { Textarea } from '@/components/ui/textarea'
import { updateSkill } from '../api'
import { getSkillCategoryOptions } from '../constants'
import {
  formatTagsInput,
  getUpdateSkillFormSchema,
  parseTagsInput,
  type UpdateSkillFormValues,
} from '../lib/skill-form'
import type { SkillSummary } from '../types'

function toFormValues(skill: SkillSummary): UpdateSkillFormValues {
  return {
    name: skill.name,
    description: skill.description,
    category: skill.category,
    tags: formatTagsInput(skill.tags),
    monetization_type: skill.monetization_type,
    price_usd: skill.price_usd,
  }
}

export function SkillMetadataForm({
  skill,
  onSaved,
}: {
  skill: SkillSummary
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [pendingValues, setPendingValues] =
    useState<UpdateSkillFormValues | null>(null)
  const categoryOptions = getSkillCategoryOptions(t)

  const form = useForm<UpdateSkillFormValues>({
    resolver: zodResolver(getUpdateSkillFormSchema(t)),
    defaultValues: toFormValues(skill),
  })

  // Skill data can change under us (e.g. after an activation refetch) —
  // keep the form in sync unless the admin has unsaved edits pending.
  useEffect(() => {
    if (!form.formState.isDirty) {
      form.reset(toFormValues(skill))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [skill])

  const hasEverBeenPublished =
    skill.status === 'published' || skill.status === 'deprecated'

  const submit = async (values: UpdateSkillFormValues) => {
    setIsSubmitting(true)
    try {
      const result = await updateSkill(skill.id, {
        name: values.name,
        description: values.description,
        category: values.category,
        tags: parseTagsInput(values.tags),
        monetization_type: values.monetization_type,
        price_usd: values.monetization_type === 'paid' ? values.price_usd : 0,
      })
      if (result.success) {
        toast.success(t('Skill updated'))
        form.reset(values)
        onSaved()
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const onSubmit = (values: UpdateSkillFormValues) => {
    // PRD §6.3 verbatim: changing monetization on an already-published skill
    // needs an explicit confirm — it affects existing users' download rights.
    if (
      hasEverBeenPublished &&
      values.monetization_type !== skill.monetization_type
    ) {
      setPendingValues(values)
      return
    }
    submit(values)
  }

  const monetizationType = form.watch('monetization_type')

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Metadata')}</CardTitle>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <div className='text-muted-foreground font-mono text-sm'>
              {t('Slug')}: {skill.slug}
              <span className='ms-2 text-xs'>
                {t('(locked — cannot be changed here)')}
              </span>
            </div>

            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
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
                  <FormDescription>{t('Comma-separated')}</FormDescription>
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
                            .value as UpdateSkillFormValues['monetization_type']
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

            <Button
              type='submit'
              disabled={isSubmitting || !form.formState.isDirty}
            >
              {isSubmitting ? t('Saving...') : t('Save changes')}
            </Button>
          </form>
        </Form>
      </CardContent>

      <AlertDialog
        open={pendingValues !== null}
        onOpenChange={(v) => !v && setPendingValues(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Are you sure?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This will affect the download entitlement of users who already have this skill. Continue?'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (pendingValues) submit(pendingValues)
                setPendingValues(null)
              }}
            >
              {t('Continue')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}
