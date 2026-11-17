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
import { useMutation } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { SettingsSection } from '../components/settings-section'
import {
  type SmartRoutingTestResult,
  testSmartRouting,
} from './smart-routing-api'

// Jev's task names, as smart-router reports them.
const TASK_LABELS: Record<string, string> = {
  code: 'Code',
  math: 'Math',
  writing: 'Writing',
  translation: 'Translation',
  analysis: 'Analysis',
  chat: 'Everyday chat',
}

// Why Jev was not used, by smart-router's jev_status.
const JEV_SKIPPED: Record<string, string> = {
  disabled: 'smart-router was started without Jev',
  skipped_kids: 'kids accounts never use Jev',
  off: 'the switch above is off',
  no_key: 'no TypeSafe key in effect yet (a saved key takes up to 30 seconds)',
  no_user_text: 'there was no user text to judge',
  timeout: 'Jev did not answer in time',
  breaker_open:
    'Jev failed several times in a row and is paused for 30 seconds',
  canceled: 'the request was canceled',
  error: 'Jev returned an error, often because the key is invalid',
}

/** Lets an admin type a request and see how deeprouter-auto would route it, without calling a model (Smart Routing V2, D7). */
export function SmartRoutingTestPanel() {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('')
  const test = useMutation({ mutationFn: testSmartRouting })
  const result = test.data?.success ? test.data.data : undefined

  return (
    <SettingsSection
      title={t('Try Jev')}
      description={t(
        'Type a request to see what Jev makes of it and which model deeprouter-auto would pick for your account. No model is called; each test sends one request to TypeSafe. It uses the settings already in effect, so allow 30 seconds after saving.'
      )}
    >
      <div className='max-w-2xl space-y-3'>
        <Textarea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          rows={4}
          aria-label={t('Request to test')}
          placeholder={t(
            'For example: Write me a template for a proof of income letter'
          )}
        />
        <Button
          type='button'
          variant='secondary'
          onClick={() => test.mutate(prompt.trim())}
          disabled={!prompt.trim() || test.isPending}
        >
          {test.isPending ? (
            <Loader2 className='me-2 size-4 animate-spin' />
          ) : null}
          {t('Test')}
        </Button>
        {result ? <TestResult result={result} /> : null}
      </div>
    </SettingsSection>
  )
}

/** Shows one test's outcome: Jev's verdict or why it was skipped, the model, the rule and the timing. */
function TestResult({ result }: { result: SmartRoutingTestResult }) {
  const { t } = useTranslation()

  if (result.outcome !== 'ok') {
    return (
      <p className='text-warning text-sm' role='status'>
        {result.outcome === 'not_configured'
          ? t(
              'smart-router is not configured on this server, so deeprouter-auto always uses the default model.'
            )
          : t(
              'smart-router did not answer, so a live request would use the default model.'
            )}
      </p>
    )
  }

  const ex = result.explain
  const status = ex?.jev_status ?? ''
  const task = ex?.task ?? ''
  const overWait = result.elapsed_ms > result.gateway_wait_ms
  const swapped =
    ex !== undefined &&
    result.primary !== undefined &&
    ex.proposed_primary !== result.primary

  return (
    <dl
      className='grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 rounded-lg border p-4 text-sm'
      role='status'
    >
      <dt className='text-muted-foreground'>{t('Jev verdict')}</dt>
      <dd className={status === 'ok' ? 'text-success' : 'text-warning'}>
        {status === 'ok'
          ? t('{{task}}, difficulty {{difficulty}} of 5, in {{ms}} ms', {
              task: t(TASK_LABELS[task] ?? task),
              difficulty: ex?.difficulty,
              ms: ex?.jev_ms,
            })
          : t('Not used: {{why}}', {
              why: t(JEV_SKIPPED[status] ?? status),
            })}
      </dd>

      <dt className='text-muted-foreground'>{t('Model')}</dt>
      <dd className='font-medium'>
        {result.primary ??
          t('None this account can use; the gateway would use {{model}}', {
            model: result.fallback_to_default,
          })}
        {swapped ? (
          <span className='text-muted-foreground block font-normal'>
            {t(
              'The rule wanted {{model}}, which this account cannot use, so its next choice was taken.',
              { model: ex.proposed_primary }
            )}
          </span>
        ) : null}
      </dd>

      <dt className='text-muted-foreground'>{t('Rule')}</dt>
      <dd>
        {ex?.rule}
        {result.reason ? ` (${result.reason})` : null}
      </dd>

      <dt className='text-muted-foreground'>{t('Time')}</dt>
      <dd>
        {t('{{ms}} ms; live requests wait at most {{wait}} ms', {
          ms: result.elapsed_ms,
          wait: result.gateway_wait_ms,
        })}
        {overWait ? (
          <span className='text-warning block'>
            {t(
              'Longer than the gateway waits: a live request would have used the default model this time.'
            )}
          </span>
        ) : null}
      </dd>
    </dl>
  )
}
