// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { DateTimePicker } from '@/components/datetime-picker'
import { MultiSelect } from '@/components/multi-select'
import {
  createOrgKey,
  fetchOrgKeyModels,
  orgQueryKeys,
  updateOrgKey,
} from '../api'
import {
  isPickingModels,
  keyFormOf,
  keyInput,
  keyPatch,
  keyTemplateHint,
  keyTemplateLabel,
  newKeyForm,
  type OrgKeyForm,
} from '../lib/keys'
import type {
  OrgKey,
  OrgKeyGrant,
  OrgKeyHolder,
  OrgKeyTemplate,
} from '../types'
import { HolderPicker } from './holder-picker'
import { OrgSelect } from './org-select'

/** Longest key name the backend accepts, in characters. */
const KEY_NAME_MAX_LENGTH = 50

/**
 * The two choices of the template list that are not templates: every model,
 * and a list picked by hand. To the backend both are "no template" — the empty
 * string, which a select cannot tell from "nothing chosen" — so the list has
 * its own words for them. No template's key starts with a colon.
 */
const EVERY_MODEL = ':every'
const CHOSEN_MODELS = ':chosen'

type KeyDialogProps = {
  open: boolean
  /** Set to change this key; leave out to create one. */
  editing?: OrgKey
  /** The members a new key can be made out to. */
  holders: OrgKeyHolder[]
  templates: OrgKeyTemplate[]
  onClose: () => void
  /** A key was created; the grant carries its value if it is a service account's. */
  onCreated: (grant: OrgKeyGrant) => void
  onUpdated: () => void
}

/** Creates a key, or changes what an existing one is allowed. */
export function KeyDialog(props: KeyDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={(open) => !open && props.onClose()}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-xl'>
        {/* Keyed so each opening starts from that key's values. */}
        {props.open && <KeyForm key={props.editing?.id ?? 'new'} {...props} />}
      </DialogContent>
    </Dialog>
  )
}

/** A whole number of at least zero out of a number field; anything else is zero. */
function wholeNumber(raw: string): number {
  const value = Math.floor(Number(raw))
  return Number.isFinite(value) && value > 0 ? value : 0
}

type ModelPickerProps = {
  /** Whose key it is: the list offers what this member can be served. */
  holderId: number
  selected: string[]
  onChange: (models: string[]) => void
}

/**
 * The hand-picked list of a key: the models its holder can be served, to
 * search and choose from. It is only on the page while someone is picking, so
 * the list is only fetched then. A model the key is limited to already stays
 * on show when its holder can no longer be served it; taking it out is up to
 * whoever edits the key.
 */
function ModelPicker({ holderId, selected, onChange }: ModelPickerProps) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: orgQueryKeys.keyModels(holderId),
    queryFn: () => fetchOrgKeyModels(holderId),
    enabled: holderId > 0,
    staleTime: 60 * 1000,
  })
  const offered = query.data ?? []

  return (
    <div
      role='group'
      aria-label={t('Specific models')}
      className='grid gap-1.5'
      // With the list of models open, Escape closes the list. Only the next
      // press reaches the dialog and closes that.
      onKeyDown={(event) => {
        if (
          event.key === 'Escape' &&
          event.target instanceof HTMLInputElement
        ) {
          event.stopPropagation()
        }
      }}
    >
      <MultiSelect
        options={offered.map((name) => ({ label: name, value: name }))}
        selected={selected}
        onChange={onChange}
        placeholder={t('Search and choose models')}
        // Without the list's own inset, so the field lines up with the others.
        className='p-0'
      />
      <p className='text-muted-foreground text-xs'>
        {query.isError
          ? t('Could not load the models to choose from.')
          : query.isLoading
            ? t('Loading the models to choose from…')
            : offered.length === 0
              ? t('There is no model the holder of this key can use right now.')
              : selected.length === 0
                ? t('Choose at least one model.')
                : t('{{count}} model(s) selected', { count: selected.length })}
      </p>
    </div>
  )
}

/** The form inside the dialog, mounted fresh on every opening. */
function KeyForm({
  editing,
  holders,
  templates,
  onClose,
  onCreated,
  onUpdated,
}: KeyDialogProps) {
  const { t } = useTranslation()
  const [form, setForm] = useState<OrgKeyForm>(() =>
    editing ? keyFormOf(editing) : newKeyForm(holders)
  )
  const [reapply, setReapply] = useState(false)
  const [saving, setSaving] = useState(false)
  const set = (change: Partial<OrgKeyForm>) =>
    setForm((current) => ({ ...current, ...change }))

  const currency = getCurrencyLabel()
  const inTokens = getCurrencyDisplay().meta.kind === 'tokens'
  const holder = holders.find((h) => h.id === form.holderId)
  const template = templates.find((option) => option.key === form.template)
  // Applying the same template again only means something for a key that has
  // one and keeps it.
  const canReapply = Boolean(
    editing &&
    editing.policy_template &&
    form.template === editing.policy_template
  )
  const nobodyToCreateFor = !editing && holders.length === 0
  const picking = isPickingModels(form)
  // A hand-picked list with nothing in it would be "every model" to the
  // backend — the opposite of what picking says — so it cannot be saved.
  const ready =
    form.name.trim() !== '' &&
    !nobodyToCreateFor &&
    !(picking && form.models.length === 0)

  const save = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!ready) return
    setSaving(true)
    try {
      if (editing) {
        const patch = keyPatch(editing, form, canReapply && reapply)
        if (Object.keys(patch).length === 0) {
          onClose()
          return
        }
        const res = await updateOrgKey(editing.id, patch)
        if (res.success) {
          toast.success(t('Key updated'))
          onUpdated()
          onClose()
        }
        return
      }
      const res = await createOrgKey(keyInput(form))
      if (res.success && res.data) {
        onCreated(res.data)
        onClose()
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={save} className='grid gap-4'>
      <DialogHeader>
        <DialogTitle>{editing ? t('Edit key') : t('New key')}</DialogTitle>
        <DialogDescription>
          {editing
            ? t(
                'Changes apply to the next request the key makes. Its value stays the same.'
              )
            : t(
                'Decide who the key is for, what it may call and how much it may spend.'
              )}
        </DialogDescription>
      </DialogHeader>

      <div className='grid gap-2'>
        <Label htmlFor='org-key-name'>{t('Name')}</Label>
        <Input
          id='org-key-name'
          value={form.name}
          maxLength={KEY_NAME_MAX_LENGTH}
          placeholder={t('e.g. Design team — video')}
          autoFocus
          onChange={(e) => set({ name: e.target.value })}
        />
      </div>

      {editing ? (
        <p className='text-muted-foreground text-sm'>
          {t('Held by {{holder}}', {
            holder: [
              editing.holder || t('Someone who has left'),
              editing.department,
            ]
              .filter(Boolean)
              .join(' · '),
          })}
        </p>
      ) : (
        <div className='grid gap-2'>
          <Label htmlFor='org-key-holder'>{t('Who it is for')}</Label>
          {nobodyToCreateFor ? (
            <p className='text-muted-foreground text-sm'>
              {t('There is nobody you can create a key for yet.')}
            </p>
          ) : (
            <HolderPicker
              id='org-key-holder'
              holders={holders}
              value={form.holderId}
              onChange={(holderId) => set({ holderId })}
            />
          )}
          {/* What happens to the key's value depends on who holds it, and is
              said before the key exists rather than after. */}
          {holder && (
            <p className='text-muted-foreground text-xs'>
              {holder.is_service
                ? t(
                    'A service account cannot sign in, so the full key is shown to you once, right after it is created. Save it then.'
                  )
                : holder.is_owner
                  ? t(
                      'The key waits under the owner. Its value is not shown to anyone here.'
                    )
                  : t(
                      'The key’s value is not shown to you. {{name}} installs it into their own tools with one-click setup, from their API keys page.',
                      { name: holder.name }
                    )}
            </p>
          )}
        </div>
      )}

      <div className='grid gap-2'>
        <Label htmlFor='org-key-template'>{t('Policy template')}</Label>
        <OrgSelect
          id='org-key-template'
          options={[
            { value: EVERY_MODEL, label: t('Every model') },
            ...templates.map((option) => ({
              value: option.key,
              label: keyTemplateLabel(t, option.key),
            })),
            { value: CHOSEN_MODELS, label: t('Specific models') },
          ]}
          value={
            form.template || (form.picksModels ? CHOSEN_MODELS : EVERY_MODEL)
          }
          onChange={(next) =>
            next === EVERY_MODEL || next === CHOSEN_MODELS
              ? set({ template: '', picksModels: next === CHOSEN_MODELS })
              : set({ template: next })
          }
        />
        <p className='text-muted-foreground text-xs'>
          {picking
            ? t('The key may only call the models you choose here.')
            : keyTemplateHint(t, template)}
        </p>
        {picking && (
          <ModelPicker
            holderId={form.holderId}
            selected={form.models}
            onChange={(models) => set({ models })}
          />
        )}
        {canReapply && (
          <div className='flex items-start gap-2'>
            <Checkbox
              id='org-key-reapply'
              className='mt-0.5'
              checked={reapply}
              onCheckedChange={(checked) => setReapply(checked === true)}
            />
            <Label
              htmlFor='org-key-reapply'
              className='flex-col items-start gap-0.5 text-left leading-5 font-normal'
            >
              <span>{t('Apply the template again')}</span>
              <span className='text-muted-foreground text-xs'>
                {t(
                  'A template is turned into a list of models when it is applied. Tick this to pick up models added since.'
                )}
              </span>
            </Label>
          </div>
        )}
      </div>

      <div className='grid gap-3 rounded-lg border p-3'>
        <div className='flex items-center justify-between gap-3'>
          <Label htmlFor='org-key-unlimited' className='font-normal'>
            {t('No quota limit')}
          </Label>
          <Switch
            id='org-key-unlimited'
            checked={form.unlimited}
            onCheckedChange={(unlimited) => set({ unlimited })}
          />
        </div>
        {!form.unlimited && (
          <div className='grid gap-2'>
            <Label htmlFor='org-key-quota'>
              {editing
                ? t('Quota left ({{currency}})', { currency })
                : t('Quota ({{currency}})', { currency })}
            </Label>
            <Input
              id='org-key-quota'
              type='number'
              min={0}
              step={inTokens ? 1 : 0.01}
              value={form.quota}
              onChange={(e) =>
                set({ quota: Math.max(0, Number(e.target.value) || 0) })
              }
            />
            {editing && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'What the key may still spend, not what it started with. Spent so far: {{amount}}.',
                  { amount: formatQuota(editing.used_quota) }
                )}
              </p>
            )}
          </div>
        )}
      </div>

      <fieldset className='grid gap-2'>
        <legend className='mb-2 text-sm leading-none font-medium'>
          {t('Valid until')}
        </legend>
        <DateTimePicker
          value={form.expires}
          onChange={(expires) => set({ expires })}
          placeholder={t('Never expires')}
          className='min-w-0 [&_input[type=time]]:w-28'
        />
      </fieldset>

      <fieldset className='grid gap-2'>
        <legend className='mb-2 text-sm leading-none font-medium'>
          {t('Limits')}
        </legend>
        <div className='grid gap-3 sm:grid-cols-3'>
          <div className='grid gap-1.5'>
            <Label htmlFor='org-key-rpm' className='text-xs font-normal'>
              {t('Requests per minute')}
            </Label>
            <Input
              id='org-key-rpm'
              type='number'
              min={0}
              value={form.rpm}
              onChange={(e) => set({ rpm: wholeNumber(e.target.value) })}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label htmlFor='org-key-tpm' className='text-xs font-normal'>
              {t('Tokens per minute')}
            </Label>
            <Input
              id='org-key-tpm'
              type='number'
              min={0}
              value={form.tpm}
              onChange={(e) => set({ tpm: wholeNumber(e.target.value) })}
            />
          </div>
          <div className='grid gap-1.5'>
            <Label htmlFor='org-key-monthly' className='text-xs font-normal'>
              {t('Requests per month')}
            </Label>
            <Input
              id='org-key-monthly'
              type='number'
              min={0}
              value={form.monthly}
              onChange={(e) => set({ monthly: wholeNumber(e.target.value) })}
            />
          </div>
        </div>
        <p className='text-muted-foreground text-xs'>
          {t('0 means no limit.')}
        </p>
      </fieldset>

      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          onClick={onClose}
          disabled={saving}
        >
          {t('Cancel')}
        </Button>
        <Button type='submit' disabled={saving || !ready}>
          {saving ? t('Saving...') : editing ? t('Save') : t('Create')}
        </Button>
      </DialogFooter>
    </form>
  )
}
