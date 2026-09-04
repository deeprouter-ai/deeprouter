/*
Copyright (C) 2026 DeepRouter

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
*/
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import {
  AlertTriangle,
  ArrowLeft,
  Check,
  Download,
  KeyRound,
  Package,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { PageTransition } from '@/components/page-transition'
import {
  fetchMarketplaceSkill,
  fetchMyPurchases,
  fetchMySkills,
  marketplaceQueryKeys,
} from './api'
import { BuyConfirmDialog } from './components/buy-confirm-dialog'
import { skillPriceLabel } from './lib/price'
import { useSkillDownload } from './hooks/use-skill-download'

function isNotFound(error: unknown): boolean {
  return (
    (error as { response?: { status?: number } })?.response?.status === 404
  )
}

export function SkillDetailPage({ slug }: { slug: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const currentHref = useRouterState({ select: (s) => s.location.href })
  const { auth } = useAuthStore()
  const isAuthed = !!auth.user

  const detailQuery = useQuery({
    queryKey: marketplaceQueryKeys.detail(slug),
    queryFn: () => fetchMarketplaceSkill(slug),
    retry: false,
  })
  // The two personal lists power the Downloaded badge and the
  // buy-vs-download branch; the catalog is admin-curated and small, so one
  // max-size page covers them.
  const mySkillsQuery = useQuery({
    queryKey: marketplaceQueryKeys.mySkills(),
    queryFn: () => fetchMySkills({ limit: 100 }),
    enabled: isAuthed,
  })
  const myPurchasesQuery = useQuery({
    queryKey: marketplaceQueryKeys.myPurchases(),
    queryFn: () => fetchMyPurchases({ limit: 100 }),
    enabled: isAuthed,
  })

  const { download, pendingSlug } = useSkillDownload()
  const [buyOpen, setBuyOpen] = useState(false)

  const skill = detailQuery.data
  const downloaded =
    mySkillsQuery.data?.skills.some((s) => s.slug === slug) ?? false
  const purchased =
    myPurchasesQuery.data?.purchases.some((p) => p.slug === slug) ?? false
  const deprecated = skill?.status === 'deprecated'
  const paid = skill?.monetization_type === 'paid'
  const downloading = pendingSlug === slug

  // PRD §8.2 button branches: anonymous → sign-in; free or already
  // purchased → straight download; paid & unpurchased → confirm dialog
  // showing the live price.
  const handleAction = () => {
    if (!isAuthed) {
      navigate({ to: '/sign-in', search: { redirect: currentHref } })
      return
    }
    if (!paid || purchased) {
      void download(slug)
      return
    }
    setBuyOpen(true)
  }

  const handleBuyConfirm = async () => {
    const ok = await download(slug)
    if (ok) setBuyOpen(false)
  }

  return (
    <PublicLayout showMainContainer={false}>
      <div className='bg-background min-h-dvh'>
        <PageTransition className='mx-auto w-full max-w-3xl px-4 pt-20 pb-12 md:px-6'>
          <Link
            to='/marketplace'
            className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-sm transition-colors'
          >
            <ArrowLeft className='size-4' />
            {t('Back to marketplace')}
          </Link>

          {detailQuery.isLoading ? (
            <div className='mt-6 space-y-4'>
              <Skeleton className='h-9 w-2/3' />
              <Skeleton className='h-24 w-full' />
              <Skeleton className='h-40 w-full' />
            </div>
          ) : detailQuery.isError ? (
            <div className='mt-6'>
              {isNotFound(detailQuery.error) ? (
                <EmptyState
                  icon={Package}
                  title={t('Skill not found')}
                  description={t(
                    'This skill does not exist or has not been published.'
                  )}
                  action={
                    <Button variant='outline' render={<Link to='/marketplace' />}>
                      {t('Back to marketplace')}
                    </Button>
                  }
                  bordered
                />
              ) : (
                <ErrorState
                  title={t('Could not load this skill')}
                  description={t('Please check your connection and try again.')}
                  onRetry={() => detailQuery.refetch()}
                />
              )}
            </div>
          ) : skill ? (
            <>
              {deprecated && (
                // PRD §8.2: deprecated shows a banner and hides the download
                // button; everything else stays visible for people who
                // already own it.
                <div className='border-warning/40 bg-warning/10 text-foreground mt-6 flex items-start gap-2.5 rounded-xl border p-3.5 text-sm'>
                  <AlertTriangle className='text-warning mt-0.5 size-4 shrink-0' />
                  {t('This skill has been delisted.')}
                </div>
              )}

              <div className='mt-6 flex flex-wrap items-center gap-2'>
                <h1 className='text-2xl font-semibold sm:text-3xl'>
                  {skill.name}
                </h1>
                {downloaded && (
                  <Badge variant='secondary' className='gap-1'>
                    <Check className='size-3' />
                    {t('Downloaded')}
                  </Badge>
                )}
              </div>

              <div className='mt-3 flex flex-wrap items-center gap-2'>
                <Badge variant='secondary'>{skill.category}</Badge>
                <Badge variant='outline' className='tabular-nums'>
                  {skillPriceLabel(skill, t('Free'))}
                </Badge>
                {skill.version && (
                  <Badge variant='ghost' className='tabular-nums'>
                    v{skill.version}
                  </Badge>
                )}
                {skill.tags.map((tag) => (
                  <Badge key={tag} variant='ghost'>
                    {tag}
                  </Badge>
                ))}
              </div>

              <p className='text-muted-foreground mt-4 whitespace-pre-line'>
                {skill.description}
              </p>

              {/* PRD §8.2: the API-key requirement is always shown. */}
              <div className='border-border bg-card mt-6 flex items-start gap-2.5 rounded-xl border p-3.5 text-sm'>
                <KeyRound className='text-accent mt-0.5 size-4 shrink-0' />
                <span>
                  {t(
                    'Running this skill needs a DeepRouter API Key — it calls models through your DeepRouter account.'
                  )}{' '}
                  <Link
                    to='/keys'
                    className='text-accent hover:underline'
                  >
                    {t('Get your key')}
                  </Link>
                </span>
              </div>

              {skill.changelog && (
                <section className='mt-6'>
                  <h2 className='text-lg font-semibold'>
                    {t('What changed in v{{version}}', {
                      version: skill.version,
                    })}
                  </h2>
                  <p className='text-muted-foreground mt-2 text-sm whitespace-pre-line'>
                    {skill.changelog}
                  </p>
                </section>
              )}

              {!deprecated && (
                <div className='mt-8'>
                  <Button
                    size='lg'
                    onClick={handleAction}
                    disabled={downloading}
                  >
                    <Download className='size-4' />
                    {!isAuthed
                      ? t('Sign in to download')
                      : paid && !purchased
                        ? t('Buy for {{price}}', {
                            price: `$${skill.price_usd.toFixed(2)}`,
                          })
                        : t('Download')}
                  </Button>
                </div>
              )}

              <BuyConfirmDialog
                slug={slug}
                open={buyOpen}
                onOpenChange={setBuyOpen}
                onConfirm={() => void handleBuyConfirm()}
                confirming={downloading}
              />
            </>
          ) : null}
        </PageTransition>
      </div>
      <Footer />
    </PublicLayout>
  )
}
