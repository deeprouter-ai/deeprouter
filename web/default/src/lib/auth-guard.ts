// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'

// 内存中的验证标记，避免同一会话中重复验证
let sessionVerified = false

/**
 * Route guard shared by every signed-in surface (the full console under
 * `/_authenticated` and the Simple console under `/simple`): bounce to the
 * sign-in page without a local user, and re-validate the session once per
 * page load.
 */
export async function ensureAuthenticated(href: string): Promise<void> {
  const { auth } = useAuthStore.getState()

  // 如果本地没有用户信息，直接跳转登录页
  if (!auth.user) {
    throw redirect({ to: '/sign-in', search: { redirect: href } })
  }

  // 本地有用户信息，但需要验证 session 是否有效（每个会话只验证一次）
  if (!sessionVerified) {
    const res = await getSelf().catch(() => null)
    if (res?.success && res.data) {
      // 验证成功，更新用户信息（可能有变化）
      auth.setUser(res.data)
      sessionVerified = true
    } else {
      // 验证失败或 API 调用失败，清除本地缓存并跳转登录页
      auth.reset()
      throw redirect({ to: '/sign-in', search: { redirect: href } })
    }
  }
}
