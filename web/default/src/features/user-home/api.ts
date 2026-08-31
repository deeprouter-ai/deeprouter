/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { getUserProfile } from '@/features/profile/api'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
} from '@/features/subscriptions/api'
import { getUserBillingHistory } from '@/features/wallet/api'
import type { UserHomeActivePlan, UserHomeData } from './types'

export async function getUserHome(): Promise<UserHomeData> {
  const [profileRes, subsRes, plansRes, billingRes] = await Promise.all([
    getUserProfile(),
    getSelfSubscriptionFull(),
    getPublicPlans(),
    getUserBillingHistory(1, 1),
  ])

  const profile = profileRes.data
  if (!profile) {
    throw new Error('Failed to load user profile')
  }

  const activeSubscription = subsRes.data?.subscriptions.find(
    (record) => record.subscription.status === 'active'
  )?.subscription
  const plan = activeSubscription
    ? plansRes.data?.find(
        (record) => record.plan.id === activeSubscription.plan_id
      )?.plan
    : undefined

  const activePlan: UserHomeActivePlan | null = activeSubscription
    ? {
        title: plan?.title ?? '',
        status: activeSubscription.status,
        end_time: activeSubscription.end_time,
      }
    : null

  return {
    account: {
      balance_quota: profile.quota,
      used_quota: profile.used_quota,
      topups_count: billingRes.data?.total ?? 0,
    },
    active_plan: activePlan,
  }
}
