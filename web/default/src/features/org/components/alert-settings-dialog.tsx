// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Combobox } from '@/components/ui/combobox'
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
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { ErrorState } from '@/components/error-state'
import {
  fetchOrgAlertSettings,
  orgQueryKeys,
  updateOrgAlertSettings,
} from '../api'
import {
  alertSettingsForm,
  alertSettingsOf,
  MAX_WARN_LEVELS,
  type OrgAlertSettingsErrors,
  type OrgAlertSettingsForm,
  timeZoneChoices,
  viewerTimeZone,
} from '../lib/alerts'
import { weekdayLabel } from '../lib/audit'
import type { OrgAlertSettings } from '../types'

/** The days of the week in the order a working week is read: Monday first. */
const WEEK = [1, 2, 3, 4, 5, 6, 0]

type AlertSettingsDialogProps = {
  open: boolean
  onClose: () => void
}

/**
 * The settings the warnings and alerts run on (Enterprise Org PRD §4): at what
 * share of its allowance a key is warned about, what counts as unusual, and
 * when the organization works. The owner's and the admins' to change.
 */
export function AlertSettingsDialog({
  open,
  onClose,
}: AlertSettingsDialogProps) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: orgQueryKeys.alertSettings(),
    queryFn: fetchOrgAlertSettings,
    enabled: open,
    // Always asked again on opening: what the form starts from must be what
    // is stored.
    staleTime: 0,
  })

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t('Alert settings')}</DialogTitle>
          <DialogDescription>
            {t(
              'What your organization is told about its keys. Nothing here ever blocks a key: an alert is something to look at, and what to do is up to you.'
            )}
          </DialogDescription>
        </DialogHeader>
        {!open ? null : query.isError ? (
          <ErrorState
            description={t('Could not load the alert settings.')}
            onRetry={() => void query.refetch()}
          />
        ) : !query.data || query.isFetching ? (
          <div className='space-y-3'>
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className='h-10 rounded-lg' />
            ))}
          </div>
        ) : (
          <AlertSettingsForm settings={query.data} onClose={onClose} />
        )}
      </DialogContent>
    </Dialog>
  )
}

/** The message under a field that is wrong. */
function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null
  return (
    <p id={id} role='alert' className='text-destructive text-xs'>
      {message}
    </p>
  )
}

type AlertSettingsFormProps = {
  settings: OrgAlertSettings
  onClose: () => void
}

/** The form inside the dialog, mounted once the stored settings are there. */
function AlertSettingsForm({ settings, onClose }: AlertSettingsFormProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<OrgAlertSettingsForm>(() =>
    alertSettingsForm(settings, viewerTimeZone())
  )
  const [errors, setErrors] = useState<OrgAlertSettingsErrors>({})
  const [saving, setSaving] = useState(false)
  const set = (change: Partial<OrgAlertSettingsForm>) =>
    setForm((current) => ({ ...current, ...change }))

  const currency = getCurrencyLabel()
  const inTokens = getCurrencyDisplay().meta.kind === 'tokens'
  const zones = timeZoneChoices(form.timezone)

  const save = async (event: React.FormEvent) => {
    event.preventDefault()
    const read = alertSettingsOf(t, form)
    setErrors(read.errors)
    if (!read.settings) return
    setSaving(true)
    try {
      const res = await updateOrgAlertSettings(read.settings)
      if (res.success) {
        toast.success(t('Alert settings saved'))
        void queryClient.invalidateQueries({
          queryKey: orgQueryKeys.alertSettings(),
        })
        onClose()
      }
    } catch {
      // The global interceptor already said why.
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={save} noValidate className='grid gap-5'>
      <fieldset className='grid gap-2'>
        <legend className='mb-2 text-sm leading-none font-medium'>
          {t('Usage warnings')}
        </legend>
        <Label htmlFor='org-alert-warn-at' className='text-xs font-normal'>
          {t('Warn at these shares of a key’s allowance (%)')}
        </Label>
        <Input
          id='org-alert-warn-at'
          value={form.warnAt}
          placeholder='80, 100'
          inputMode='numeric'
          aria-invalid={Boolean(errors.warnAt)}
          aria-describedby='org-alert-warn-at-hint org-alert-warn-at-error'
          onChange={(e) => set({ warnAt: e.target.value })}
        />
        <FieldError id='org-alert-warn-at-error' message={errors.warnAt} />
        <p
          id='org-alert-warn-at-hint'
          className='text-muted-foreground text-xs'
        >
          {t(
            'When a key has used this much of its quota, or of its requests for the month, the owner, the admins and whoever holds the key are told — once per level. Up to {{max}} levels, separated by commas; leave it empty to send no warnings.',
            { max: MAX_WARN_LEVELS }
          )}
        </p>
      </fieldset>

      <fieldset className='grid gap-3'>
        <legend className='mb-2 text-sm leading-none font-medium'>
          {t('Unusual usage')}
        </legend>
        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='grid content-start gap-1.5'>
            <Label htmlFor='org-alert-spike' className='text-xs font-normal'>
              {t('Sudden rise: times the daily average')}
            </Label>
            <Input
              id='org-alert-spike'
              type='number'
              min={2}
              max={1000}
              value={form.spikeMultiple}
              aria-invalid={Boolean(errors.spikeMultiple)}
              aria-describedby='org-alert-spike-error'
              onChange={(e) => set({ spikeMultiple: e.target.value })}
            />
            <FieldError
              id='org-alert-spike-error'
              message={errors.spikeMultiple}
            />
          </div>
          <div className='grid content-start gap-1.5'>
            <Label
              htmlFor='org-alert-min-spend'
              className='text-xs font-normal'
            >
              {t('Smallest amount reported ({{currency}})', { currency })}
            </Label>
            <Input
              id='org-alert-min-spend'
              type='number'
              min={0}
              step={inTokens ? 1 : 0.01}
              value={form.minSpend}
              aria-invalid={Boolean(errors.minSpend)}
              aria-describedby='org-alert-min-spend-error'
              onChange={(e) => set({ minSpend: e.target.value })}
            />
            <FieldError
              id='org-alert-min-spend-error'
              message={errors.minSpend}
            />
          </div>
        </div>
        <p className='text-muted-foreground text-xs'>
          {t(
            'A key is reported when it spends, within 24 hours, more than this many times what it spent per day over the week before — and at least the smallest amount. An unfamiliar IP address is always reported. A key is only watched once it has been with its holder for 7 days.'
          )}
        </p>
      </fieldset>

      <div className='grid gap-3 rounded-lg border p-3'>
        <div className='flex items-center justify-between gap-3'>
          <Label htmlFor='org-alert-work-hours' className='font-normal'>
            {t('Report spending outside working hours')}
          </Label>
          <Switch
            id='org-alert-work-hours'
            checked={form.hasWorkHours}
            // Switched off, the fields under it go: whatever was typed into
            // them goes too, so nothing unseen can stop the form from saving.
            onCheckedChange={(hasWorkHours) =>
              set(
                hasWorkHours
                  ? { hasWorkHours }
                  : {
                      ...alertSettingsForm(settings, viewerTimeZone()),
                      warnAt: form.warnAt,
                      spikeMultiple: form.spikeMultiple,
                      minSpend: form.minSpend,
                      hasWorkHours,
                    }
              )
            }
          />
        </div>
        {!form.hasWorkHours ? (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Off: your organization has not said when it works, so this rule has nothing to go by.'
            )}
          </p>
        ) : (
          <>
            <div className='grid gap-1.5'>
              <Label
                htmlFor='org-alert-timezone'
                className='text-xs font-normal'
              >
                {t('Time zone')}
              </Label>
              <Combobox
                id='org-alert-timezone'
                options={zones.map((zone) => ({ value: zone, label: zone }))}
                value={form.timezone}
                onValueChange={(timezone) => set({ timezone: timezone ?? '' })}
                placeholder={t('Search time zones')}
                emptyText={t('No time zone by that name.')}
                allowCustomValue
              />
              <FieldError
                id='org-alert-timezone-error'
                message={errors.timezone}
              />
            </div>
            <fieldset className='grid gap-1.5'>
              <legend className='mb-1.5 text-xs'>{t('Working days')}</legend>
              <div className='flex flex-wrap gap-x-4 gap-y-2'>
                {WEEK.map((day) => (
                  <div key={day} className='flex items-center gap-1.5'>
                    <Checkbox
                      id={`org-alert-day-${day}`}
                      checked={form.days.includes(day)}
                      onCheckedChange={(checked) =>
                        set({
                          days:
                            checked === true
                              ? [...form.days, day]
                              : form.days.filter((d) => d !== day),
                        })
                      }
                    />
                    <Label
                      htmlFor={`org-alert-day-${day}`}
                      className='font-normal'
                    >
                      {weekdayLabel(t, day)}
                    </Label>
                  </div>
                ))}
              </div>
              <FieldError id='org-alert-days-error' message={errors.days} />
            </fieldset>
            <div className='grid gap-3 sm:grid-cols-3'>
              <div className='grid content-start gap-1.5'>
                <Label
                  htmlFor='org-alert-start'
                  className='text-xs font-normal'
                >
                  {t('Day starts')}
                </Label>
                <Input
                  id='org-alert-start'
                  type='time'
                  value={form.start}
                  aria-invalid={Boolean(errors.hours)}
                  onChange={(e) => set({ start: e.target.value })}
                />
              </div>
              <div className='grid content-start gap-1.5'>
                <Label htmlFor='org-alert-end' className='text-xs font-normal'>
                  {t('Day ends')}
                </Label>
                <Input
                  id='org-alert-end'
                  type='time'
                  value={form.end}
                  aria-invalid={Boolean(errors.hours)}
                  aria-describedby='org-alert-hours-error'
                  onChange={(e) => set({ end: e.target.value })}
                />
              </div>
              <div className='grid content-start gap-1.5'>
                <Label
                  htmlFor='org-alert-off-hours'
                  className='text-xs font-normal'
                >
                  {t('Report from (% of daily average)')}
                </Label>
                <Input
                  id='org-alert-off-hours'
                  type='number'
                  min={1}
                  max={1000}
                  value={form.offHoursPercent}
                  aria-invalid={Boolean(errors.offHoursPercent)}
                  aria-describedby='org-alert-off-hours-error'
                  onChange={(e) => set({ offHoursPercent: e.target.value })}
                />
              </div>
            </div>
            <FieldError id='org-alert-hours-error' message={errors.hours} />
            <FieldError
              id='org-alert-off-hours-error'
              message={errors.offHoursPercent}
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'A key is reported when, within 24 hours, what it spends outside these hours reaches that share of what it spent per day over the week before — and at least the smallest amount above. A day that ends at 00:00 runs to midnight.'
              )}
            </p>
          </>
        )}
      </div>

      <DialogFooter>
        <Button
          type='button'
          variant='outline'
          onClick={onClose}
          disabled={saving}
        >
          {t('Cancel')}
        </Button>
        <Button type='submit' disabled={saving}>
          {saving ? t('Saving...') : t('Save')}
        </Button>
      </DialogFooter>
    </form>
  )
}
