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
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  ArrowRight,
  Check,
  Code,
  Copy,
  Gift,
  KeyRound,
  MessageSquare,
  PlayCircle,
  Sparkles,
  Terminal,
  Users,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { takeWelcomeHandoff } from '@/features/auth/lib/storage'
import type { RegisterResponseData } from '@/features/auth/types'
import { updateUserSettings } from '@/features/profile/api'
import { PERSONA_PRESETS } from '@/features/profile/lib/persona-presets'
import type { Persona, UserSettings } from '@/features/profile/types'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

// Step 2 — persona (3 cards, reuses PersonaPickerDialog content shape)
const PERSONAS: Array<{
  id: Persona
  icon: LucideIcon
  titleKey: string
  descKey: string
  badge?: string
}> = [
  {
    id: 'casual',
    icon: MessageSquare,
    titleKey: 'Casual',
    // Verbose-by-default because /welcome runs BEFORE persona is set,
    // so FieldHint can't detect casual. Verbose for everyone is fine
    // here — this screen is shown exactly once per account.
    descKey:
      'For chatting, writing, translation, image generation. No code needed — you use a desktop app like Cherry Studio.',
  },
  {
    id: 'dev',
    icon: Terminal,
    titleKey: 'Developer',
    descKey:
      'For coding and integrating the API into your own scripts or products. You will write code or use tools like Cursor.',
    badge: 'Most users',
  },
  {
    id: 'team',
    icon: Users,
    titleKey: 'Team / Enterprise',
    descKey:
      'For shared API keys, audit trails, and team integration. UI is the same as Developer for now; team-only tools coming soon.',
  },
]

// Step 3 — brand preference (1-click chips)
type BrandId = 'claude' | 'openai' | 'gemini' | 'deepseek' | ''
const BRANDS: Array<{ id: BrandId; label: string }> = [
  { id: 'claude', label: 'Claude' },
  { id: 'openai', label: 'OpenAI' },
  { id: 'gemini', label: 'Gemini' },
  { id: 'deepseek', label: 'DeepSeek' },
  { id: '', label: 'No preference' },
]

// Step 4 — preferred client / landing
type ClientId =
  | 'cherry-studio' | 'chatbox' | 'lobechat'
  | 'cursor' | 'claude-code' | 'code'
  | 'playground' | 'dashboard'

const CLIENTS: Array<{
  id: ClientId
  icon: LucideIcon
  titleKey: string
  descKey: string
  recommendedFor: Persona[]
}> = [
  {
    id: 'cherry-studio',
    icon: Sparkles,
    titleKey: 'Cherry Studio',
    descKey:
      'Free desktop app. Double-click to install. Best place to start if you don\'t code.',
    recommendedFor: ['casual'],
  },
  {
    id: 'chatbox',
    icon: MessageSquare,
    titleKey: 'Chatbox',
    descKey:
      'Web + desktop + mobile. Works in browser, no install needed.',
    recommendedFor: [],
  },
  {
    id: 'lobechat',
    icon: MessageSquare,
    titleKey: 'LobeChat',
    descKey: 'Open-source chat client with plugins for advanced use.',
    recommendedFor: [],
  },
  {
    id: 'cursor',
    icon: Code,
    titleKey: 'Cursor',
    descKey:
      'AI-powered code editor. Replaces VS Code for AI pair programming.',
    recommendedFor: ['dev', 'team'],
  },
  {
    id: 'claude-code',
    icon: Terminal,
    titleKey: 'Claude Code',
    descKey:
      "Anthropic's command-line AI assistant. You'll need a terminal.",
    recommendedFor: ['dev'],
  },
  {
    id: 'code',
    icon: Code,
    titleKey: 'Python / Node',
    descKey:
      'Call the API directly from your own Python or Node.js code.',
    recommendedFor: ['dev'],
  },
  {
    id: 'playground',
    icon: PlayCircle,
    titleKey: 'Try in browser',
    descKey:
      'Zero setup — chat with the AI right here on this site. Best to test quickly.',
    recommendedFor: ['casual'],
  },
  {
    id: 'dashboard',
    icon: KeyRound,
    titleKey: 'Just look around',
    descKey:
      'Skip the guide and explore the dashboard yourself. You can come back later.',
    recommendedFor: ['team'],
  },
]

const QUOTA_PER_USD = 500_000
const AVG_CHAT_COST_USD = 0.005

function estimateChats(quota: number): number {
  if (!quota || !Number.isFinite(quota) || quota <= 0) return 0
  return Math.max(0, Math.floor(quota / QUOTA_PER_USD / AVG_CHAT_COST_USD))
}

function formatChatsCount(n: number): string {
  if (n < 1000) return String(n)
  if (n < 10_000) return `${(n / 1000).toFixed(1).replace(/\.0$/, '')}k`
  return `${Math.floor(n / 1000)}k`
}

export function Welcome() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.auth.user)
  const setUser = useAuthStore((s) => s.auth.setUser)

  // Read the one-shot register handoff exactly once on mount. After
  // this, the default token is gone — refresh erases it (intentional).
  const [handoff] = useState<RegisterResponseData | null>(() =>
    takeWelcomeHandoff<RegisterResponseData>()
  )

  // Unauthenticated visitors get bounced to /sign-in. Otherwise stay on
  // the wizard even without a handoff — covers:
  //   * Old backend (didn't return data) — user is logged in, just no
  //     default-token banner
  //   * OAuth signups landing here via PersonaPickerHost redirect
  //   * Privacy-mode browsers where sessionStorage write failed
  //   * User navigating to /welcome on their own to redo the picker
  // The default-token banner is conditionally rendered against `handoff`.
  useEffect(() => {
    if (!user) {
      navigate({ to: '/sign-in', replace: true })
    }
  }, [user, navigate])

  const [persona, setPersona] = useState<Persona | null>(null)
  const [brand, setBrand] = useState<BrandId>('')
  const [client, setClient] = useState<ClientId | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // Pre-suggest a sensible client based on persona pick — but user can
  // override any time before final submit.
  useEffect(() => {
    if (!persona || client) return
    const suggested = CLIENTS.find((c) =>
      c.recommendedFor.includes(persona)
    )
    if (suggested) setClient(suggested.id)
  }, [persona, client])

  const handleFinish = async (
    landingOverride?: ClientId,
    skipAll = false
  ) => {
    if (submitting) return
    setSubmitting(true)
    try {
      const finalClient = landingOverride ?? client ?? 'dashboard'
      const finalPersona = persona ?? 'dev' // legacy-safe default
      const finalBrand = brand
      const settingPatch: Partial<UserSettings> = {
        persona: finalPersona,
        brand_preference: finalBrand as UserSettings['brand_preference'],
        preferred_client: finalClient,
      }
      // Snap the right sidebar preset for the chosen persona so the
      // dashboard's sidebar config takes effect on next page load.
      const preset = PERSONA_PRESETS[finalPersona]
      if (preset?.sidebarModules) {
        ;(settingPatch as { sidebar_modules?: string }).sidebar_modules =
          preset.sidebarModules
      }
      const res = await updateUserSettings(settingPatch)
      if (!res?.success && !skipAll) {
        toast.error(res?.message || t('Could not save your selection.'))
        return
      }
      // Sync the auth store so anything that reads persona (sidebar,
      // mode-picker default) sees the fresh value without a refresh.
      if (user) {
        const rawSetting = user.setting
        const currentSetting: UserSettings =
          typeof rawSetting === 'string'
            ? (JSON.parse(rawSetting) as UserSettings)
            : ((rawSetting as UserSettings | undefined) ?? {})
        setUser({
          ...user,
          setting: {
            ...currentSetting,
            ...settingPatch,
          } as unknown as Record<string, unknown>,
          sidebar_modules:
            (settingPatch as { sidebar_modules?: string }).sidebar_modules ??
            user.sidebar_modules,
        })
      }

      // Decide where to land. Priority:
      //   1. backend's hint from register response (handoff.next)
      //   2. matching /onboarding/<client> route
      //   3. /playground or /dashboard
      let target: string = '/dashboard/overview'
      if (handoff?.next) {
        target = handoff.next
      } else if (
        finalClient === 'cherry-studio' ||
        finalClient === 'chatbox' ||
        finalClient === 'lobechat' ||
        finalClient === 'cursor' ||
        finalClient === 'claude-code' ||
        finalClient === 'code'
      ) {
        target = `/onboarding/${finalClient}`
      } else if (finalClient === 'playground') {
        target = '/playground'
      } else {
        target = preset?.defaultRoute ?? '/dashboard/overview'
      }
      navigate({ to: target as never, replace: true })
    } catch {
      toast.error(t('Could not save your selection.'))
    } finally {
      setSubmitting(false)
    }
  }

  if (!user) return null

  return (
    <div className='mx-auto max-w-3xl px-4 py-6 sm:py-10'>
      {/* Welcome banner — surfaces the 3 things the user just got */}
      <div className='bg-card mb-6 rounded-2xl border p-5 sm:p-6'>
        <div className='flex items-center gap-2 text-sm'>
          <span className='text-xl'>👋</span>
          <span className='text-foreground font-medium'>
            {t('Welcome to DeepRouter, {{name}}', {
              name: handoff?.display_name || user?.username || '',
            })}
          </span>
        </div>
        <div className='mt-4 grid gap-3 sm:grid-cols-2'>
          <WelcomeCard
            icon={Gift}
            label={t('Trial credit')}
            value={
              handoff?.trial_quota
                ? `¥${(handoff.trial_quota / 500000).toFixed(2)}`
                : '—'
            }
            sub={
              handoff?.trial_quota
                ? t('≈ {{count}} chats free to try', {
                    count: formatChatsCount(estimateChats(handoff.trial_quota)),
                  })
                : ''
            }
          />
          {handoff?.default_token ? (
            <CopyCard
              icon={KeyRound}
              label={t('Your default API key (shown once)')}
              value={handoff.default_token}
            />
          ) : (
            <WelcomeCard
              icon={KeyRound}
              label={t('Default API key')}
              value={t('Available in /keys')}
              sub=''
            />
          )}
        </div>
      </div>

      <Stepper currentStep={persona ? (client ? 3 : 2) : 1} totalSteps={3} />

      {/* Step 2 — Persona */}
      <Section title={t('How do you plan to use DeepRouter?')} stepLabel={t('Step 1 of 3')}>
        <div className='grid gap-3 sm:grid-cols-3'>
          {PERSONAS.map((p) => (
            <ChoiceCard
              key={p.id}
              icon={<p.icon className='h-5 w-5' />}
              title={t(p.titleKey)}
              description={t(p.descKey)}
              badge={p.badge ? t(p.badge) : undefined}
              selected={persona === p.id}
              onClick={() => setPersona(p.id)}
            />
          ))}
        </div>
      </Section>

      {/* Step 3 — Brand */}
      {persona && (
        <Section
          title={t('Pick your favourite AI provider (optional)')}
          subtitle={t(
            'Your default key will route to this brand when you call model: "deeprouter".'
          )}
          stepLabel={t('Step 2 of 3')}
        >
          <div className='flex flex-wrap gap-2'>
            {BRANDS.map((b) => (
              <Button
                key={b.id || 'none'}
                type='button'
                variant={brand === b.id ? 'default' : 'outline'}
                size='sm'
                onClick={() => setBrand(b.id)}
                className='rounded-full text-xs'
              >
                {t(b.label)}
              </Button>
            ))}
          </div>
        </Section>
      )}

      {/* Step 4 — Landing */}
      {persona && (
        <Section
          title={t('Where would you like to start?')}
          stepLabel={t('Step 3 of 3')}
        >
          <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
            {CLIENTS.map((c) => {
              const recommended = c.recommendedFor.includes(persona)
              return (
                <ChoiceCard
                  key={c.id}
                  icon={<c.icon className='h-5 w-5' />}
                  title={t(c.titleKey)}
                  description={t(c.descKey)}
                  badge={recommended ? t('Recommended') : undefined}
                  selected={client === c.id}
                  onClick={() => setClient(c.id)}
                />
              )
            })}
          </div>
        </Section>
      )}

      <div className='mt-6 flex items-center justify-between gap-2'>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          disabled={submitting}
          onClick={() => handleFinish('dashboard', true)}
        >
          {t('Skip — set this later')}
        </Button>
        <Button
          type='button'
          disabled={!persona || !client || submitting}
          onClick={() => handleFinish()}
        >
          {submitting ? t('Saving...') : t('Continue')}
          <ArrowRight className='ml-1.5 h-4 w-4' />
        </Button>
      </div>
    </div>
  )
}

function Section({
  title,
  subtitle,
  stepLabel,
  children,
}: {
  title: string
  subtitle?: string
  stepLabel: string
  children: React.ReactNode
}) {
  return (
    <section className='bg-card mb-4 rounded-xl border p-4 sm:p-5'>
      <div className='mb-3 flex items-baseline justify-between gap-2'>
        <h3 className='text-sm font-semibold sm:text-base'>{title}</h3>
        <span className='text-muted-foreground text-[11px] font-medium'>
          {stepLabel}
        </span>
      </div>
      {subtitle && (
        <p className='text-muted-foreground mb-3 text-xs sm:text-sm'>
          {subtitle}
        </p>
      )}
      {children}
    </section>
  )
}

function ChoiceCard({
  icon,
  title,
  description,
  badge,
  selected,
  onClick,
}: {
  icon: React.ReactNode
  title: string
  description: string
  badge?: string
  selected: boolean
  onClick: () => void
}) {
  return (
    <button
      type='button'
      onClick={onClick}
      className={cn(
        'group relative flex h-full flex-col items-start gap-2 rounded-lg border bg-background p-3 text-left transition-all hover:shadow-sm',
        selected
          ? 'border-foreground ring-foreground/15 ring-2'
          : 'border-border hover:border-foreground/40'
      )}
      aria-pressed={selected}
    >
      {selected && (
        <span className='bg-foreground text-background absolute end-2 top-2 inline-flex h-5 w-5 items-center justify-center rounded-full'>
          <Check className='h-3 w-3' />
        </span>
      )}
      <span className='bg-muted text-muted-foreground flex h-9 w-9 shrink-0 items-center justify-center rounded-md border'>
        {icon}
      </span>
      <span className='flex items-center gap-1.5'>
        <span className='text-sm font-medium'>{title}</span>
        {badge && (
          <span className='bg-foreground/10 text-foreground rounded-full px-1.5 py-0.5 text-[10px] font-medium'>
            {badge}
          </span>
        )}
      </span>
      <span className='text-muted-foreground line-clamp-2 text-xs leading-snug'>
        {description}
      </span>
    </button>
  )
}

function WelcomeCard({
  icon: Icon,
  label,
  value,
  sub,
}: {
  icon: LucideIcon
  label: string
  value: string
  sub: string
}) {
  return (
    <div className='bg-background flex items-start gap-3 rounded-lg border p-3'>
      <span className='bg-muted text-muted-foreground flex h-10 w-10 shrink-0 items-center justify-center rounded-md border'>
        <Icon className='h-5 w-5' />
      </span>
      <div className='min-w-0 flex-1'>
        <p className='text-muted-foreground text-[11px] font-medium'>{label}</p>
        <p className='font-mono text-base font-semibold'>{value}</p>
        {sub && (
          <p className='text-muted-foreground/80 text-[11px]'>{sub}</p>
        )}
      </div>
    </div>
  )
}

function CopyCard({
  icon: Icon,
  label,
  value,
}: {
  icon: LucideIcon
  label: string
  value: string
}) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Copy failed'))
    }
  }
  return (
    <div className='bg-background flex items-start gap-3 rounded-lg border p-3'>
      <span className='bg-muted text-muted-foreground flex h-10 w-10 shrink-0 items-center justify-center rounded-md border'>
        <Icon className='h-5 w-5' />
      </span>
      <div className='min-w-0 flex-1'>
        <p className='text-muted-foreground text-[11px] font-medium'>{label}</p>
        <div className='mt-0.5 flex items-center gap-1'>
          <code className='flex-1 truncate font-mono text-xs' title={value}>
            {value}
          </code>
          <Button
            type='button'
            size='sm'
            variant='ghost'
            onClick={handleCopy}
            className='h-6 px-1.5'
          >
            {copied ? (
              <Check className='h-3 w-3' />
            ) : (
              <Copy className='h-3 w-3' />
            )}
          </Button>
        </div>
        <p className='text-amber-600 mt-0.5 text-[10px] dark:text-amber-400'>
          {t("Copy now — won't be shown again after navigating away.")}
        </p>
      </div>
    </div>
  )
}

function Stepper({
  currentStep,
  totalSteps,
}: {
  currentStep: number
  totalSteps: number
}) {
  const items = useMemo(
    () => Array.from({ length: totalSteps }, (_, i) => i + 1),
    [totalSteps]
  )
  return (
    <div className='mb-4 flex items-center gap-2'>
      {items.map((n) => (
        <div
          key={n}
          className={cn(
            'h-1 flex-1 rounded-full transition-colors',
            n <= currentStep ? 'bg-foreground' : 'bg-muted'
          )}
        />
      ))}
    </div>
  )
}
