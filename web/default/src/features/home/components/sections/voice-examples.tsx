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
import { useEffect, useState } from 'react'
import { Languages, Mic2, Play, Radio, Square, Volume2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

const VOICE_EXAMPLES = [
  {
    title: 'Product demo narration',
    text: 'The first move is what sets everything in motion.',
    meta: 'ElevenLabs-style TTS',
    icon: Volume2,
    tone: 'accent',
    lang: 'en-US',
    rate: 0.95,
    pitch: 0.95,
    voiceHints: ['narrator', 'daniel', 'guy', 'male'],
  },
  {
    title: 'AI tutor voice',
    text: 'Let us slow this down and walk through the idea step by step.',
    meta: 'Warm explanatory tone',
    icon: Mic2,
    tone: 'success',
    lang: 'en-US',
    rate: 0.88,
    pitch: 1.08,
    voiceHints: ['samantha', 'jenny', 'female', 'google us english'],
  },
  {
    title: 'Multilingual announcement',
    text: 'Translate the message, keep the pacing, and generate natural speech.',
    meta: 'EN / 中文 / 日本語',
    icon: Languages,
    tone: 'warning',
    lang: 'zh-CN',
    rate: 0.92,
    pitch: 1,
    voiceHints: ['tingting', 'mei', 'xiaoxiao', 'chinese'],
  },
  {
    title: 'Realtime agent reply',
    text: 'Stream the answer as audio while the assistant is still thinking.',
    meta: 'Low-latency streaming',
    icon: Radio,
    tone: 'accent',
    lang: 'en-US',
    rate: 1.05,
    pitch: 1,
    voiceHints: ['alex', 'google us english', 'english'],
  },
] as const

const WAVE_BARS = [18, 34, 24, 46, 30, 58, 38, 72, 45, 62, 36, 50, 28, 40, 22]

export function VoiceExamples() {
  const { t } = useTranslation()
  const [activeVoice, setActiveVoice] = useState<string | null>(null)
  const [isSpeechSupported, setIsSpeechSupported] = useState(false)

  useEffect(() => {
    setIsSpeechSupported(
      typeof window !== 'undefined' && 'speechSynthesis' in window
    )

    return () => {
      if (typeof window !== 'undefined' && 'speechSynthesis' in window) {
        window.speechSynthesis.cancel()
      }
    }
  }, [])

  const playExample = (example: (typeof VOICE_EXAMPLES)[number]) => {
    if (!isSpeechSupported) return

    if (activeVoice === example.title) {
      window.speechSynthesis.cancel()
      setActiveVoice(null)
      return
    }

    window.speechSynthesis.cancel()

    const speechText = t(example.text)
    const speechLang = /[\u3400-\u9fff]/.test(speechText)
      ? 'zh-CN'
      : example.lang
    const utterance = new SpeechSynthesisUtterance(speechText)
    const voices = window.speechSynthesis.getVoices()
    const matchingVoice = voices.find((voice) => {
      const voiceName = voice.name.toLowerCase()
      const voiceLang = voice.lang.toLowerCase()

      return (
        voiceLang.startsWith(speechLang.toLowerCase().slice(0, 2)) &&
        example.voiceHints.some((hint) =>
          voiceName.includes(hint.toLowerCase())
        )
      )
    })

    utterance.lang = speechLang
    utterance.rate = example.rate
    utterance.pitch = example.pitch
    utterance.voice = matchingVoice ?? null
    utterance.onend = () => setActiveVoice(null)
    utterance.onerror = () => setActiveVoice(null)

    setActiveVoice(example.title)
    window.speechSynthesis.speak(utterance)
  }

  return (
    <section className='relative z-10 px-6 py-20 md:py-28'>
      <div className='mx-auto grid max-w-7xl gap-8 lg:grid-cols-[0.85fr_1.15fr] lg:items-stretch'>
        <AnimateInView className='flex flex-col justify-between'>
          <div>
            <p className='text-muted-foreground mb-3 text-xs font-semibold tracking-widest uppercase'>
              {t('Voice generation')}
            </p>
            <h2 className='max-w-xl text-3xl leading-tight font-bold tracking-normal md:text-5xl'>
              {t('Turn prompts into natural audio.')}
            </h2>
            <p className='text-muted-foreground mt-5 max-w-lg text-sm leading-relaxed md:text-base'>
              {t(
                'Use DeepRouter for text-to-speech, narration, multilingual voice, and realtime audio workflows from the same account you use for chat models.'
              )}
            </p>
          </div>

          <div className='border-border bg-card/75 mt-8 rounded-2xl border p-5 shadow-[0_14px_38px_rgb(28_28_28/0.06)]'>
            <div className='flex items-center justify-between gap-4'>
              <div>
                <p className='text-sm font-semibold'>{t('TTS request')}</p>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Model route: voice provider → audio response')}
                </p>
              </div>
              <span className='bg-success/10 text-success rounded-full px-3 py-1 text-xs font-semibold'>
                {t('Ready')}
              </span>
            </div>
            <div className='bg-foreground mt-5 flex h-24 items-end gap-1.5 overflow-hidden rounded-xl px-4 py-4'>
              {WAVE_BARS.map((height, index) => (
                <span
                  key={`${height}-${index}`}
                  className='bg-primary-foreground/80 w-full min-w-2 rounded-full'
                  style={{ height: `${height}%` }}
                />
              ))}
            </div>
          </div>
        </AnimateInView>

        <div className='grid gap-3 sm:grid-cols-2'>
          {VOICE_EXAMPLES.map((example, index) => {
            const Icon = example.icon
            const isActive = activeVoice === example.title

            return (
              <AnimateInView
                key={example.title}
                delay={index * 80}
                animation='scale-in'
                className={`border-border bg-card/80 rounded-2xl border p-5 shadow-[0_12px_34px_rgb(28_28_28/0.06)] transition duration-300 ${
                  isActive
                    ? 'border-primary/40 shadow-[0_18px_46px_rgb(84_108_255/0.18)]'
                    : ''
                }`}
              >
                <div className='flex items-center justify-between gap-3'>
                  <div className='flex min-w-0 items-center gap-3'>
                    <div
                      className={`flex size-10 shrink-0 items-center justify-center rounded-xl ${
                        example.tone === 'success'
                          ? 'bg-success/10 text-success'
                          : example.tone === 'warning'
                            ? 'bg-warning/10 text-warning'
                            : 'bg-accent/10 text-accent'
                      }`}
                    >
                      <Icon className='size-5' strokeWidth={1.7} />
                    </div>
                    <div className='min-w-0'>
                      <h3 className='truncate text-sm font-semibold'>
                        {t(example.title)}
                      </h3>
                      <p className='text-muted-foreground mt-0.5 truncate text-xs'>
                        {t(example.meta)}
                      </p>
                    </div>
                  </div>

                  <button
                    type='button'
                    disabled={!isSpeechSupported}
                    onClick={() => playExample(example)}
                    className='border-border bg-background hover:bg-muted disabled:text-muted-foreground flex size-10 shrink-0 items-center justify-center rounded-full border transition disabled:cursor-not-allowed'
                    aria-label={`${isActive ? t('Stop sample') : t('Play sample')}: ${t(example.title)}`}
                  >
                    {isActive ? (
                      <Square className='size-4 fill-current' strokeWidth={2} />
                    ) : (
                      <Play
                        className='ml-0.5 size-4 fill-current'
                        strokeWidth={2}
                      />
                    )}
                  </button>
                </div>
                <p className='text-muted-foreground mt-5 min-h-16 text-sm leading-relaxed'>
                  “{t(example.text)}”
                </p>
                <div className='mt-5 flex items-center gap-1.5'>
                  {WAVE_BARS.slice(0, 10).map((height, barIndex) => (
                    <span
                      key={`${example.title}-${barIndex}`}
                      className={`w-full rounded-full transition-colors ${
                        isActive ? 'bg-primary animate-pulse' : 'bg-border'
                      }`}
                      style={{
                        height: `${Math.max(8, height / 2)}px`,
                        animationDelay: `${barIndex * 80}ms`,
                      }}
                    />
                  ))}
                </div>

                <button
                  type='button'
                  disabled={!isSpeechSupported}
                  onClick={() => playExample(example)}
                  className='text-muted-foreground hover:text-foreground disabled:text-muted-foreground/60 mt-5 text-xs font-semibold transition disabled:cursor-not-allowed'
                >
                  {isSpeechSupported
                    ? isActive
                      ? t('Stop sample')
                      : t('Play sample')
                    : t('Audio preview is not supported in this browser')}
                </button>
              </AnimateInView>
            )
          })}
        </div>
      </div>
    </section>
  )
}
