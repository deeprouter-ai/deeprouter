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
// Coverage: preset values that are markers rather than links, and the relative
// navigation that shipped because nothing told them apart.
import { describe, expect, it } from 'vitest'
import { chatPresetAction, parseChatConfig, resolveChatUrl } from './chat-links'

describe('chatPresetAction', () => {
  it('treats every real link as a link', () => {
    expect(chatPresetAction('https://chat-preview.lobehub.com/?settings=x')).toBe(
      'link'
    )
    expect(chatPresetAction('cherrystudio://providers/api-keys?v=1')).toBe('link')
    expect(chatPresetAction('fluentread')).toBe('link')
  })

  it('sends the ccswitch marker to its dialog', () => {
    // The admin console stores the literal string `ccswitch`, which is a name,
    // not an address. Opened as a URL it becomes a relative path and navigates
    // the console to /ccswitch — the defect this helper exists to end.
    expect(chatPresetAction('ccswitch')).toBe('cc-switch-dialog')
    expect(chatPresetAction('CCSwitch')).toBe('cc-switch-dialog')
  })

  it('refuses to guess at anything else without a scheme', () => {
    expect(chatPresetAction('somefutureapp')).toBe('unsupported')
    expect(chatPresetAction('/keys')).toBe('unsupported')
  })
})

describe('the shape that made the defect invisible', () => {
  it('parses the marker into a preset that looks openable', () => {
    // parseChatConfig keeps `ccswitch` and types it custom-protocol, and
    // resolveChatUrl hands it straight back — a truthy string, so every caller
    // that only checked "did I get a URL" opened it. Pinned so the next change
    // has to reckon with it rather than rediscover it.
    const [preset] = parseChatConfig([{ 'CC Switch': 'ccswitch' }])
    expect(preset.type).toBe('custom-protocol')
    expect(
      resolveChatUrl({
        template: preset.url,
        apiKey: 'sk-test',
        serverAddress: 'https://example.test',
      })
    ).toBe('ccswitch')
    expect(chatPresetAction(preset.url)).toBe('cc-switch-dialog')
  })
})
