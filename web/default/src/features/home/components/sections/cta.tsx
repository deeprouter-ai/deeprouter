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
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { AnimateInView } from '@/components/animate-in-view'

interface CTAProps {
  className?: string
  isAuthenticated?: boolean
}

export function CTA(props: CTAProps) {
  const { t } = useTranslation()

  if (props.isAuthenticated) {
    return null
  }

  return (
    <section className='relative z-10 overflow-hidden px-6 py-24 md:py-32'>
      <AnimateInView
        className='border-border bg-card/75 mx-auto max-w-3xl rounded-2xl border px-6 py-12 text-center shadow-[0_16px_44px_rgb(28_28_28/0.08)] md:px-12 md:py-14'
        animation='scale-in'
      >
        <h2 className='text-3xl leading-tight font-bold tracking-normal md:text-5xl'>
          {t('Start using every model.')}
          <br />
          <span className='text-accent'>{t('With one account.')}</span>
        </h2>
        <p className='text-muted-foreground mx-auto mt-5 max-w-md text-sm leading-relaxed md:text-base'>
          {t(
            'No API keys to manage, no foreign cards required, no engineering team needed. Top up in your currency and start routing today.'
          )}
        </p>
        <div className='mt-8 flex items-center justify-center gap-3'>
          <Button className='group' render={<Link to='/sign-up' />}>
            {t('Get Started')}
            <ArrowRight className='ml-1 size-3.5 transition-transform duration-200 group-hover:translate-x-0.5' />
          </Button>
          <Button variant='outline' render={<Link to='/pricing' />}>
            {t('View Pricing')}
          </Button>
        </div>
      </AnimateInView>
    </section>
  )
}
