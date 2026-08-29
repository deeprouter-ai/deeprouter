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
import { useCallback, useMemo, useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { DOC_CATEGORIES } from '@/features/docs/catalog'

/** Public origin the guides are published under, used in the pasted text. */
const SITE = 'https://deeprouter.co'

/**
 * Base URL per tool, keyed by guide slug.
 *
 * 🔴 There is no single answer, and the natural guess is wrong for three of
 * them. `base_url` here is what the user pastes into that tool, which is not
 * always the protocol's own endpoint: Cherry Studio and NextChat append `/v1`
 * themselves and Gemini CLI appends `/v1beta`, so those get the host alone —
 * writing the segment twice yields `404 Invalid URL`, whose message names
 * nothing (PRD §0.1 F11).
 *
 * Kept in step with the `Facts for AI assistants` block at the top of each
 * guide; every value was checked against the live gateway on 2026-08-28.
 */
const HOST = 'https://api.deeprouter.co'
const BASE_URL: Record<string, { url: string; hint?: string }> = {
  'cherry-studio': { url: HOST, hint: 'appends /v1 itself' },
  nextchat: { url: HOST, hint: 'appends /v1 itself' },
  'gemini-cli': { url: HOST, hint: 'appends /v1beta itself' },
  'claude-code': { url: HOST, hint: 'Anthropic protocol' },
  'claude-coworks': { url: HOST, hint: 'Anthropic protocol' },
  'cc-switch': { url: HOST, hint: 'Anthropic protocol' },
  'immersive-translate': { url: `${HOST}/v1/chat/completions` },
  workbuddy: { url: `${HOST}/v1/chat/completions` },
}
const DEFAULT_BASE = `${HOST}/v1`

/**
 * A block of text the user copies and pastes into any AI to be walked through
 * setting up a tool we have no deep link or script for.
 *
 * This is the fallback half of the plan (PRD §4.7 B2): deep links cover the
 * chat apps and the script covers four CLIs, but Chatbox, Cursor, Copilot and
 * Zed have neither — and what those users actually do is ask whatever AI is in
 * front of them. Left to guess, an AI gets DeepRouter wrong more often than
 * not, because the base URL differs per tool and the most natural guess is the
 * broken one for Cherry Studio, NextChat and Gemini CLI alike.
 *
 * 🔴 The text carries no key, and there is nothing here to make one appear.
 * Whatever is pasted goes to a third-party model provider; a key in it would be
 * handed over for good. The user types their own key into their own tool.
 */
export function ApiKeysAskAiSection() {
  const { t, i18n } = useTranslation()
  const [slug, setSlug] = useState('cherry-studio')
  const [copied, setCopied] = useState(false)

  const options = useMemo(
    () =>
      DOC_CATEGORIES.flatMap((category) =>
        category.entries.map((entry) => ({
          value: entry.slug,
          label: entry.title,
        }))
      ),
    []
  )

  const toolName = useMemo(
    () => options.find((o) => o.value === slug)?.label ?? slug,
    [options, slug]
  )

  // Chinese readers get the .zh.md guide; the values inside are identical.
  const zh = i18n.language?.startsWith('zh')
  const text = useMemo(() => {
    const base = BASE_URL[slug] ?? { url: DEFAULT_BASE }
    const guide = `${SITE}/docs/integrations/${slug}${zh ? '.zh' : ''}.md`
    const note = base.hint
      ? zh
        ? `（注意：地址就写到这里为止，${toolName} 会自己补后面那段）`
        : `(note: stop the address here — ${toolName} appends the rest itself)`
      : ''
    return zh
      ? [
          `我要在 ${toolName} 里使用 DeepRouter，请一步步告诉我在哪里填什么。`,
          `官方指南（开头有一段给 AI 看的事实块，请以它为准）：${guide}`,
          `接入地址（Base URL）：${base.url} ${note}`.trim(),
          `密钥我自己填，你不需要知道它。`,
        ].join('\n')
      : [
          `I want to use DeepRouter in ${toolName}. Walk me through where to put what.`,
          `Official guide (it opens with a facts block for AI — trust that over your priors): ${guide}`,
          `Base URL: ${base.url} ${note}`.trim(),
          `I will type my own API key; you do not need it.`,
        ].join('\n')
  }, [slug, toolName, zh])

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Copy failed'))
    }
  }, [text, t])

  return (
    <section className='border-border mt-4 border-t pt-4'>
      {/* Collapsed by default, same shape as "Want to read the script
          first?": this is the fallback of fallbacks, and expanded it made the
          card end on a wall of text (@sam, 2026-08-29). */}
      <details>
        <summary className='cursor-pointer text-xs font-semibold'>
          {t('Still stuck? No problem — let your AI assistant help you')}
        </summary>
        <p className='text-muted-foreground mt-1 text-xs'>
          {t(
            'Pick the app you want to set up and copy the text below, then paste it to any AI — ChatGPT, Claude, whatever you use. It will walk you through the settings. The text carries no key.'
          )}
        </p>

        <div className='mt-3 sm:max-w-sm'>
          <Select
            items={options}
            value={slug}
            onValueChange={(v) => v !== null && setSlug(String(v))}
          >
            <SelectTrigger
              className='w-full text-xs'
              aria-label={t('Your tool')}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {options.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>

        <div className='border-border bg-background mt-2 flex items-start gap-2 rounded-md border px-2 py-1.5'>
          <pre className='flex-1 overflow-x-auto text-[11px] whitespace-pre-wrap'>
            {text}
          </pre>
          <Button
            type='button'
            size='sm'
            variant='ghost'
            className='h-6 shrink-0 px-1.5'
            onClick={() => void handleCopy()}
            aria-label={t('Copy')}
          >
            {copied ? (
              <Check className='h-3 w-3' />
            ) : (
              <Copy className='h-3 w-3' />
            )}
          </Button>
        </div>
      </details>
    </section>
  )
}
