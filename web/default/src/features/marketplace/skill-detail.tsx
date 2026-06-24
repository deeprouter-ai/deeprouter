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
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { ArrowLeft, PackageOpen } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  downloadSkillPackage,
  getMarketplaceSkill,
  recordMarketplaceSkillEvent,
} from './api'
import { KidsBadge, PlanBadge } from './components/badges'
import { ErrorBanner } from './components/error-banner'
import { LockState } from './components/lock-state'
import { normalizeLockState } from './components/lock-state-utils'
import { SkillCTA } from './components/skill-cta'
import type { MarketplaceSkill, SkillCTAAction } from './types'

interface SkillDetailProps {
  slug: string
}

function getSkillCTA(skill: MarketplaceSkill): SkillCTAAction {
  const action = skill.availability?.cta
  if (
    action === 'download' ||
    action === 'enable' ||
    action === 'use' ||
    action === 'upgrade' ||
    action === 'renew' ||
    action === 'contact_sales' ||
    action === 'login' ||
    action === 'unavailable'
  ) {
    return action
  }
  if (skill.status === 'deprecated') return 'unavailable'
  return 'view'
}

export function SkillDetail({ slug }: SkillDetailProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [downloading, setDownloading] = useState(false)

  const skillQuery = useQuery({
    queryKey: ['marketplace-skill', slug],
    queryFn: () => getMarketplaceSkill(slug),
  })

  const skill = skillQuery.data
  const errorMessage =
    (
      skillQuery.error as {
        response?: { data?: { error?: { message?: string } } }
        message?: string
      }
    )?.response?.data?.error?.message ??
    (skillQuery.error as Error | null)?.message

  async function handleCTA() {
    if (!skill) return
    const action = getSkillCTA(skill)
    const skillId = skill.slug || skill.id

    void recordMarketplaceSkillEvent(skillId, {
      event_type: 'skill_detail_view',
      entry_point: 'skill_detail',
    }).catch(() => undefined)

    if (action === 'download' || action === 'enable' || action === 'use') {
      setDownloading(true)
      try {
        await downloadSkillPackage(skillId, 'skill_detail')
      } catch (err) {
        const apiErr = err as {
          response?: { data?: { error?: { code?: string; message?: string } } }
          message?: string
        }
        const code = apiErr?.response?.data?.error?.code
        const msg =
          apiErr?.response?.data?.error?.message ??
          (err as Error | null)?.message ??
          t('Download failed.')
        if (code === 'SKILL_PLAN_REQUIRED') {
          toast.error(t('A higher plan is required to download this skill.'))
        } else if (code === 'SKILL_AUTH_REQUIRED' || code === 'AUTH_REQUIRED') {
          toast.error(t('Please sign in to download skills.'))
          void navigate({ to: '/sign-in' })
        } else {
          toast.error(msg)
        }
      } finally {
        setDownloading(false)
      }
      return
    }

    if (action === 'upgrade' || action === 'renew') {
      void navigate({ to: '/subscriptions' })
      return
    }

    if (action === 'login') {
      void navigate({ to: '/sign-in' })
      return
    }

    if (action === 'contact_sales') {
      void navigate({ to: '/help/faq' })
      return
    }
  }

  if (skillQuery.isLoading) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <Skeleton className='h-8 w-48' />
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <Card className='max-w-2xl'>
            <CardHeader>
              <Skeleton className='h-6 w-3/4' />
            </CardHeader>
            <CardContent className='space-y-3'>
              <Skeleton className='h-4 w-full' />
              <Skeleton className='h-4 w-5/6' />
              <Skeleton className='h-4 w-2/3' />
            </CardContent>
          </Card>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  if (skillQuery.isError || !skill) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Skill Detail')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='flex flex-col gap-4'>
            <Button
              variant='ghost'
              size='sm'
              className='w-fit'
              onClick={() => void navigate({ to: '/skills' })}
            >
              <ArrowLeft className='mr-2 size-4' />
              {t('Back to Marketplace')}
            </Button>
            <ErrorBanner
              message={errorMessage ?? t('Skill not found.')}
              retryable
              onRetry={() => void skillQuery.refetch()}
            />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  const action = getSkillCTA(skill)
  const lockState = normalizeLockState(skill.availability?.lock_code)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{skill.name}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {skill.short_description}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <Button
            variant='ghost'
            size='sm'
            className='w-fit'
            onClick={() => void navigate({ to: '/skills' })}
          >
            <ArrowLeft className='mr-2 size-4' />
            {t('Back to Marketplace')}
          </Button>

          <Card className='max-w-2xl'>
            <CardHeader>
              <div className='flex items-start gap-3'>
                <div className='bg-muted text-muted-foreground flex size-10 shrink-0 items-center justify-center rounded-lg'>
                  <PackageOpen className='size-5' aria-hidden='true' />
                </div>
                <div className='min-w-0'>
                  <CardTitle>{skill.name}</CardTitle>
                  <p className='text-muted-foreground mt-0.5 text-sm'>
                    {skill.category}
                  </p>
                </div>
              </div>
            </CardHeader>
            <CardContent className='space-y-4'>
              {skill.description && (
                <p className='text-muted-foreground text-sm leading-6'>
                  {skill.description}
                </p>
              )}
              <div className='flex flex-wrap gap-2'>
                <PlanBadge plan={skill.required_plan} />
                {skill.is_kids_safe && <KidsBadge state='kids_safe' />}
                {skill.is_kids_exclusive && (
                  <KidsBadge state='kids_exclusive' />
                )}
              </div>
              {lockState && <LockState state={lockState} />}
            </CardContent>
            <CardFooter>
              <SkillCTA
                action={action}
                disabled={downloading}
                onClick={() => void handleCTA()}
              />
            </CardFooter>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
