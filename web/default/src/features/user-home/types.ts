/*
Copyright (C) 2026 DeepRouter
SPDX-License-Identifier: AGPL-3.0-or-later
*/
export interface UserHomeAccountStatus {
  balance_quota: number
  used_quota: number
  topups_count: number
}

export interface UserHomeActivePlan {
  title: string
  status: string
  end_time: number
}

export interface UserHomeData {
  account: UserHomeAccountStatus
  active_plan: UserHomeActivePlan | null
}
