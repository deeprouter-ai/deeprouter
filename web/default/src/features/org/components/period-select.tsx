// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useTranslation } from 'react-i18next'
import { Label } from '@/components/ui/label'
import { DatePicker } from '@/components/date-picker'
import {
  type OrgPeriod,
  type OrgPeriodPreset,
  periodIsValid,
  periodLabel,
} from '../lib/usage'
import { OrgSelect } from './org-select'

type PeriodSelectProps = {
  /** Prefix of the ids of the fields: two of these can be on one page. */
  id: string
  /** The presets to offer, in order. */
  presets: OrgPeriodPreset[]
  value: OrgPeriod
  onChange: (period: OrgPeriod) => void
}

/**
 * Chooses the stretch of time a report or the audit log covers: a preset, or
 * — under "Custom dates" — a first and a last day.
 */
export function PeriodSelect({
  id,
  presets,
  value,
  onChange,
}: PeriodSelectProps) {
  const { t } = useTranslation()

  return (
    <>
      {/* On a phone it shares its line with the filter next to it. */}
      <div className='min-w-0 flex-1 sm:w-44 sm:flex-none'>
        <Label htmlFor={`${id}-preset`} className='sr-only'>
          {t('Period')}
        </Label>
        <OrgSelect
          id={`${id}-preset`}
          options={presets.map((preset) => ({
            value: preset,
            label: periodLabel(t, preset),
          }))}
          value={value.preset}
          onChange={(preset) => onChange({ ...value, preset })}
        />
      </div>
      {value.preset === 'custom' && (
        <div
          role='group'
          aria-label={t('Custom dates')}
          className='flex flex-wrap items-center gap-2'
        >
          <DatePicker
            selected={value.from}
            onSelect={(from) => onChange({ ...value, from })}
            placeholder={t('First day')}
          />
          <span aria-hidden='true' className='text-muted-foreground'>
            –
          </span>
          <DatePicker
            selected={value.to}
            onSelect={(to) => onChange({ ...value, to })}
            placeholder={t('Last day')}
          />
          {!periodIsValid(value) && (
            <p role='alert' className='text-destructive w-full text-sm'>
              {t('The last day is before the first.')}
            </p>
          )}
        </div>
      )}
    </>
  )
}
