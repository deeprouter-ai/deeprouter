// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import type {
  OrgKey,
  OrgKeyHolder,
  OrgKeyInput,
  OrgKeyPatch,
  OrgKeyTemplate,
} from '../types'

/** The gateway's status of a key that works. */
const STATUS_ENABLED = 1
/** …of one an administrator froze. */
const STATUS_FROZEN = 2
/** …and the two the gateway sets itself when it runs without a cache. */
const STATUS_EXPIRED = 3
const STATUS_EXHAUSTED = 4

/** What a key is doing right now, as a person would say it. */
export type OrgKeyState = 'active' | 'frozen' | 'expired' | 'exhausted'

/**
 * The state a key is shown in. Frozen is what someone did to it and comes
 * first. Expired and out of quota are read off the expiry and the quota as
 * well as the status: with a cache in front of it the gateway turns such a key
 * away without ever writing that into the status.
 */
export function keyState(key: OrgKey, nowSeconds: number): OrgKeyState {
  if (key.status === STATUS_FROZEN) return 'frozen'
  if (
    key.status === STATUS_EXPIRED ||
    (key.expired_time !== -1 && key.expired_time <= nowSeconds)
  ) {
    return 'expired'
  }
  if (
    key.status === STATUS_EXHAUSTED ||
    (!key.unlimited_quota && key.remain_quota <= 0)
  ) {
    return 'exhausted'
  }
  return 'active'
}

/**
 * Whether a key is switched on. Anything else is lifted by "unfreeze" — which
 * the backend refuses while the key has no quota left or is past its expiry.
 */
export function isSwitchedOn(key: OrgKey): boolean {
  return key.status === STATUS_ENABLED
}

/** A key's state in words. */
export function keyStateLabel(t: TFunction, state: OrgKeyState): string {
  switch (state) {
    case 'frozen':
      return t('Frozen')
    case 'expired':
      return t('Expired')
    case 'exhausted':
      return t('Out of quota')
    default:
      return t('Working')
  }
}

/** The masked value the way every key is written: with its `sk-` prefix. */
export function maskedKey(key: OrgKey): string {
  return `sk-${key.key}`
}

/**
 * Name of a policy template. A template the page has no name for yet — one
 * added on the backend first — shows as its key.
 */
export function keyTemplateLabel(t: TFunction, template: string): string {
  switch (template) {
    case 'creative':
      return t('Creative pack')
    case 'coding':
      return t('Coding pack')
    default:
      return template
  }
}

/** Name of one purpose a policy template is made of. */
function purposeLabel(t: TFunction, purpose: string): string {
  switch (purpose) {
    case 'image':
      return t('image generation')
    case 'video':
      return t('video generation')
    case 'chat':
      return t('chat and writing')
    case 'coding':
      return t('writing code')
    default:
      return purpose
  }
}

/**
 * What a policy template lets a key call, in one line. It is put together from
 * the purposes the backend lists for the template, so the page keeps no idea of
 * its own about what a template contains.
 */
export function keyTemplateHint(
  t: TFunction,
  template: OrgKeyTemplate | undefined
): string {
  if (!template) {
    return t('The key may call every model the organization can use.')
  }
  return t('The key may only call models for: {{purposes}}.', {
    purposes: template.purposes
      .map((purpose) => purposeLabel(t, purpose))
      .join(t(', ')),
  })
}

/** What a key may call, as short as a table cell needs it. */
export function keyModelsSummary(t: TFunction, key: OrgKey): string {
  if (key.policy_template) return keyTemplateLabel(t, key.policy_template)
  if (key.model_limits.length === 0) return t('Every model')
  return t('{{count}} model(s)', { count: key.model_limits.length })
}

/**
 * The models a key is limited to by hand: its list when no policy template
 * put it there, otherwise none. A template's name says what it allows; a
 * hand-picked list has to be spelled out.
 */
export function handPickedModels(key: OrgKey): string[] {
  return key.policy_template ? [] : key.model_limits
}

/** How a member is offered in the "who is it for" list. */
export function keyHolderLabel(t: TFunction, holder: OrgKeyHolder): string {
  if (holder.is_owner) {
    return t('{{name}} (owner — not handed out yet)', { name: holder.name })
  }
  if (holder.is_service) {
    return t('{{name}} (service account) · {{department}}', {
      name: holder.name,
      department: holder.department,
    })
  }
  return `${holder.name} · ${holder.department}`
}

/** What the key form holds, in the units a person types. */
export type OrgKeyForm = {
  name: string
  holderId: number
  /** The policy template; empty for none. */
  template: string
  /** Without a template: whether the key is limited to models picked by hand. */
  picksModels: boolean
  /** The models picked by hand; they count only while `picksModels` is on. */
  models: string[]
  unlimited: boolean
  /** The quota in the console's display currency. */
  quota: number
  /** Undefined means the key never expires. */
  expires: Date | undefined
  rpm: number
  tpm: number
  monthly: number
}

/**
 * The form of a new key. It is parked under the owner unless the creator says
 * who it is for; a creator who cannot park keys starts on the first member they
 * can create one for.
 */
export function newKeyForm(holders: OrgKeyHolder[]): OrgKeyForm {
  const holder = holders.find((h) => h.is_owner) ?? holders[0]
  return {
    name: '',
    holderId: holder?.id ?? 0,
    template: '',
    picksModels: false,
    models: [],
    unlimited: false,
    quota: 10,
    expires: undefined,
    rpm: 0,
    tpm: 0,
    monthly: 0,
  }
}

/** The form of an existing key. */
export function keyFormOf(key: OrgKey): OrgKeyForm {
  return {
    name: key.name,
    holderId: key.holder_id,
    template: key.policy_template,
    picksModels: handPickedModels(key).length > 0,
    models: handPickedModels(key),
    unlimited: key.unlimited_quota,
    quota: quotaUnitsToDollars(key.remain_quota),
    expires:
      key.expired_time > 0 ? new Date(key.expired_time * 1000) : undefined,
    rpm: key.rpm_limit,
    tpm: key.tpm_limit,
    monthly: key.monthly_limit,
  }
}

/** A form's expiry the way the backend takes it: Unix seconds, or -1 for never. */
function expiryOf(form: OrgKeyForm): number {
  return form.expires ? Math.floor(form.expires.getTime() / 1000) : -1
}

/**
 * Whether a form limits its key to models picked by hand. Choosing a template
 * takes precedence: the picks are kept in the form in case the template is
 * taken off again, but they are not asked for.
 */
export function isPickingModels(form: OrgKeyForm): boolean {
  return !form.template && form.picksModels
}

/** The hand-picked list a form asks for; none unless it is picking models. */
function chosenModels(form: OrgKeyForm): string[] {
  return isPickingModels(form) ? form.models : []
}

/** Whether two lists name the same models; neither names one twice. */
function sameModels(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((name) => b.includes(name))
}

/** What a filled-in form asks the backend to create. */
export function keyInput(form: OrgKeyForm): OrgKeyInput {
  return {
    name: form.name.trim(),
    holder_id: form.holderId,
    policy_template: form.template,
    model_limits: chosenModels(form),
    remain_quota: form.unlimited ? 0 : parseQuotaFromDollars(form.quota),
    unlimited_quota: form.unlimited,
    expired_time: expiryOf(form),
    rpm_limit: form.rpm,
    tpm_limit: form.tpm,
    monthly_limit: form.monthly,
  }
}

/**
 * The change a form makes to an existing key: only the fields that were
 * actually edited. That matters most for the quota — the key is spending it
 * while the form is open, and sending back the number the form started with
 * would undo that spending.
 *
 * `reapplyTemplate` asks for the template to be applied afresh although it was
 * not changed, which is how a key picks up models added to the catalogue since.
 *
 * The template and the hand-picked list are sent each only when it changed.
 * The backend replaces the two together and reads the one left out as empty,
 * which is exactly what an unchanged one is whenever the other changes.
 */
export function keyPatch(
  key: OrgKey,
  form: OrgKeyForm,
  reapplyTemplate: boolean
): OrgKeyPatch {
  const before = keyFormOf(key)
  const patch: OrgKeyPatch = {}
  if (form.name.trim() !== before.name) patch.name = form.name.trim()
  if (form.unlimited !== before.unlimited) {
    patch.unlimited_quota = form.unlimited
  }
  // A key that goes from unlimited to limited needs its quota said, whatever
  // number the field happened to hold.
  const nowLimited = !form.unlimited && before.unlimited
  if (!form.unlimited && (form.quota !== before.quota || nowLimited)) {
    patch.remain_quota = parseQuotaFromDollars(form.quota)
  }
  if (expiryOf(form) !== expiryOf(before)) patch.expired_time = expiryOf(form)
  if (form.rpm !== before.rpm) patch.rpm_limit = form.rpm
  if (form.tpm !== before.tpm) patch.tpm_limit = form.tpm
  if (form.monthly !== before.monthly) patch.monthly_limit = form.monthly
  if (form.template !== before.template || reapplyTemplate) {
    patch.policy_template = form.template
  }
  if (!sameModels(chosenModels(form), chosenModels(before))) {
    patch.model_limits = chosenModels(form)
  }
  return patch
}
