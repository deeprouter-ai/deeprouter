// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
export type KeyModel = { id: string; supported_endpoint_types: string[] }

export function isMediaPurpose(purpose?: string | null): boolean {
  return purpose === 'video' || purpose === 'image' || purpose === 'voice'
}

export function mediaEndpoint(purpose: string, model: KeyModel): string | null {
  const types = model.supported_endpoint_types ?? []
  if (purpose === 'image' && types.includes('image-generation'))
    return '/images/generations'
  if (purpose === 'voice' && types.includes('audio-speech'))
    return '/audio/speech'
  if (purpose === 'voice' && types.includes('audio-transcription'))
    return '/audio/transcriptions'
  if (purpose === 'video' && types.includes('openai-video')) return '/videos'
  if (purpose === 'video' && types.includes('video-generation'))
    return '/video/generations'
  return null
}

// Keep secrets out of copyable examples and shell history. The selected model
// comes from this key's authenticated directory, never a made-up media alias.
export function mediaExample(
  baseUrl: string,
  endpoint: string,
  model: string
): string {
  const auth = '-H "Authorization: Bearer $DEEPROUTER_API_KEY"'
  if (endpoint === '/audio/transcriptions') {
    return `curl ${baseUrl}${endpoint} \\\n  ${auth} \\\n  -F 'model=${model}' -F 'file=@speech.wav'`
  }
  if (endpoint === '/videos') {
    return `curl ${baseUrl}${endpoint} \\\n  ${auth} \\\n  -F 'model=${model}' -F 'prompt=A calm ocean at sunset'\n\n# Query the task with its returned id\ncurl ${baseUrl}/videos/TASK_ID ${auth}`
  }
  const body =
    endpoint === '/audio/speech'
      ? {
          model,
          input: 'Hello from DeepRouter.',
          response_format: 'mp3',
          ...(!model.startsWith('eleven_') && !model.startsWith('speech-')
            ? { voice: 'alloy' }
            : {}),
        }
      : { model, prompt: 'A calm ocean at sunset' }
  const output =
    endpoint === '/audio/speech' ? ' \\\n  --output speech.mp3' : ''
  const query =
    endpoint === '/video/generations'
      ? `\n\n# Query the task with its returned task_id\ncurl ${baseUrl}/video/generations/TASK_ID ${auth}`
      : ''
  return `curl ${baseUrl}${endpoint} \\\n  ${auth} \\\n  -H 'Content-Type: application/json' \\\n  -d '${JSON.stringify(body)}'${output}${query}`
}
