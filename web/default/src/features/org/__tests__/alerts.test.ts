// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'
import {
  alertSettingsForm,
  alertSettingsOf,
  alertStateLabel,
  alertSummary,
  alertTitle,
  type OrgAlertSettingsForm,
  timeZoneChoices,
  viewerTimeZone,
} from '../lib/alerts'
import type { OrgAlert, OrgAlertSettings } from '../types'

// Enterprise Org P8 and P9 (meta-repo docs/enterprise-org-prd.md §4): how an
// alert reads on the page, and what the settings form sends. The backend
// checks the settings too, but answers a bad request with one message for all
// of them — so which field is wrong is the form's to say, and these tests pin
// that it draws every line where the backend does. Amounts run on the
// default, 500,000 quota units to the dollar.

/** A translator that fills in the placeholders and changes nothing else. */
const t = ((key: string, values?: Record<string, unknown>) =>
  key.replace(/{{(\w+)}}/g, (_, name: string) =>
    String(values?.[name] ?? '')
  )) as unknown as TFunction

/** An alert on sally's key, as `GET /api/org/alerts` lists one. */
function alertOf(over: Partial<OrgAlert>): OrgAlert {
  return {
    id: 1,
    rule: 'quota',
    level: 80,
    key_id: 100,
    holder_id: 3,
    holder: 'sally',
    department_id: 11,
    department: 'Sales',
    detail: { key: 'Design tools' },
    created_time: 1790000000,
    state: '',
    acked_by: 0,
    acked_by_name: '',
    acked_time: 0,
    ...over,
  }
}

/** What a new organization's settings are. */
const DEFAULTS: OrgAlertSettings = {
  warn_at: [80, 100],
  spike_multiple: 5,
  off_hours_percent: 50,
  min_spend: 500000,
  work_hours: null,
}

/** The form of the default settings with some fields typed over. */
function formOf(
  over: Partial<OrgAlertSettingsForm> = {}
): OrgAlertSettingsForm {
  return { ...alertSettingsForm(DEFAULTS, 'Asia/Shanghai'), ...over }
}

/** A form with working hours switched on, Monday to Friday, nine to six. */
function workingFormOf(
  over: Partial<OrgAlertSettingsForm> = {}
): OrgAlertSettingsForm {
  return formOf({ hasWorkHours: true, ...over })
}

describe('what an alert is called', () => {
  it('names a warning by how much of the allowance is used', () => {
    expect(alertTitle(t, alertOf({ rule: 'quota', level: 80 }))).toBe(
      '80% of quota used'
    )
    expect(alertTitle(t, alertOf({ rule: 'monthly', level: 50 }))).toBe(
      '50% of monthly requests used'
    )
  })

  it('says the allowance is gone at 100%, which is when the key stops', () => {
    expect(alertTitle(t, alertOf({ rule: 'quota', level: 100 }))).toBe(
      'Quota used up'
    )
    expect(alertTitle(t, alertOf({ rule: 'monthly', level: 100 }))).toBe(
      'Monthly requests used up'
    )
  })

  it('says a key refused for lack of quota has stopped, whatever its level reads', () => {
    const refused = alertOf({
      rule: 'quota',
      level: 100,
      detail: { key: 'k', used: 4650000, limit: 5000000, needed: 600000 },
    })
    expect(alertTitle(t, refused)).toBe('Quota too low to use')
    expect(alertSummary(t, refused)).toBe(
      '$9.3 used of $10. A request that needed $1.2 was refused.'
    )
  })

  it('names each anomaly rule', () => {
    expect(alertTitle(t, alertOf({ rule: 'spike', level: 0 }))).toBe(
      'Sudden rise in spending'
    )
    expect(alertTitle(t, alertOf({ rule: 'offhours', level: 0 }))).toBe(
      'Spending outside working hours'
    )
    expect(alertTitle(t, alertOf({ rule: 'new_ip', level: 0 }))).toBe(
      'Unfamiliar IP address'
    )
  })

  it('shows a rule it has no words for as its code', () => {
    const future = alertOf({ rule: 'from_the_future', level: 0 })
    expect(alertTitle(t, future)).toBe('from_the_future')
    expect(alertSummary(t, future)).toBe('')
  })
})

describe('the numbers behind an alert', () => {
  it('gives a quota warning in money and a monthly one in requests', () => {
    expect(
      alertSummary(
        t,
        alertOf({
          rule: 'quota',
          detail: { key: 'k', used: 4000000, limit: 5000000 },
        })
      )
    ).toBe('$8 used of $10')
    expect(
      alertSummary(
        t,
        alertOf({
          rule: 'monthly',
          detail: { key: 'k', used: 800, limit: 1000 },
        })
      )
    ).toBe('800 of 1000 requests made this month')
  })

  it('sets what a key spent in a day against what it usually spends', () => {
    const detail = { key: 'k', spent: 2500000, daily_average: 250000 }
    expect(alertSummary(t, alertOf({ rule: 'spike', level: 0, detail }))).toBe(
      '$5 in the last 24 hours, against $0.5 a day over the week before'
    )
    expect(
      alertSummary(t, alertOf({ rule: 'offhours', level: 0, detail }))
    ).toBe(
      '$5 outside working hours in the last 24 hours, against $0.5 a day over the week before'
    )
  })

  it('lists the unfamiliar addresses and says what was known before', () => {
    expect(
      alertSummary(
        t,
        alertOf({
          rule: 'new_ip',
          level: 0,
          detail: {
            key: 'k',
            ips: ['203.0.113.5', '198.51.100.7'],
            known_ips: 3,
          },
        })
      )
    ).toBe(
      'Used from 203.0.113.5, 198.51.100.7. In the 30 days before it was used from 3 other address(es).'
    )
    // A key nobody had used from anywhere: the first address is unfamiliar too.
    expect(
      alertSummary(
        t,
        alertOf({
          rule: 'new_ip',
          level: 0,
          detail: { key: 'k', ips: ['203.0.113.5'] },
        })
      )
    ).toBe(
      'Used from 203.0.113.5. No address is on record for it in the 30 days before.'
    )
  })

  it('reads an alert whose numbers are missing as zero rather than failing', () => {
    expect(alertSummary(t, alertOf({ rule: 'quota', detail: {} }))).toBe(
      '$0 used of $0'
    )
    expect(alertSummary(t, alertOf({ rule: 'new_ip', detail: {} }))).toBe(
      'Used from . No address is on record for it in the 30 days before.'
    )
  })
})

describe('whether an alert has been dealt with', () => {
  it('has a word for each state', () => {
    expect(alertStateLabel(t, '')).toBe('Unresolved')
    expect(alertStateLabel(t, 'handled')).toBe('Handled')
    expect(alertStateLabel(t, 'false_alarm')).toBe('False alarm')
  })
})

describe('the settings form', () => {
  it('opens on the stored settings, the floor in dollars', () => {
    expect(alertSettingsForm(DEFAULTS, 'Europe/Berlin')).toEqual({
      warnAt: '80, 100',
      spikeMultiple: '5',
      offHoursPercent: '50',
      minSpend: '1',
      hasWorkHours: false,
      // With no working hours set, the usual ones wait behind the switch: the
      // viewer's own zone, Monday to Friday, nine to six.
      timezone: 'Europe/Berlin',
      days: [1, 2, 3, 4, 5],
      start: '09:00',
      end: '18:00',
    })
  })

  it('opens on the working hours an organization has set', () => {
    const form = alertSettingsForm(
      {
        ...DEFAULTS,
        warn_at: [],
        min_spend: 1250000,
        work_hours: {
          timezone: 'America/New_York',
          days: [0, 6],
          start: 7 * 60 + 30,
          end: 24 * 60,
        },
      },
      'Europe/Berlin'
    )
    expect(form.warnAt).toBe('')
    expect(form.minSpend).toBe('2.5')
    expect(form.hasWorkHours).toBe(true)
    expect(form.timezone).toBe('America/New_York')
    expect(form.days).toEqual([0, 6])
    expect(form.start).toBe('07:30')
    // A day that runs to midnight ends at 00:00 on the clock.
    expect(form.end).toBe('00:00')
  })

  it('sends back what it was opened on when nothing is touched', () => {
    const stored: OrgAlertSettings[] = [
      DEFAULTS,
      { ...DEFAULTS, warn_at: [], min_spend: 0 },
      { ...DEFAULTS, warn_at: [10, 25, 50, 75, 100], min_spend: 1234567 },
      {
        ...DEFAULTS,
        spike_multiple: 1000,
        off_hours_percent: 1000,
        work_hours: {
          timezone: 'Asia/Shanghai',
          days: [1, 2, 3, 4, 5],
          start: 540,
          end: 1080,
        },
      },
      {
        ...DEFAULTS,
        work_hours: { timezone: 'UTC', days: [0, 6], start: 0, end: 1440 },
      },
    ]
    for (const settings of stored) {
      const read = alertSettingsOf(t, alertSettingsForm(settings, 'UTC'))
      expect(read.errors).toEqual({})
      expect(read.settings).toEqual(settings)
    }
  })

  describe('the warning levels', () => {
    /** The levels a typed list comes out as, or the complaint about it. */
    const levels = (warnAt: string) => {
      const read = alertSettingsOf(t, formOf({ warnAt }))
      return read.settings?.warn_at ?? read.errors.warnAt
    }

    it('takes them however they are separated, in order and once each', () => {
      expect(levels('80,100')).toEqual([80, 100])
      expect(levels('100 80  80')).toEqual([80, 100])
      expect(levels(' 50，80、100；1 ')).toEqual([1, 50, 80, 100])
      expect(levels('7')).toEqual([7])
    })

    it('takes none at all: an organization may want no warnings', () => {
      expect(levels('')).toEqual([])
      expect(levels('  ,  ')).toEqual([])
    })

    it('holds each level to a whole number from 1 to 100', () => {
      for (const bad of ['0', '101', '80, 0', 'eighty', '8.5', '-5', '80%']) {
        expect(levels(bad), bad).toBe(
          'Each level is a whole number from 1 to 100.'
        )
      }
      expect(levels('1, 100')).toEqual([1, 100])
    })

    it('holds them to five', () => {
      expect(levels('10, 20, 30, 40, 50')).toEqual([10, 20, 30, 40, 50])
      expect(levels('10, 20, 30, 40, 50, 60')).toBe('At most 5 levels.')
      // The same level typed twice is one level.
      expect(levels('10, 20, 30, 40, 50, 50')).toEqual([10, 20, 30, 40, 50])
    })
  })

  describe('the anomaly rules', () => {
    it('holds the multiple to a whole number from 2 to 1000', () => {
      for (const bad of ['1', '0', '1001', '', '2.5', 'five', '-3']) {
        const read = alertSettingsOf(t, formOf({ spikeMultiple: bad }))
        expect(read.settings, bad).toBeUndefined()
        expect(read.errors).toEqual({
          spikeMultiple: 'A whole number from 2 to 1000.',
        })
      }
      for (const [typed, stored] of [
        ['2', 2],
        ['1000', 1000],
        [' 12 ', 12],
      ] as const) {
        expect(
          alertSettingsOf(t, formOf({ spikeMultiple: typed })).settings
            ?.spike_multiple
        ).toBe(stored)
      }
    })

    it('holds the off-hours share to a whole number from 1 to 1000', () => {
      for (const bad of ['0', '1001', '', '12.5']) {
        const read = alertSettingsOf(t, workingFormOf({ offHoursPercent: bad }))
        expect(read.settings, bad).toBeUndefined()
        expect(read.errors).toEqual({
          offHoursPercent: 'A whole number from 1 to 1000.',
        })
      }
      expect(
        alertSettingsOf(t, workingFormOf({ offHoursPercent: '1' })).settings
          ?.off_hours_percent
      ).toBe(1)
    })

    it('turns the floor from dollars into quota units, and refuses less than nothing', () => {
      for (const [typed, stored] of [
        ['0', 0],
        ['1', 500000],
        ['2.5', 1250000],
        ['0.01', 5000],
      ] as const) {
        expect(
          alertSettingsOf(t, formOf({ minSpend: typed })).settings?.min_spend,
          typed
        ).toBe(stored)
      }
      for (const bad of ['-1', '', 'a dollar']) {
        const read = alertSettingsOf(t, formOf({ minSpend: bad }))
        expect(read.settings, bad).toBeUndefined()
        expect(read.errors).toEqual({ minSpend: 'An amount of 0 or more.' })
      }
    })
  })

  describe('the working hours', () => {
    it('are not sent, and not checked, while the switch is off', () => {
      const read = alertSettingsOf(
        t,
        formOf({
          timezone: 'Nowhere/At_All',
          days: [],
          start: '18:00',
          end: '09:00',
        })
      )
      expect(read.errors).toEqual({})
      expect(read.settings?.work_hours).toBeNull()
    })

    it('go out as a zone, days in order and minutes of the day', () => {
      expect(
        alertSettingsOf(
          t,
          workingFormOf({
            timezone: ' Europe/Berlin ',
            days: [5, 1, 3],
            start: '08:30',
            end: '17:45',
          })
        ).settings?.work_hours
      ).toEqual({
        timezone: 'Europe/Berlin',
        days: [1, 3, 5],
        start: 8 * 60 + 30,
        end: 17 * 60 + 45,
      })
    })

    it('need a time zone the browser knows — and not "wherever the server is"', () => {
      for (const bad of ['', 'Local', 'Mars/Olympus_Mons', 'not a zone']) {
        const read = alertSettingsOf(t, workingFormOf({ timezone: bad }))
        expect(read.settings, bad).toBeUndefined()
        expect(read.errors).toEqual({
          timezone: 'Choose a time zone from the list.',
        })
      }
      for (const good of ['UTC', 'Asia/Shanghai', 'America/New_York']) {
        expect(
          alertSettingsOf(t, workingFormOf({ timezone: good })).errors,
          good
        ).toEqual({})
      }
    })

    it('need at least one working day', () => {
      const read = alertSettingsOf(t, workingFormOf({ days: [] }))
      expect(read.settings).toBeUndefined()
      expect(read.errors).toEqual({ days: 'Choose at least one working day.' })
      expect(
        alertSettingsOf(t, workingFormOf({ days: [0] })).settings?.work_hours
          ?.days
      ).toEqual([0])
    })

    it('need a day that ends after it starts', () => {
      const complaint =
        'The day has to end later than it starts. Hours that run past midnight are not supported.'
      for (const [start, end] of [
        ['18:00', '09:00'],
        ['09:00', '09:00'],
        ['', '18:00'],
        ['09:00', ''],
        ['25:00', '26:00'],
      ]) {
        const read = alertSettingsOf(t, workingFormOf({ start, end }))
        expect(read.settings, `${start}–${end}`).toBeUndefined()
        expect(read.errors).toEqual({ hours: complaint })
      }
    })

    it('read an end at 00:00 as midnight at the end of the day', () => {
      expect(
        alertSettingsOf(t, workingFormOf({ start: '08:00', end: '00:00' }))
          .settings?.work_hours
      ).toMatchObject({ start: 480, end: 1440 })
      // The whole day, every minute of it.
      expect(
        alertSettingsOf(t, workingFormOf({ start: '00:00', end: '00:00' }))
          .settings?.work_hours
      ).toMatchObject({ start: 0, end: 1440 })
    })
  })

  it('names every field that is wrong at once', () => {
    const read = alertSettingsOf(
      t,
      workingFormOf({
        warnAt: '150',
        spikeMultiple: '1',
        offHoursPercent: '0',
        minSpend: '-2',
        timezone: 'Local',
        days: [],
        start: '10:00',
        end: '08:00',
      })
    )
    expect(read.settings).toBeUndefined()
    expect(Object.keys(read.errors).sort()).toEqual([
      'days',
      'hours',
      'minSpend',
      'offHoursPercent',
      'spikeMultiple',
      'timezone',
      'warnAt',
    ])
  })
})

describe('the time zones on offer', () => {
  it('start with the one the form is on and name none twice', () => {
    const zones = timeZoneChoices('Asia/Shanghai')
    expect(zones[0]).toBe('Asia/Shanghai')
    expect(zones).toContain('UTC')
    expect(new Set(zones).size).toBe(zones.length)
    // A form that is on no zone yet does not offer an empty one.
    expect(timeZoneChoices('')).not.toContain('')
  })

  it('knows which zone the viewer is in', () => {
    const zone = viewerTimeZone()
    expect(zone).not.toBe('')
    expect(
      alertSettingsOf(t, workingFormOf({ timezone: zone })).errors
    ).toEqual({})
  })
})
