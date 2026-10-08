// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'
import {
  handPickedModels,
  isPickingModels,
  isSwitchedOn,
  keyFormOf,
  keyHolderLabel,
  keyInput,
  keyModelsSummary,
  keyPatch,
  keyState,
  keyStateLabel,
  keyTemplateHint,
  keyTemplateLabel,
  maskedKey,
  newKeyForm,
} from '../lib/keys'
import { keyHolders, keyOf, keyTemplates } from './fixtures'

// Enterprise Org P5 (meta-repo docs/enterprise-org-prd.md §3): what the keys
// page says about a key and what its form asks the backend for. The quota is
// in the console's display currency on the form and in quota units on the
// wire; these tests run on the default, 500,000 units to the dollar.

/** A translator that fills in the placeholders and changes nothing else. */
const t = ((key: string, values?: Record<string, unknown>) =>
  key.replace(/{{(\w+)}}/g, (_, name: string) =>
    String(values?.[name] ?? '')
  )) as unknown as TFunction

const NOW = 1790000000
const HOUR = 3600

describe('the state a key is shown in', () => {
  it('is working while the key is enabled, in time and has quota', () => {
    expect(keyState(keyOf(), NOW)).toBe('active')
    expect(keyState(keyOf({ expired_time: NOW + HOUR }), NOW)).toBe('active')
  })

  it('does not count a key without a quota limit as out of quota', () => {
    const key = keyOf({ unlimited_quota: true, remain_quota: 0 })
    expect(keyState(key, NOW)).toBe('active')
  })

  it('reads expiry and quota off the key, whatever the status says', () => {
    // With a cache in front of it the gateway turns such keys away without
    // ever writing that into the status, so the status alone would say
    // "working" about a key that does not.
    expect(keyState(keyOf({ expired_time: NOW - 1 }), NOW)).toBe('expired')
    expect(keyState(keyOf({ expired_time: NOW }), NOW)).toBe('expired')
    expect(keyState(keyOf({ remain_quota: 0 }), NOW)).toBe('exhausted')
    expect(keyState(keyOf({ remain_quota: -3 }), NOW)).toBe('exhausted')
  })

  it('follows the status the gateway wrote', () => {
    expect(keyState(keyOf({ status: 3 }), NOW)).toBe('expired')
    expect(keyState(keyOf({ status: 4 }), NOW)).toBe('exhausted')
  })

  it('says frozen first: that is what someone did to the key', () => {
    const key = keyOf({ status: 2, expired_time: NOW - 1, remain_quota: 0 })
    expect(keyState(key, NOW)).toBe('frozen')
  })

  it('offers unfreeze for everything that is not switched on', () => {
    expect(isSwitchedOn(keyOf())).toBe(true)
    for (const status of [2, 3, 4]) {
      expect(isSwitchedOn(keyOf({ status })), String(status)).toBe(false)
    }
    // Expired by the clock but still switched on: there is nothing to lift.
    expect(isSwitchedOn(keyOf({ expired_time: NOW - 1 }))).toBe(true)
  })

  it('has a word for every state', () => {
    expect(keyStateLabel(t, 'active')).toBe('Working')
    expect(keyStateLabel(t, 'frozen')).toBe('Frozen')
    expect(keyStateLabel(t, 'expired')).toBe('Expired')
    expect(keyStateLabel(t, 'exhausted')).toBe('Out of quota')
  })
})

describe('what a key is shown as', () => {
  it('writes the masked value with its prefix', () => {
    expect(maskedKey(keyOf())).toBe('sk-abcd**********wxyz')
  })

  it('names the templates, and shows one it has no name for as its key', () => {
    expect(keyTemplateLabel(t, 'creative')).toBe('Creative pack')
    expect(keyTemplateLabel(t, 'coding')).toBe('Coding pack')
    expect(keyTemplateLabel(t, 'research')).toBe('research')
  })

  it('says what a template allows from the purposes the backend lists', () => {
    expect(keyTemplateHint(t, undefined)).toBe(
      'The key may call every model the organization can use.'
    )
    expect(keyTemplateHint(t, keyTemplates[0])).toBe(
      'The key may only call models for: image generation, video generation, chat and writing.'
    )
    expect(keyTemplateHint(t, keyTemplates[1])).toBe(
      'The key may only call models for: writing code.'
    )
    // A purpose added on the backend first shows as its code.
    expect(keyTemplateHint(t, { key: 'x', purposes: ['voice'] })).toBe(
      'The key may only call models for: voice.'
    )
  })

  it('sums up what a key may call', () => {
    expect(keyModelsSummary(t, keyOf())).toBe('Every model')
    expect(
      keyModelsSummary(
        t,
        keyOf({ policy_template: 'coding', model_limits: ['a', 'b'] })
      )
    ).toBe('Coding pack')
    expect(keyModelsSummary(t, keyOf({ model_limits: ['a', 'b', 'c'] }))).toBe(
      '3 model(s)'
    )
  })

  it('tells a hand-picked list from the list a template put on the key', () => {
    // PRD §5 (D33): a template's name says what it allows; a hand-picked list
    // has to be spelled out.
    expect(handPickedModels(keyOf({ model_limits: ['a', 'b'] }))).toEqual([
      'a',
      'b',
    ])
    expect(
      handPickedModels(
        keyOf({ policy_template: 'coding', model_limits: ['a'] })
      )
    ).toEqual([])
    expect(handPickedModels(keyOf())).toEqual([])
  })

  it('tells the owner, a service account and a person apart in the holder list', () => {
    expect(keyHolderLabel(t, keyHolders[0])).toBe(
      'Fiona Founder (owner — not handed out yet)'
    )
    expect(keyHolderLabel(t, keyHolders[1])).toBe('sally · Sales')
    expect(keyHolderLabel(t, keyHolders[2])).toBe(
      'CI Pipeline (service account) · General'
    )
  })
})

describe('the form of a new key', () => {
  it('starts parked under the owner, with a limited quota', () => {
    // Wherever the owner stands in the list: the backend sorts by id, and the
    // owner need not be the oldest account of the organization.
    const ownerLast = [...keyHolders.slice(1), keyHolders[0]]
    expect(newKeyForm(ownerLast).holderId).toBe(1)
    expect(newKeyForm(keyHolders)).toEqual({
      name: '',
      holderId: 1,
      template: '',
      picksModels: false,
      models: [],
      unlimited: false,
      quota: 10,
      expires: undefined,
      rpm: 0,
      tpm: 0,
      monthly: 0,
    })
  })

  it('starts on the first member when the owner is not on offer', () => {
    // A manager who may create keys is offered their departments only.
    expect(newKeyForm(keyHolders.slice(1)).holderId).toBe(3)
    expect(newKeyForm([]).holderId).toBe(0)
  })

  it('asks for what was filled in, in quota units and Unix seconds', () => {
    const expires = new Date('2027-01-02T03:04:05Z')
    expect(
      keyInput({
        name: '  Video team  ',
        holderId: 4,
        template: 'creative',
        picksModels: false,
        models: [],
        unlimited: false,
        quota: 12.5,
        expires,
        rpm: 60,
        tpm: 90000,
        monthly: 1000,
      })
    ).toEqual({
      name: 'Video team',
      holder_id: 4,
      policy_template: 'creative',
      model_limits: [],
      remain_quota: 6250000,
      unlimited_quota: false,
      expired_time: Math.floor(expires.getTime() / 1000),
      rpm_limit: 60,
      tpm_limit: 90000,
      monthly_limit: 1000,
    })
  })

  it('asks for a hand-picked list only while the form is picking models', () => {
    // PRD §5 (D33): 不套模板时可以手动指定模型.
    const form = { ...newKeyForm(keyHolders), models: ['gpt-4o', 'model-b'] }
    const picking = { ...form, picksModels: true }
    expect(isPickingModels(form)).toBe(false)
    expect(isPickingModels(picking)).toBe(true)

    expect(keyInput(picking)).toMatchObject({
      policy_template: '',
      model_limits: ['gpt-4o', 'model-b'],
    })
    // Picks left in the form by someone who went back to "every model".
    expect(keyInput(form).model_limits).toEqual([])
    // A template takes precedence: the backend refuses the two together.
    const templated = { ...picking, template: 'coding' }
    expect(isPickingModels(templated)).toBe(false)
    expect(keyInput(templated)).toMatchObject({
      policy_template: 'coding',
      model_limits: [],
    })
  })

  it('sends no quota for a key without a limit, and -1 for one that never expires', () => {
    const input = keyInput({ ...newKeyForm(keyHolders), unlimited: true })
    expect(input.unlimited_quota).toBe(true)
    expect(input.remain_quota).toBe(0)
    expect(input.expired_time).toBe(-1)
  })
})

describe('the change a form makes to an existing key', () => {
  // A key that has been spending: what is left is no round number.
  const key = keyOf({
    remain_quota: 1234567,
    used_quota: 3765433,
    policy_template: 'coding',
    model_limits: ['a'],
    expired_time: NOW + HOUR,
    rpm_limit: 60,
  })

  it('opens on what the key has', () => {
    expect(keyFormOf(key)).toEqual({
      name: 'Design tools',
      holderId: 3,
      template: 'coding',
      // The list a template put on the key is not a hand-picked one.
      picksModels: false,
      models: [],
      unlimited: false,
      quota: 1234567 / 500000,
      expires: new Date((NOW + HOUR) * 1000),
      rpm: 60,
      tpm: 0,
      monthly: 0,
    })
    expect(keyFormOf(keyOf()).expires).toBeUndefined()
  })

  it('opens a hand-picked key on its list', () => {
    const form = keyFormOf(keyOf({ model_limits: ['gpt-4o', 'model-b'] }))
    expect(form.template).toBe('')
    expect(form.picksModels).toBe(true)
    expect(form.models).toEqual(['gpt-4o', 'model-b'])
    expect(keyFormOf(keyOf()).picksModels).toBe(false)
  })

  it('is empty when nothing was edited', () => {
    expect(keyPatch(key, keyFormOf(key), false)).toEqual({})
  })

  it('carries only the field that was edited — never the quota along with it', () => {
    // The key spends while the form is open: sending back the quota the form
    // opened with would hand the key what it has spent since.
    expect(
      keyPatch(key, { ...keyFormOf(key), name: ' Renamed ' }, false)
    ).toEqual({ name: 'Renamed' })
    expect(keyPatch(key, { ...keyFormOf(key), tpm: 500 }, false)).toEqual({
      tpm_limit: 500,
    })
    expect(keyPatch(key, { ...keyFormOf(key), rpm: 0 }, false)).toEqual({
      rpm_limit: 0,
    })
    expect(keyPatch(key, { ...keyFormOf(key), monthly: 9 }, false)).toEqual({
      monthly_limit: 9,
    })
  })

  it('sends a quota that was typed', () => {
    expect(keyPatch(key, { ...keyFormOf(key), quota: 20 }, false)).toEqual({
      remain_quota: 10000000,
    })
    expect(keyPatch(key, { ...keyFormOf(key), quota: 0 }, false)).toEqual({
      remain_quota: 0,
    })
  })

  it('lifts the limit without sending a quota', () => {
    expect(
      keyPatch(key, { ...keyFormOf(key), unlimited: true }, false)
    ).toEqual({ unlimited_quota: true })
  })

  it('says the quota when a limit is put on, whatever the field held', () => {
    const unlimited = keyOf({ unlimited_quota: true, remain_quota: 0 })
    expect(
      keyPatch(unlimited, { ...keyFormOf(unlimited), unlimited: false }, false)
    ).toEqual({ unlimited_quota: false, remain_quota: 0 })
    expect(
      keyPatch(
        unlimited,
        { ...keyFormOf(unlimited), unlimited: false, quota: 5 },
        false
      )
    ).toEqual({ unlimited_quota: false, remain_quota: 2500000 })
  })

  it('sets, moves and clears the expiry', () => {
    const later = new Date((NOW + 2 * HOUR) * 1000)
    expect(keyPatch(key, { ...keyFormOf(key), expires: later }, false)).toEqual(
      { expired_time: NOW + 2 * HOUR }
    )
    expect(
      keyPatch(key, { ...keyFormOf(key), expires: undefined }, false)
    ).toEqual({ expired_time: -1 })
    const never = keyOf()
    expect(
      keyPatch(never, { ...keyFormOf(never), expires: later }, false)
    ).toEqual({ expired_time: NOW + 2 * HOUR })
  })

  it('changes the template, and takes it off', () => {
    expect(
      keyPatch(key, { ...keyFormOf(key), template: 'creative' }, false)
    ).toEqual({ policy_template: 'creative' })
    expect(keyPatch(key, { ...keyFormOf(key), template: '' }, false)).toEqual({
      policy_template: '',
    })
  })

  it('sends the unchanged template only when asked to apply it again', () => {
    expect(keyPatch(key, keyFormOf(key), true)).toEqual({
      policy_template: 'coding',
    })
    expect(keyPatch(key, keyFormOf(key), false)).toEqual({})
  })
})

describe('the change a form makes to what a key may call', () => {
  // PRD §5 (D33). The template and the hand-picked list are sent each only
  // when it changed; the backend replaces the two together and reads the one
  // left out as empty.
  const open = keyOf()
  const templated = keyOf({ policy_template: 'coding', model_limits: ['x*'] })
  const picked = keyOf({ model_limits: ['gpt-4o', 'model-b'] })
  /** The form of a key after someone edited what it may call. */
  const edited = (
    key: typeof open,
    change: Partial<ReturnType<typeof keyFormOf>>
  ) => keyPatch(key, { ...keyFormOf(key), ...change }, false)

  it('puts a hand-picked list on a key that could call everything', () => {
    expect(edited(open, { picksModels: true, models: ['gpt-4o'] })).toEqual({
      model_limits: ['gpt-4o'],
    })
  })

  it('sends a list that grew, shrank or was swapped — and nothing for one in another order', () => {
    expect(
      edited(picked, { models: ['gpt-4o', 'model-b', 'model-c'] })
    ).toEqual({ model_limits: ['gpt-4o', 'model-b', 'model-c'] })
    expect(edited(picked, { models: ['model-b'] })).toEqual({
      model_limits: ['model-b'],
    })
    expect(edited(picked, { models: ['gpt-4o', 'model-c'] })).toEqual({
      model_limits: ['gpt-4o', 'model-c'],
    })
    expect(edited(picked, { models: ['model-b', 'gpt-4o'] })).toEqual({})
  })

  it('lifts a hand-picked list by sending an empty one', () => {
    expect(edited(picked, { picksModels: false })).toEqual({ model_limits: [] })
  })

  it('goes from a list to a template, and from a template to a list', () => {
    expect(edited(picked, { template: 'creative' })).toEqual({
      policy_template: 'creative',
      model_limits: [],
    })
    expect(
      edited(templated, {
        template: '',
        picksModels: true,
        models: ['gpt-4o'],
      })
    ).toEqual({ policy_template: '', model_limits: ['gpt-4o'] })
  })

  it('takes a template off without mentioning a list the key never had', () => {
    expect(edited(templated, { template: '' })).toEqual({ policy_template: '' })
  })

  it('does not ask for picks made under a template that was kept', () => {
    expect(
      edited(templated, { picksModels: true, models: ['gpt-4o'] })
    ).toEqual({})
  })

  it('leaves the list alone when something else was edited', () => {
    // A founder's first key carries the rules of a personal purpose: nothing
    // the form could offer, and nothing it sends back unasked.
    const legacy = keyOf({ model_limits: ['claude-*', 'deeprouter-auto'] })
    expect(keyPatch(legacy, keyFormOf(legacy), false)).toEqual({})
    expect(edited(legacy, { name: 'Founder’s key' })).toEqual({
      name: 'Founder’s key',
    })
    expect(edited(picked, { rpm: 30 })).toEqual({ rpm_limit: 30 })
  })
})
