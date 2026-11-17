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
import { api } from '@/lib/api'

/** Whether an admin has saved a TypeSafe key. The key itself is never returned. */
export async function getSmartRoutingStatus(): Promise<{
  success: boolean
  message?: string
  data?: { jev_api_key_configured: boolean }
}> {
  const res = await api.get('/api/option/smart_router', {
    disableDuplicate: true,
  } as Record<string, unknown>)
  return res.data
}

/** smart-router's answer to one admin test, as relayed by the gateway (Smart Routing V2, D7). */
export interface SmartRoutingTestResult {
  /** ok: smart-router answered; not_configured: none on this server; unreachable: it did not answer. */
  outcome: 'ok' | 'not_configured' | 'unreachable'
  primary?: string
  fallback_chain?: string[]
  reason?: string
  error?: string
  fallback_to_default?: string
  explain?: {
    jev_status: string
    jev_ms: number
    task?: string
    difficulty?: number
    rule: string
    proposed_primary: string
  }
  elapsed_ms: number
  gateway_wait_ms: number
}

/** Asks how deeprouter-auto would route `prompt` for the signed-in admin. No model is called. */
export async function testSmartRouting(prompt: string): Promise<{
  success: boolean
  message?: string
  data?: SmartRoutingTestResult
}> {
  const res = await api.post('/api/option/smart_router/test', { prompt })
  return res.data
}
