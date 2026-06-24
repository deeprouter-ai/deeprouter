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
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircle, GitBranch } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { TitledCard } from '@/components/ui/titled-card'
import { StatusBadge } from '@/components/status-badge'
import {
  createAdminSkill,
  createAdminSkillVersion,
  getAdminSkillAuditLog,
  getAdminSkillVersions,
  updateAdminSkill,
} from '../api'
import type {
  AdminSkill,
  AdminSkillAuditEntry,
  CreateSkillPayload,
  CreateVersionPayload,
  SkillVersionMetadata,
  UpdateSkillPayload,
} from '../types'

const SLUG_REGEX = /^[a-z0-9](?:[a-z0-9-]{0,126}[a-z0-9])?$/

const editorSchema = z
  .object({
    slug: z
      .string()
      .min(1, 'Required')
      .max(128, 'Max 128 characters')
      .regex(
        SLUG_REGEX,
        'Lowercase letters, numbers and hyphens only; must start and end with a letter or digit'
      ),
    name: z.string().min(1, 'Required').max(160, 'Max 160 characters'),
    short_description: z
      .string()
      .min(1, 'Required')
      .max(280, 'Max 280 characters'),
    description: z.string().min(1, 'Required'),
    category: z.string().min(1, 'Required').max(64, 'Max 64 characters'),
    tags: z.string().optional(),
    icon_url: z.string().optional(),
    input_hints: z.string().optional(),
    example_inputs: z.string().optional(),
    example_outputs: z.string().optional(),
    required_plan: z.enum(['free', 'pro', 'enterprise']),
    monetization_type: z.enum(['free', 'plan_included', 'token_markup']),
    price_markup: z.string().optional(),
    free_quota_per_month: z.string().optional(),
    max_input_tokens: z.string().optional(),
    instruction_template: z.string().optional(),
    output_schema: z.string().optional(),
    model_whitelist: z.string().optional(),
    timeout_seconds: z.string().optional(),
    is_kids_safe: z.boolean().optional(),
    is_kids_exclusive: z.boolean().optional(),
    kids_approval_status: z.enum([
      'not_required',
      'pending',
      'approved',
      'emergency_approved',
      'rejected',
      'revoked',
    ]),
    featured_flag: z.boolean().optional(),
    featured_rank: z.string().optional(),
  })
  .superRefine((data, ctx) => {
    const needsMaxInputTokens =
      data.required_plan === 'free' ||
      data.monetization_type === 'free' ||
      hasNumberValue(data.free_quota_per_month)

    if (needsMaxInputTokens && !data.max_input_tokens?.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Required for Free or free-quota Skills',
        path: ['max_input_tokens'],
      })
    }

    if (
      data.monetization_type === 'token_markup' &&
      (!data.price_markup?.trim() || Number(data.price_markup) <= 0)
    ) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Required and must be > 0 for token markup pricing',
        path: ['price_markup'],
      })
    }

    if (hasNumberValue(data.timeout_seconds)) {
      const timeout = Number(data.timeout_seconds)
      if (timeout < 1 || timeout > 120) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'Must be between 1 and 120 seconds',
          path: ['timeout_seconds'],
        })
      }
    }

    if (hasNumberValue(data.featured_rank) && Number(data.featured_rank) < 0) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Must be 0 or greater',
        path: ['featured_rank'],
      })
    }

    if (data.is_kids_exclusive && !data.is_kids_safe) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Kids Exclusive requires Kids Safe',
        path: ['is_kids_exclusive'],
      })
    }

    if (data.output_schema?.trim()) {
      try {
        JSON.parse(data.output_schema)
      } catch {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'Must be valid JSON',
          path: ['output_schema'],
        })
      }
    }
  })

type EditorValues = z.infer<typeof editorSchema>
type StringFieldName = {
  [K in keyof EditorValues]-?: NonNullable<EditorValues[K]> extends string
    ? K
    : never
}[keyof EditorValues]
type BooleanFieldName = {
  [K in keyof EditorValues]-?: NonNullable<EditorValues[K]> extends boolean
    ? K
    : never
}[keyof EditorValues]

interface AdminSkillEditorProps {
  skill: AdminSkill | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated?: (skill: AdminSkill) => void
}

const createDefaults: EditorValues = {
  slug: '',
  name: '',
  short_description: '',
  description: '',
  category: '',
  tags: '',
  icon_url: '',
  input_hints: '',
  example_inputs: '',
  example_outputs: '',
  required_plan: 'pro',
  monetization_type: 'plan_included',
  price_markup: '',
  free_quota_per_month: '',
  max_input_tokens: '',
  instruction_template: '',
  output_schema: '',
  model_whitelist: '',
  timeout_seconds: '45',
  is_kids_safe: false,
  is_kids_exclusive: false,
  kids_approval_status: 'not_required',
  featured_flag: false,
  featured_rank: '',
}

export function AdminSkillEditor({
  skill,
  open,
  onOpenChange,
  onCreated,
}: AdminSkillEditorProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isEditMode = skill != null
  const [templateDirty, setTemplateDirty] = useState(false)

  const form = useForm<EditorValues>({
    resolver: zodResolver(editorSchema),
    defaultValues: isEditMode ? skillToDefaults(skill) : createDefaults,
  })

  useEffect(() => {
    if (!open) return
    form.reset(isEditMode ? skillToDefaults(skill) : createDefaults)
    setTemplateDirty(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, skill?.id])

  const monetizationType = form.watch('monetization_type')
  const requiredPlan = form.watch('required_plan')
  const freeQuota = form.watch('free_quota_per_month')
  const isKidsSafe = form.watch('is_kids_safe')

  const needsMaxInputTokens =
    requiredPlan === 'free' ||
    monetizationType === 'free' ||
    hasNumberValue(freeQuota)

  const { data: versionsData, isLoading: versionsLoading } = useQuery({
    queryKey: ['admin-skill-versions', skill?.id],
    queryFn: () => getAdminSkillVersions(skill!.id, { page: 1, limit: 20 }),
    enabled: isEditMode && open,
  })

  const { data: auditData, isLoading: auditLoading } = useQuery({
    queryKey: ['admin-skill-audit-log', skill?.id],
    queryFn: () => getAdminSkillAuditLog(skill!.id, { page: 1, limit: 20 }),
    enabled: isEditMode && open,
  })

  const createSkillMutation = useMutation({ mutationFn: createAdminSkill })
  const updateSkillMutation = useMutation({
    mutationFn: ({
      skillId,
      payload,
    }: {
      skillId: string
      payload: UpdateSkillPayload
    }) => updateAdminSkill(skillId, payload),
  })
  const createVersionMutation = useMutation({
    mutationFn: ({
      skillId,
      payload,
    }: {
      skillId: string
      payload: CreateVersionPayload
    }) => createAdminSkillVersion(skillId, payload),
  })

  async function onSubmit(values: EditorValues) {
    if (isEditMode) {
      await saveExistingSkill(values)
      return
    }
    await createSkillDraft(values)
  }

  async function createSkillDraft(values: EditorValues) {
    const payload = buildSkillPayload(values, false) as CreateSkillPayload

    try {
      const created = await createSkillMutation.mutateAsync(payload)
      if (values.instruction_template?.trim()) {
        await createVersionMutation.mutateAsync({
          skillId: created.id,
          payload: buildVersionPayload(values),
        })
      }
      await queryClient.invalidateQueries({ queryKey: ['admin-skills'] })
      toast.success(t('Skill draft created successfully.'))
      onCreated?.(created)
      onOpenChange(false)
    } catch (err) {
      handleSaveError(err)
    }
  }

  async function saveExistingSkill(values: EditorValues) {
    if (!skill) return
    const updatePayload = buildSkillPayload(values, true) as UpdateSkillPayload
    const shouldCreateVersion =
      templateDirty && Boolean(values.instruction_template?.trim())
    let updated: AdminSkill | null = null
    let updateSkipped = false

    try {
      try {
        updated = await updateSkillMutation.mutateAsync({
          skillId: skill.id,
          payload: updatePayload,
        })
      } catch (err) {
        if (!isNotFoundError(err) || !shouldCreateVersion) {
          throw err
        }
        updateSkipped = true
      }

      if (shouldCreateVersion) {
        await createVersionMutation.mutateAsync({
          skillId: skill.id,
          payload: buildVersionPayload(values),
        })
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-skills'] }),
        queryClient.invalidateQueries({
          queryKey: ['admin-skill-versions', skill.id],
        }),
        queryClient.invalidateQueries({
          queryKey: ['admin-skill-audit-log', skill.id],
        }),
      ])
      if (updated) {
        form.reset(skillToDefaults(updated))
      }
      setTemplateDirty(false)
      toast.success(
        updateSkipped
          ? t('New version created. Metadata save is waiting on DR-52.')
          : shouldCreateVersion
            ? t('Skill saved and a new version was created.')
            : t('Skill saved successfully.')
      )
    } catch (err) {
      handleSaveError(err)
    }
  }

  function isNotFoundError(err: unknown) {
    return (
      (err as { response?: { status?: number } })?.response?.status === 404
    )
  }

  function handleSaveError(err: unknown) {
    const apiErr = err as {
      response?: { data?: { error?: { code?: string; message?: string } } }
      message?: string
    }
    const code = apiErr?.response?.data?.error?.code
    const msg =
      apiErr?.response?.data?.error?.message ??
      (err as Error | null)?.message ??
      t('Failed to save skill.')
    if (code === 'SKILL_CONFLICT') {
      form.setError('slug', {
        message: t('A Skill with this slug already exists.'),
      })
      return
    }
    toast.error(msg)
  }

  const isSaving =
    createSkillMutation.isPending ||
    updateSkillMutation.isPending ||
    createVersionMutation.isPending
  const versions: SkillVersionMetadata[] = versionsData?.data ?? []
  const auditRows: AdminSkillAuditEntry[] = auditData?.data ?? []

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        className='flex flex-col gap-0 p-0 sm:max-w-3xl'
        showCloseButton={false}
      >
        <SheetHeader className='shrink-0 border-b px-5 py-4'>
          <div className='flex items-center justify-between pr-2'>
            <div className='min-w-0'>
              <SheetTitle>
                {isEditMode ? skill.name : t('Create Skill')}
              </SheetTitle>
              <SheetDescription className='mt-0.5 font-mono text-xs'>
                {isEditMode ? skill.slug : t('New draft skill')}
              </SheetDescription>
            </div>
            <SheetClose render={<Button variant='outline' size='sm' />}>
              {t('Close')}
            </SheetClose>
          </div>
        </SheetHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex min-h-0 flex-1 flex-col'
          >
            <div className='flex-1 space-y-4 overflow-y-auto px-5 py-4'>
              <TitledCard title={t('Metadata')}>
                <div className='space-y-4'>
                  <div className='grid gap-4 sm:grid-cols-2'>
                    <TextField
                      control={form.control}
                      name='slug'
                      label={t('Slug')}
                      disabled={isEditMode}
                      placeholder='my-skill-name'
                    />
                    <TextField
                      control={form.control}
                      name='name'
                      label={t('Name')}
                      placeholder={t('Display name')}
                    />
                  </div>
                  <TextField
                    control={form.control}
                    name='short_description'
                    label={t('Short Description')}
                    placeholder={t('One-line summary')}
                  />
                  <TextAreaField
                    control={form.control}
                    name='description'
                    label={t('Description')}
                    rows={4}
                    placeholder={t('Full Markdown description')}
                  />
                  <div className='grid gap-4 sm:grid-cols-2'>
                    <TextField
                      control={form.control}
                      name='category'
                      label={t('Category')}
                      placeholder='writing'
                    />
                    <TextField
                      control={form.control}
                      name='icon_url'
                      label={t('Icon URL')}
                      placeholder='https://example.com/icon.png'
                    />
                  </div>
                  <TextField
                    control={form.control}
                    name='tags'
                    label={t('Tags')}
                    placeholder='writing, productivity, ai'
                  />
                </div>
              </TitledCard>

              <TitledCard title={t('User Guidance')}>
                <div className='space-y-4'>
                  <TextAreaField
                    control={form.control}
                    name='input_hints'
                    label={t('Input Hints')}
                    rows={3}
                    placeholder={t('One hint per line')}
                  />
                  <TextAreaField
                    control={form.control}
                    name='example_inputs'
                    label={t('Example Inputs')}
                    rows={3}
                    placeholder={t('One example input per line')}
                  />
                  <TextAreaField
                    control={form.control}
                    name='example_outputs'
                    label={t('Example Outputs')}
                    rows={3}
                    placeholder={t('One example output per line')}
                  />
                </div>
              </TitledCard>

              <TitledCard title={t('Entitlement')}>
                <div className='space-y-4'>
                  <div className='grid gap-4 sm:grid-cols-2'>
                    <SelectField
                      control={form.control}
                      name='required_plan'
                      label={t('Required Plan')}
                      options={[
                        ['free', t('Free')],
                        ['pro', t('Pro')],
                        ['enterprise', t('Enterprise')],
                      ]}
                    />
                    <SelectField
                      control={form.control}
                      name='monetization_type'
                      label={t('Monetization Type')}
                      options={[
                        ['free', t('Free')],
                        ['plan_included', t('Plan Included')],
                        ['token_markup', t('Token Markup')],
                      ]}
                    />
                  </div>
                  {monetizationType === 'token_markup' && (
                    <TextField
                      control={form.control}
                      name='price_markup'
                      label={t('Price Markup')}
                      type='number'
                      min={0.01}
                      step={0.01}
                      placeholder='0.25'
                    />
                  )}
                  <TextField
                    control={form.control}
                    name='free_quota_per_month'
                    label={t('Free Quota / Month')}
                    type='number'
                    min={0}
                    placeholder='10'
                  />
                </div>
              </TitledCard>

              <TitledCard title={t('Execution')}>
                <div className='space-y-4'>
                  {isEditMode && templateDirty && (
                    <Alert>
                      <GitBranch className='size-4' />
                      <AlertTitle>{t('Version change')}</AlertTitle>
                      <AlertDescription>
                        {t(
                          'Saving will create a new Skill version from this template.'
                        )}
                      </AlertDescription>
                    </Alert>
                  )}
                  <TextAreaField
                    control={form.control}
                    name='instruction_template'
                    label={
                      isEditMode
                        ? t('Instruction Template (creates a new version)')
                        : t('Instruction Template')
                    }
                    rows={8}
                    placeholder={t('System prompt / instruction template')}
                    className='font-mono text-xs'
                    onValueChange={() => {
                      if (isEditMode) setTemplateDirty(true)
                    }}
                  />
                  <TextAreaField
                    control={form.control}
                    name='output_schema'
                    label={t('Output Schema')}
                    rows={5}
                    placeholder='{"type":"object","properties":{}}'
                    className='font-mono text-xs'
                  />
                  <TextAreaField
                    control={form.control}
                    name='model_whitelist'
                    label={t('Model Whitelist')}
                    rows={3}
                    placeholder={t('One model alias per line')}
                  />
                  <div className='grid gap-4 sm:grid-cols-2'>
                    <TextField
                      control={form.control}
                      name='timeout_seconds'
                      label={t('Timeout Seconds')}
                      type='number'
                      min={1}
                      max={120}
                      placeholder='45'
                    />
                    <FormField
                      control={form.control}
                      name='max_input_tokens'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('Max Input Tokens')}
                            {needsMaxInputTokens && (
                              <span className='text-destructive ml-1'>*</span>
                            )}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={1}
                              placeholder='4096'
                              {...field}
                            />
                          </FormControl>
                          {needsMaxInputTokens && !field.value?.trim() && (
                            <p className='text-destructive text-xs'>
                              {t('Required for Free or free-quota Skills')}
                            </p>
                          )}
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </div>
              </TitledCard>

              <TitledCard title={t('Safety')}>
                <div className='space-y-4'>
                  <SwitchField
                    control={form.control}
                    name='is_kids_safe'
                    label={t('Kids Safe')}
                    onCheckedChange={(checked) => {
                      if (!checked) {
                        form.setValue('is_kids_exclusive', false, {
                          shouldDirty: true,
                        })
                      }
                    }}
                  />
                  <SwitchField
                    control={form.control}
                    name='is_kids_exclusive'
                    label={t('Kids Exclusive')}
                    disabled={!isKidsSafe}
                    onCheckedChange={(checked) => {
                      if (checked) {
                        form.setValue('is_kids_safe', true, {
                          shouldDirty: true,
                        })
                      }
                    }}
                  />
                  <SelectField
                    control={form.control}
                    name='kids_approval_status'
                    label={t('Approval Status')}
                    options={[
                      ['not_required', t('Not Required')],
                      ['pending', t('Pending')],
                      ['approved', t('Approved')],
                      ['emergency_approved', t('Emergency Approved')],
                      ['rejected', t('Rejected')],
                      ['revoked', t('Revoked')],
                    ]}
                  />
                </div>
              </TitledCard>

              <TitledCard title={t('Promotion')}>
                <div className='space-y-4'>
                  <SwitchField
                    control={form.control}
                    name='featured_flag'
                    label={t('Featured')}
                  />
                  <TextField
                    control={form.control}
                    name='featured_rank'
                    label={t('Featured Rank')}
                    type='number'
                    min={0}
                    placeholder='1'
                  />
                </div>
              </TitledCard>

              {isEditMode && (
                <TitledCard title={t('Version History')}>
                  {versionsLoading ? (
                    <p className='text-muted-foreground text-sm'>
                      {t('Loading versions...')}
                    </p>
                  ) : versions.length === 0 ? (
                    <p className='text-muted-foreground text-sm'>
                      {t(
                        'No versions yet. Add an instruction template above to create the first version.'
                      )}
                    </p>
                  ) : (
                    <div className='divide-y'>
                      {versions.map((v) => (
                        <VersionRow key={v.id} version={v} />
                      ))}
                    </div>
                  )}
                </TitledCard>
              )}

              {isEditMode && (
                <TitledCard title={t('Audit Log')}>
                  {auditLoading ? (
                    <p className='text-muted-foreground text-sm'>
                      {t('Loading audit log...')}
                    </p>
                  ) : auditRows.length === 0 ? (
                    <p className='text-muted-foreground text-sm'>
                      {t('No audit entries yet.')}
                    </p>
                  ) : (
                    <div className='divide-y'>
                      {auditRows.map((entry) => (
                        <AuditRow key={entry.id} entry={entry} />
                      ))}
                    </div>
                  )}
                </TitledCard>
              )}
            </div>

            <SheetFooter className='mt-0 shrink-0 border-t px-5 py-3'>
              {isEditMode ? (
                <p className='text-muted-foreground mr-auto text-xs'>
                  {templateDirty
                    ? t('Template edit will create a new version.')
                    : t('Metadata and config changes save in place.')}
                </p>
              ) : (
                <AlertCircle className='text-muted-foreground mr-auto hidden size-4 sm:block' />
              )}
              <Button type='submit' disabled={isSaving}>
                {isSaving
                  ? t('Saving...')
                  : isEditMode
                    ? t('Save')
                    : t('Create Draft')}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

function skillToDefaults(skill: AdminSkill): EditorValues {
  return {
    slug: skill.slug,
    name: skill.name,
    short_description: skill.short_description ?? '',
    description: skill.description ?? '',
    category: skill.category,
    tags: stringifyList(skill.tags, ', '),
    icon_url: skill.icon_url ?? '',
    input_hints: stringifyList(skill.input_hints, '\n'),
    example_inputs: stringifyList(skill.example_inputs, '\n'),
    example_outputs: stringifyList(skill.example_outputs, '\n'),
    required_plan: skill.required_plan,
    monetization_type: skill.monetization_type,
    price_markup:
      skill.price_markup && skill.price_markup > 0
        ? String(skill.price_markup)
        : '',
    free_quota_per_month:
      skill.free_quota_per_month != null
        ? String(skill.free_quota_per_month)
        : '',
    max_input_tokens:
      skill.max_input_tokens != null ? String(skill.max_input_tokens) : '',
    instruction_template: '',
    output_schema: '',
    model_whitelist: stringifyList(skill.model_whitelist, '\n'),
    timeout_seconds:
      skill.timeout_seconds != null ? String(skill.timeout_seconds) : '45',
    is_kids_safe: skill.is_kids_safe,
    is_kids_exclusive: skill.is_kids_exclusive,
    kids_approval_status: skill.kids_approval_status,
    featured_flag: skill.featured_flag,
    featured_rank:
      skill.featured_rank != null ? String(skill.featured_rank) : '',
  }
}

function buildSkillPayload(
  values: EditorValues,
  isEditMode: boolean
): CreateSkillPayload | UpdateSkillPayload {
  // In edit mode, send null for cleared nullable fields so the backend can
  // distinguish "user intentionally cleared this" from "field was not sent".
  const nullableInt = (v?: string): number | null | undefined =>
    isEditMode
      ? hasNumberValue(v)
        ? Number(v)
        : null
      : hasNumberValue(v)
        ? Number(v)
        : undefined

  // icon_url: always send in edit mode (empty string signals clear on backend).
  const iconUrl: string | null | undefined = isEditMode
    ? (values.icon_url?.trim() || null)
    : (values.icon_url?.trim() || undefined)

  const payload: CreateSkillPayload | UpdateSkillPayload = {
    slug: values.slug,
    name: values.name,
    short_description: values.short_description,
    description: values.description,
    category: values.category,
    tags: parseCommaList(values.tags),
    input_hints: parseLineList(values.input_hints),
    example_inputs: parseLineList(values.example_inputs),
    example_outputs: parseLineList(values.example_outputs),
    required_plan: values.required_plan,
    monetization_type: values.monetization_type,
    ...(values.monetization_type === 'token_markup' &&
    hasNumberValue(values.price_markup)
      ? { price_markup: Number(values.price_markup) }
      : {}),
    model_whitelist: parseLineList(values.model_whitelist),
    ...(hasNumberValue(values.timeout_seconds)
      ? { timeout_seconds: Number(values.timeout_seconds) }
      : {}),
    is_kids_safe: values.is_kids_safe ?? false,
    is_kids_exclusive: values.is_kids_exclusive ?? false,
    kids_approval_status: values.kids_approval_status,
    featured_flag: values.featured_flag ?? false,
  }

  if (iconUrl !== undefined) payload.icon_url = iconUrl
  const fqpm = nullableInt(values.free_quota_per_month)
  if (fqpm !== undefined) payload.free_quota_per_month = fqpm
  const mit = nullableInt(values.max_input_tokens)
  if (mit !== undefined) payload.max_input_tokens = mit
  const fr = nullableInt(values.featured_rank)
  if (fr !== undefined) payload.featured_rank = fr

  return payload
}

function buildVersionPayload(values: EditorValues): CreateVersionPayload {
  return {
    instruction_template: values.instruction_template?.trim() ?? '',
    ...(values.output_schema?.trim()
      ? { output_schema: JSON.parse(values.output_schema) as unknown }
      : {}),
  }
}

function hasNumberValue(value?: string) {
  return value != null && value.trim() !== '' && !Number.isNaN(Number(value))
}

function parseLineList(value?: string) {
  return (value ?? '')
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseCommaList(value?: string) {
  return (value ?? '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function stringifyList(value: unknown[] | undefined, joiner: string) {
  if (!Array.isArray(value)) return ''
  return value.map((item) => String(item)).join(joiner)
}

function VersionRow({ version }: { version: SkillVersionMetadata }) {
  const { t } = useTranslation()
  const statusVariant: Record<
    string,
    'success' | 'warning' | 'neutral' | 'danger'
  > = {
    active: 'success',
    draft: 'neutral',
    inactive: 'warning',
    archived: 'danger',
  }

  return (
    <div className='flex items-center gap-3 py-2.5 text-sm'>
      <span className='text-muted-foreground w-8 shrink-0 font-mono text-xs'>
        v{version.version_number}
      </span>
      <StatusBadge
        label={t(
          version.status.charAt(0).toUpperCase() + version.status.slice(1)
        )}
        variant={statusVariant[version.status] ?? 'neutral'}
        copyable={false}
      />
      <span className='text-muted-foreground truncate font-mono text-xs'>
        {version.instruction_template_sha256.slice(0, 8)}
      </span>
      <span className='text-muted-foreground ml-auto shrink-0 text-xs'>
        {new Date(version.created_at).toLocaleDateString()}
      </span>
    </div>
  )
}

function AuditRow({ entry }: { entry: AdminSkillAuditEntry }) {
  const { t } = useTranslation()

  return (
    <div className='space-y-1 py-2.5 text-sm'>
      <div className='flex items-center gap-2'>
        <span className='font-medium'>{t(labelFromAction(entry.action))}</span>
        <span className='text-muted-foreground ml-auto shrink-0 text-xs'>
          {new Date(entry.created_at).toLocaleString()}
        </span>
      </div>
      <div className='text-muted-foreground text-xs'>
        {t('Actor')} {entry.actor_id} - {entry.changed_fields.join(', ')}
      </div>
    </div>
  )
}

function labelFromAction(action: string) {
  return action
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function TextField({
  control,
  name,
  label,
  disabled,
  ...inputProps
}: {
  control: ReturnType<typeof useForm<EditorValues>>['control']
  name: StringFieldName
  label: string
  disabled?: boolean
} & React.ComponentProps<typeof Input>) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Input
              disabled={disabled}
              {...inputProps}
              name={field.name}
              onBlur={field.onBlur}
              onChange={field.onChange}
              ref={field.ref}
              value={String(field.value ?? '')}
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function TextAreaField({
  control,
  name,
  label,
  rows,
  className,
  onValueChange,
  ...textareaProps
}: {
  control: ReturnType<typeof useForm<EditorValues>>['control']
  name: StringFieldName
  label: string
  rows?: number
  className?: string
  onValueChange?: () => void
} & React.ComponentProps<typeof Textarea>) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Textarea
              rows={rows}
              className={className}
              {...textareaProps}
              name={field.name}
              onBlur={field.onBlur}
              ref={field.ref}
              value={String(field.value ?? '')}
              onChange={(event) => {
                field.onChange(event)
                onValueChange?.()
              }}
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function SelectField({
  control,
  name,
  label,
  options,
}: {
  control: ReturnType<typeof useForm<EditorValues>>['control']
  name: StringFieldName
  label: string
  options: Array<[string, string]>
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <Select value={String(field.value)} onValueChange={field.onChange}>
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              {options.map(([value, label]) => (
                <SelectItem key={value} value={value}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function SwitchField({
  control,
  name,
  label,
  disabled,
  onCheckedChange,
}: {
  control: ReturnType<typeof useForm<EditorValues>>['control']
  name: BooleanFieldName
  label: string
  disabled?: boolean
  onCheckedChange?: (checked: boolean) => void
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <div className='flex items-center gap-3'>
            <Switch
              checked={Boolean(field.value)}
              onCheckedChange={(checked) => {
                field.onChange(checked)
                onCheckedChange?.(checked)
              }}
              disabled={disabled}
            />
            <Label>{label}</Label>
          </div>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
