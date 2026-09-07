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
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Package } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { PageTransition } from '@/components/page-transition'
import { SearchBar } from '@/features/pricing/components/search-bar'
import { fetchMarketplaceSkills, marketplaceQueryKeys } from './api'
import { SkillCard } from './components/skill-card'

// PRD §5.1 initial category list. `category` is free text server-side, so an
// admin-invented category still reaches users through search / All.
const CATEGORIES: { value: string; labelKey: string }[] = [
  { value: 'writing', labelKey: 'Writing' },
  { value: 'translation', labelKey: 'Translation' },
  { value: 'code', labelKey: 'Code' },
  { value: 'data-analysis', labelKey: 'Data Analysis' },
  { value: 'research', labelKey: 'Research' },
  { value: 'legal', labelKey: 'Legal' },
  { value: 'finance', labelKey: 'Finance' },
]

// The catalog is admin-curated and small, so one page covers it; 100 is the
// backend's maximum page size.
const PAGE_LIMIT = 100

export function MarketplacePage() {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [category, setCategory] = useState<string>('all')

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search.trim()), 300)
    return () => clearTimeout(timer)
  }, [search])

  const params = {
    q: debouncedSearch || undefined,
    category: category === 'all' ? undefined : category,
    limit: PAGE_LIMIT,
  }
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: marketplaceQueryKeys.list(params),
    queryFn: () => fetchMarketplaceSkills(params),
  })

  const filtered = Boolean(debouncedSearch) || category !== 'all'
  const skills = data?.skills ?? []
  // PRD §8.1: featured cards on top, rank ascending, at most 4 — only on the
  // unfiltered view. The backend already sorts featured-first, so the first
  // items are the featured ones.
  const featured = filtered ? [] : skills.filter((s) => s.featured_flag).slice(0, 4)
  const rest = skills.filter((s) => !featured.includes(s))

  return (
    <PublicLayout showMainContainer={false}>
      <div className='bg-background min-h-dvh'>
        <PageTransition className='mx-auto w-full max-w-7xl px-4 pt-20 pb-12 md:px-6'>
          <h1 className='text-2xl font-semibold sm:text-3xl'>
            {t('Skill Marketplace')}
          </h1>
          <p className='text-muted-foreground mt-2 max-w-2xl'>
            {t(
              'Hand-tested skills for Claude Code. Download one, drop it into your setup, and it runs on your DeepRouter key.'
            )}
          </p>

          <div className='mt-6 flex flex-col gap-3 sm:flex-row sm:items-center'>
            <SearchBar
              value={search}
              onChange={setSearch}
              onClear={() => setSearch('')}
              placeholder={t('Search skills...')}
              className='sm:max-w-xs'
            />
            <div className='flex flex-wrap gap-1.5'>
              {[{ value: 'all', labelKey: 'All' }, ...CATEGORIES].map((c) => (
                <button
                  key={c.value}
                  type='button'
                  onClick={() => setCategory(c.value)}
                  className={cn(
                    'rounded-[7px] border px-3 py-1.5 text-sm transition-colors',
                    category === c.value
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'border-border bg-card text-muted-foreground hover:text-foreground'
                  )}
                >
                  {t(c.labelKey)}
                </button>
              ))}
            </div>
          </div>

          {isLoading ? (
            <div className='mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
              {Array.from({ length: 8 }).map((_, i) => (
                <Skeleton key={i} className='h-36 rounded-xl' />
              ))}
            </div>
          ) : isError ? (
            <div className='mt-8'>
              <ErrorState
                title={t('Could not load the marketplace')}
                description={t('Please check your connection and try again.')}
                onRetry={() => refetch()}
              />
            </div>
          ) : skills.length === 0 ? (
            <div className='mt-8'>
              <EmptyState
                icon={Package}
                title={
                  filtered
                    ? t('No skills match your search')
                    : t('No skills available yet, stay tuned')
                }
                description={
                  filtered
                    ? t('Try a different keyword or category.')
                    : undefined
                }
                bordered
              />
            </div>
          ) : (
            <>
              {featured.length > 0 && (
                <section className='mt-8'>
                  <h2 className='text-lg font-semibold'>{t('Featured')}</h2>
                  <div className='mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
                    {featured.map((skill) => (
                      <SkillCard key={skill.id} skill={skill} />
                    ))}
                  </div>
                </section>
              )}
              <section className='mt-8'>
                {featured.length > 0 && (
                  <h2 className='text-lg font-semibold'>{t('All skills')}</h2>
                )}
                <div className='mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
                  {rest.map((skill) => (
                    <SkillCard key={skill.id} skill={skill} />
                  ))}
                </div>
              </section>
            </>
          )}
        </PageTransition>
      </div>
      <Footer />
    </PublicLayout>
  )
}
