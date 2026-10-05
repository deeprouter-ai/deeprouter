// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useId, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
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
import { defaultBaseUrl } from '../lib/integration'
import {
  mediaEndpoint,
  mediaExample,
  type KeyModel,
} from '../lib/media-integration'
import { KeyModelDiscovery } from './key-model-discovery'

export function MediaKeySetup({
  apiKey,
  purpose,
}: {
  apiKey?: string | null
  purpose: string
}) {
  const { t } = useTranslation()
  const id = useId()
  const baseUrl = defaultBaseUrl()
  const [selected, setSelected] = useState('')
  const { data, isPending, isError, refetch } = useQuery({
    // Only a short suffix distinguishes credentials; full keys never enter cache keys.
    queryKey: ['media-key-models', id, baseUrl, apiKey?.slice(-8)],
    enabled: Boolean(apiKey),
    gcTime: 0,
    retry: false,
    queryFn: async ({ signal }) => {
      const response = await fetch(`${baseUrl}/models`, {
        headers: { Authorization: `Bearer ${apiKey}` },
        signal,
      })
      if (!response.ok) throw new Error('Cannot load key models')
      const body = (await response.json()) as {
        success?: boolean
        data?: KeyModel[]
      }
      if (body.success === false || !Array.isArray(body.data))
        throw new Error('Invalid model directory')
      return body.data
    },
  })
  const models = (data ?? []).filter((model) => mediaEndpoint(purpose, model))
  const model = models.find((item) => item.id === selected) ?? models[0]
  const endpoint = model ? mediaEndpoint(purpose, model) : null
  const guidance =
    purpose === 'video'
      ? 'Use this key in a tool that supports video generation. Submit a video task, then check its status until the video is ready.'
      : purpose === 'image'
        ? 'Use this key in a tool that supports image generation. Choose one of the included models below.'
        : 'Use this key in a tool that supports speech. Choose one of the included models below. You do not need a separate provider key.'

  return (
    <div className='space-y-3' aria-live='polite'>
      <p className='text-muted-foreground text-sm'>{t(guidance)}</p>
      <KeyModelDiscovery apiKey={apiKey} showModels={false} />
      {!apiKey ? (
        <p className='text-sm'>
          {t('Open this guide from a specific key to see its included models.')}
        </p>
      ) : isPending ? (
        <p className='text-sm'>{t('Loading included models…')}</p>
      ) : isError ? (
        <div className='space-y-2'>
          <p className='text-sm'>
            {t(
              'Could not load this key’s models. Retry before configuring your tool.'
            )}
          </p>
          <Button variant='outline' size='sm' onClick={() => void refetch()}>
            {t('Retry')}
          </Button>
        </div>
      ) : models.length === 0 ? (
        <p className='text-sm'>
          {t(
            'This key has no models for this purpose. Create a new key with the matching purpose, or contact support.'
          )}
        </p>
      ) : (
        <>
          <p className='text-sm font-semibold'>
            {t('All {{count}} models below are included in this key.', {
              count: models.length,
            })}
          </p>
          <div className='space-y-1 text-sm'>
            <label htmlFor={`${id}-model`}>{t('Choose a model to use')}</label>
            <Select
              items={models.map((item) => ({ value: item.id, label: item.id }))}
              value={model?.id}
              onValueChange={(value) => value !== null && setSelected(value)}
            >
              <SelectTrigger id={`${id}-model`} className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {models.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.id}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <Button
            size='sm'
            variant='outline'
            onClick={async () => {
              if (!model) return
              try {
                await navigator.clipboard.writeText(model.id)
                toast.success(t('Copied'))
              } catch {
                toast.error(t('Copy failed'))
              }
            }}
          >
            {t('Copy model name')}
          </Button>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Copy this key into your tool’s key field, then select the same model. Model access is configured; generation has not been tested.'
            )}
          </p>
          <details className='border-border rounded-lg border p-3'>
            <summary className='cursor-pointer text-sm'>
              {t('Developer details')}
            </summary>
            <div className='mt-3 space-y-2 text-xs'>
              <p>
                {t('Base URL')}: <code>{baseUrl}</code>
              </p>
              <p>
                {t('Endpoint')}:{' '}
                <code>
                  POST {baseUrl}
                  {endpoint}
                </code>
              </p>
              <p>
                {t(
                  'Example only. Running a generation request uses your account balance. Set DEEPROUTER_API_KEY securely and adjust model-specific parameters before running.'
                )}
              </p>
              <pre className='bg-muted/30 overflow-x-auto rounded-md p-3'>
                <code>
                  {endpoint &&
                    model &&
                    mediaExample(baseUrl, endpoint, model.id)}
                </code>
              </pre>
              {purpose === 'video' && (
                <p>
                  {t(
                    'A task ID means the request was accepted. Wait for a completed task and a playable video before considering generation successful.'
                  )}
                </p>
              )}
            </div>
          </details>
        </>
      )}
    </div>
  )
}
