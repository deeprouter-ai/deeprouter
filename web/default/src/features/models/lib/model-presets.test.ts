import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { PROVIDER_PRESETS } from '@/features/channels/lib/provider-presets'
import { MODEL_PRESETS } from './model-presets'

describe('model discovery catalog', () => {
  it('requires model and API sources for every importable model', () => {
    expect(new Set(MODEL_PRESETS.map((model) => model.model_name)).size).toBe(
      MODEL_PRESETS.length
    )
    for (const model of MODEL_PRESETS) {
      expect(new URL(model.sourceUrl).protocol).toBe('https:')
      expect(new URL(model.apiDocsUrl).protocol).toBe('https:')
      expect(model.description.length).toBeGreaterThan(0)
      expect(model.endpoints.length).toBeGreaterThan(0)
    }
  })
  it('keeps ElevenLabs TTS imports and seed aligned with the adapter', () => {
    const adapter = readFileSync(
      '../../relay/channel/elevenlabs/constant.go',
      'utf8'
    )
    const block = adapter.split('var ModelList = []string{')[1].split('}')[0]
    const expected = [...block.matchAll(/"([^"]+)"/g)]
      .map((match) => match[1])
      .sort()
    const channel = PROVIDER_PRESETS.find(
      (preset) => preset.id === 'elevenlabs'
    )!
    expect(channel.models.split(',').sort()).toEqual(expected)
    expect(
      MODEL_PRESETS.filter((preset) => preset.model_name.startsWith('eleven_'))
        .map((preset) => preset.model_name)
        .sort()
    ).toEqual(expected)
    const seed = readFileSync(
      '../../scripts/seed-models/channels.yaml',
      'utf8'
    ).split('  - name: ElevenLabs 语音合成')[1]
    expect(
      [...seed.matchAll(/ {6}- (\S+)/g)].map((match) => match[1]).sort()
    ).toEqual(expected)
    expect(seed).toContain('enabled: false')
    expect(expected.some((name) => name.startsWith('music_'))).toBe(false)
  })
})
