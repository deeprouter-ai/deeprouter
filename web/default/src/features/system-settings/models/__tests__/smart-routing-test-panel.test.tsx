// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SmartRoutingTestResult } from '../smart-routing-api'
import { SmartRoutingTestPanel } from '../smart-routing-test-panel'

// Smart Routing V2 (meta-repo docs/smart-routing-v2-prd.md, D7): the admin test
// panel shows what Jev judged (or why it was not used), the model and rule
// deeprouter-auto would pick, and whether the answer came within the gateway's
// wait. It only displays what the gateway relays; it never calls a model.

const mockTest = vi.hoisted(() => vi.fn())

vi.mock('@/features/system-settings/models/smart-routing-api', () => ({
  testSmartRouting: mockTest,
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, vars?: Record<string, unknown>) =>
      key.replace(/{{(\w+)}}/g, (_, name: string) => String(vars?.[name])),
  }),
}))

/** Renders the panel, types `prompt` and presses Test with the gateway answering `data`. */
async function runTest(data: SmartRoutingTestResult, prompt = 'hello') {
  mockTest.mockResolvedValue({ success: true, data })
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={new QueryClient()}>
      <SmartRoutingTestPanel />
    </QueryClientProvider>
  )
  await user.type(screen.getByLabelText('Request to test'), prompt)
  await user.click(screen.getByRole('button', { name: 'Test' }))
}

describe('SmartRoutingTestPanel', () => {
  beforeEach(() => mockTest.mockReset())

  it('cannot be run without text', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <SmartRoutingTestPanel />
      </QueryClientProvider>
    )
    expect(
      screen.getByRole('button', { name: 'Test' }).hasAttribute('disabled')
    ).toBe(true)
  })

  it("shows Jev's verdict, the model, the rule and the timing", async () => {
    await runTest(
      {
        outcome: 'ok',
        primary: 'claude-sonnet-5',
        reason: 'jev:code_hard',
        explain: {
          jev_status: 'ok',
          jev_ms: 287,
          task: 'code',
          difficulty: 4,
          rule: 'jev_code_hard',
          proposed_primary: 'claude-sonnet-5',
        },
        elapsed_ms: 412,
        gateway_wait_ms: 800,
      },
      '  design a lock-free map  '
    )

    // react-query passes its own context as a second argument; the prompt is the first.
    expect(mockTest.mock.calls[0][0]).toBe('design a lock-free map')
    expect(
      await screen.findByText('Code, difficulty 4 of 5, in 287 ms')
    ).toBeTruthy()
    expect(screen.getByText('claude-sonnet-5')).toBeTruthy()
    expect(screen.getByText('jev_code_hard (jev:code_hard)')).toBeTruthy()
    expect(
      screen.getByText('412 ms; live requests wait at most 800 ms')
    ).toBeTruthy()
    expect(screen.queryByText(/Longer than the gateway waits/)).toBeNull()
  })

  it('says why Jev was not used and when no model fits', async () => {
    await runTest({
      outcome: 'ok',
      error: 'no_model_matches_constraints',
      fallback_to_default: 'gpt-4o-mini',
      explain: {
        jev_status: 'no_key',
        jev_ms: 0,
        rule: 'chinese_short',
        proposed_primary: 'deepseek-chat',
      },
      elapsed_ms: 5,
      gateway_wait_ms: 800,
    })

    expect(
      await screen.findByText(
        'Not used: no TypeSafe key in effect yet (a saved key takes up to 30 seconds)'
      )
    ).toBeTruthy()
    expect(
      screen.getByText(
        'None this account can use; the gateway would use gpt-4o-mini'
      )
    ).toBeTruthy()
  })

  it("flags a swapped model and an answer slower than the gateway's wait", async () => {
    await runTest({
      outcome: 'ok',
      primary: 'gpt-4o',
      reason: 'jev:code_hard',
      explain: {
        jev_status: 'ok',
        jev_ms: 780,
        task: 'code',
        difficulty: 5,
        rule: 'jev_code_hard',
        proposed_primary: 'claude-sonnet-5',
      },
      elapsed_ms: 1900,
      gateway_wait_ms: 800,
    })

    expect(
      await screen.findByText(
        'The rule wanted claude-sonnet-5, which this account cannot use, so its next choice was taken.'
      )
    ).toBeTruthy()
    expect(
      screen.getByText(
        'Longer than the gateway waits: a live request would have used the default model this time.'
      )
    ).toBeTruthy()
  })

  it('says when smart-router did not answer', async () => {
    await runTest({
      outcome: 'unreachable',
      elapsed_ms: 3000,
      gateway_wait_ms: 800,
    })
    expect(
      await screen.findByText(
        'smart-router did not answer, so a live request would use the default model.'
      )
    ).toBeTruthy()
  })
})
