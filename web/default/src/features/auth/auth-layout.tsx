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
import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Skeleton } from '@/components/ui/skeleton'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()

  const valueProps = [
    'GPT-5, Claude, Gemini & 20+ models',
    'Pay in your local currency',
    'Drop-in OpenAI-compatible API',
  ]

  return (
    <div className='relative grid min-h-svh lg:grid-cols-[1.05fr_1fr]'>
      {/* Left: brand panel (lg+) */}
      <aside className='bg-muted/40 relative hidden overflow-hidden lg:flex lg:flex-col lg:justify-between lg:p-12 xl:p-16'>
        {/* Animated background blobs */}
        <div
          aria-hidden
          className='pointer-events-none absolute inset-0 -z-10 overflow-hidden'
        >
          <span
            className='auth-blob auth-blob-a bg-accent/30 dark:bg-accent/25 absolute -top-24 -left-16 h-[28rem] w-[28rem] opacity-60'
          />
          <span
            className='auth-blob auth-blob-b absolute top-1/3 -right-24 h-[24rem] w-[24rem] bg-emerald-300/30 opacity-55 dark:bg-emerald-400/15'
          />
          <span
            className='auth-blob auth-blob-c absolute -bottom-28 left-1/3 h-[26rem] w-[26rem] bg-violet-300/30 opacity-50 dark:bg-violet-400/15'
          />
        </div>
        <div
          aria-hidden
          className='auth-grid-pulse text-foreground pointer-events-none absolute inset-0 -z-10 [background-image:linear-gradient(to_right,currentColor_1px,transparent_1px),linear-gradient(to_bottom,currentColor_1px,transparent_1px)] [background-size:28px_28px]'
        />

        <Link
          to='/'
          className='landing-animate-fade-up inline-flex w-fit items-center gap-2.5 transition-opacity hover:opacity-80'
          style={{ animationDelay: '0ms' }}
        >
          {loading ? (
            <Skeleton className='h-9 w-9 rounded-full' />
          ) : (
            <img
              src={logo}
              alt={systemName}
              className='h-9 w-9 rounded-full object-cover'
            />
          )}
          {loading ? (
            <Skeleton className='h-5 w-24' />
          ) : (
            <span className='text-base font-semibold tracking-tight'>
              {systemName}
            </span>
          )}
        </Link>

        <div className='space-y-10'>
          <div className='space-y-4'>
            <h2
              className='landing-animate-fade-up text-[2.75rem] leading-[1.05] font-bold tracking-tight'
              style={{ animationDelay: '80ms' }}
            >
              {t('One account.')}
              <br />
              <span className='text-accent'>{t('Every AI model.')}</span>
            </h2>
            <p
              className='landing-animate-fade-up text-muted-foreground max-w-md text-base leading-relaxed'
              style={{ animationDelay: '160ms' }}
            >
              {t(
                'Chat with GPT-5, Claude, Gemini and 20+ models from one place — no API keys, no foreign cards, no engineering.'
              )}
            </p>
          </div>

          <ul className='space-y-3 text-sm'>
            {valueProps.map((item, i) => (
              <li
                key={item}
                className='landing-animate-fade-up flex items-center gap-3'
                style={{ animationDelay: `${240 + i * 70}ms` }}
              >
                <span className='bg-accent/10 text-accent inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full'>
                  <Check className='h-3 w-3' strokeWidth={3} />
                </span>
                <span className='text-foreground/90'>{t(item)}</span>
              </li>
            ))}
          </ul>

          <div
            className='landing-animate-scale-in bg-card/80 border-border/60 max-w-md overflow-hidden rounded-xl border shadow-[0_8px_24px_rgb(28_28_28/0.05)] backdrop-blur'
            style={{ animationDelay: '480ms' }}
          >
            <div className='border-border/60 flex items-center gap-1.5 border-b px-3.5 py-2'>
              <span className='h-2.5 w-2.5 rounded-full bg-red-400/70' />
              <span className='h-2.5 w-2.5 rounded-full bg-amber-400/70' />
              <span className='h-2.5 w-2.5 rounded-full bg-emerald-400/70' />
              <span className='text-muted-foreground/70 ml-2 font-mono text-[10px]'>
                ~/deeprouter
              </span>
            </div>
            <pre className='overflow-x-auto px-4 py-3.5 font-mono text-[11.5px] leading-relaxed'>
              <code>
                <span className='text-muted-foreground'>$</span> curl
                deeprouter.ai/v1/chat/completions \{'\n'}
                {'    '}-H{' '}
                <span className='text-accent'>
                  "Authorization: Bearer sk-..."
                </span>{' '}
                \{'\n'}
                {'    '}-d{' '}
                <span className='text-accent'>
                  '{'{ "model": "auto", "messages": [...] }'}'
                </span>
                <span className='auth-caret text-accent ml-1 inline-block'>
                  ▋
                </span>
              </code>
            </pre>
          </div>
        </div>

        <p
          className='landing-animate-fade-up text-muted-foreground text-xs'
          style={{ animationDelay: '620ms' }}
        >
          {t('Trusted by developers shipping production AI features.')}
        </p>
      </aside>

      {/* Right: form */}
      <div className='relative flex flex-col px-6 py-8 sm:px-10 sm:py-12 lg:px-14'>
        <Link
          to='/'
          className='inline-flex w-fit items-center gap-2 transition-opacity hover:opacity-80 lg:hidden'
        >
          {loading ? (
            <Skeleton className='h-7 w-7 rounded-full' />
          ) : (
            <img
              src={logo}
              alt={systemName}
              className='h-7 w-7 rounded-full object-cover'
            />
          )}
          {loading ? (
            <Skeleton className='h-5 w-20' />
          ) : (
            <span className='text-sm font-semibold tracking-tight'>
              {systemName}
            </span>
          )}
        </Link>

        <div
          className='landing-animate-fade-up mx-auto flex w-full max-w-md flex-1 flex-col justify-center py-10 lg:py-0'
          style={{ animationDelay: '120ms' }}
        >
          {children}
        </div>
      </div>
    </div>
  )
}
