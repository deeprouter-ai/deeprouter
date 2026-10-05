// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useId, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { defaultBaseUrl } from '../lib/integration'
import type { KeyModel } from '../lib/media-integration'

export function KeyModelDiscovery({
  apiKey,
  showModels = true,
}: {
  apiKey?: string | null
  showModels?: boolean
}) {
  const { t } = useTranslation()
  const id = useId()
  const [open, setOpen] = useState(false)
  const baseUrl = defaultBaseUrl()
  const { data, isPending, isError, refetch } = useQuery({
    queryKey: ['key-model-discovery', id, baseUrl, apiKey?.slice(-8)],
    enabled: open && showModels && Boolean(apiKey),
    gcTime: 0,
    retry: false,
    queryFn: async ({ signal }) => {
      const response = await fetch(`${baseUrl}/models`, {
        headers: { Authorization: `Bearer ${apiKey}` },
        signal,
      })
      if (!response.ok) throw new Error('Model discovery failed')
      const body = (await response.json()) as {
        success?: boolean
        data?: KeyModel[]
      }
      if (body.success === false || !Array.isArray(body.data))
        throw new Error('Invalid model directory')
      return body.data
    },
  })
  return (
    <details
      className='border-border rounded-lg border p-3'
      onToggle={(event) => setOpen(event.currentTarget.open)}
    >
      <summary className='cursor-pointer text-sm'>
        {t('Where can I find this key’s models?')}
      </summary>
      <div className='mt-3 space-y-2 text-xs'>
        <p>
          {t(
            'This key’s model directory is the source of truth. Provider websites and the public pricing catalog may list models this key cannot use.'
          )}
        </p>
        <p>
          <code>GET {baseUrl}/models</code>
        </p>
        <p>
          {t(
            'Set DEEPROUTER_API_KEY securely, then run this read-only query. It does not generate media.'
          )}
        </p>
        <pre className='bg-muted/30 overflow-x-auto rounded-md p-3'>
          <code>{`curl ${baseUrl}/models \\\n  -H "Authorization: Bearer $DEEPROUTER_API_KEY"`}</code>
        </pre>
        {showModels &&
          (!apiKey ? (
            <p>
              {t(
                'Open this guide from a specific key to see its included models.'
              )}
            </p>
          ) : isError ? (
            <div>
              <p>
                {t(
                  'Could not load this key’s models. Retry before configuring your tool.'
                )}
              </p>
              <Button
                size='sm'
                variant='outline'
                onClick={() => void refetch()}
              >
                {t('Retry')}
              </Button>
            </div>
          ) : isPending ? (
            <p>{t('Loading included models…')}</p>
          ) : (
            <ul className='max-h-48 space-y-1 overflow-y-auto'>
              {data?.length ? (
                data.map((model) => (
                  <li key={model.id}>
                    <code>{model.id}</code>
                    <span className='text-muted-foreground'>
                      {' '}
                      —{' '}
                      {(model.supported_endpoint_types ?? []).join(', ') ||
                        t('Endpoint metadata unavailable')}
                    </span>
                  </li>
                ))
              ) : (
                <li>{t('No models are available to this key.')}</li>
              )}
            </ul>
          ))}
      </div>
    </details>
  )
}
