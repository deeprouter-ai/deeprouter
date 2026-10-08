// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ReactNode } from 'react'
import { Building2, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

/**
 * One thing a member of an organization is told about how their account
 * works: an icon, a line that names it, and a sentence or two that say what
 * it means for them.
 */
export function OrgNotice(props: {
  icon: LucideIcon
  title: string
  children: ReactNode
  className?: string
}) {
  const Icon = props.icon
  return (
    <section
      className={cn(
        'bg-card border-border flex items-start gap-3 rounded-2xl border p-5',
        props.className
      )}
    >
      <span className='bg-muted text-foreground inline-flex size-10 shrink-0 items-center justify-center rounded-xl'>
        <Icon className='size-5' aria-hidden='true' />
      </span>
      <div className='min-w-0'>
        <p className='text-base font-semibold'>{props.title}</p>
        <p className='text-muted-foreground mt-1 text-sm leading-relaxed'>
          {props.children}
        </p>
      </div>
    </section>
  )
}

/**
 * What a member of an organization is shown where a personal account sees its
 * balance and a way to top up (Enterprise Org, meta-repo
 * `docs/enterprise-org-prd.md` §4): their keys spend the company wallet, so
 * there is no balance of theirs to show and nothing for them to pay.
 */
export function CompanyWalletNotice(props: { className?: string }) {
  const { t } = useTranslation()
  return (
    <OrgNotice
      icon={Building2}
      title={t('Paid by your organization')}
      className={props.className}
    >
      {t(
        'The keys assigned to you spend your organization’s balance. There is nothing for you to top up.'
      )}
    </OrgNotice>
  )
}
