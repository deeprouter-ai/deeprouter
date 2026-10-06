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
import { useQuery } from '@tanstack/react-query'
import {
  LayoutDashboard,
  Activity,
  Key,
  FileText,
  Home,
  Wallet,
  Box,
  Users,
  Ticket,
  User,
  Command,
  Radio,
  // MessageSquare,  // un-comment when restoring chat-presets dropdown
  CreditCard,
  Clapperboard,
  ListTodo,
  Settings,
  HelpCircle,
  Sparkles,
  Receipt,
  Building2,
  ShieldCheck,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { WORKSPACE_IDS } from '@/components/layout/lib/workspace-registry'
import { type SidebarData } from '@/components/layout/types'
import {
  fetchMyPurchases,
  marketplaceQueryKeys,
} from '@/features/marketplace/api'
import {
  canSeeOrg,
  useOrgMembership,
} from '@/features/org/hooks/use-org-membership'

export function useSidebarData(): SidebarData {
  const { t } = useTranslation()

  // PRD §8.4: the Purchase History entry only exists for users who have at
  // least one paid purchase. One cached probe per session-ish window; a
  // purchase invalidates the marketplace key prefix so the entry appears
  // right away.
  const { data: purchasesProbe } = useQuery({
    queryKey: marketplaceQueryKeys.myPurchases(),
    queryFn: () => fetchMyPurchases({ limit: 100 }),
    staleTime: 5 * 60 * 1000,
  })
  const hasPurchases = (purchasesProbe?.total ?? 0) > 0

  // Enterprise Org: the "Organization" group exists for members whose role
  // lets them see members, departments and roles — the owner and admins, a
  // manager, a read-only member — and not for Staff. A personal account's
  // probe answers null and the group never appears.
  const { data: orgMembership } = useOrgMembership()
  const seesOrg = canSeeOrg(orgMembership)

  return {
    workspaces: [
      {
        id: WORKSPACE_IDS.DEFAULT,
        name: '', // Dynamically fetches system name
        logo: Command,
        plan: '', // Dynamically fetches system version
      },
    ],
    navGroups: [
      // DeepRouter: the "Chat / Playground" nav group is removed — the
      // playground is a developer console inherited from upstream new-api
      // and end users never use it ("不做 chat 是红线"). /playground itself
      // redirects to the dashboard. Re-add a group here if a developer-mode
      // playground is ever reintroduced.
      {
        id: 'general',
        title: t('General'),
        items: [
          {
            title: t('Overview'),
            url: '/dashboard/overview',
            icon: Activity,
          },
          {
            title: t('Dashboard'),
            url: '/dashboard/models',
            icon: LayoutDashboard,
          },
          {
            title: t('API Keys'),
            url: '/keys',
            icon: Key,
          },
          // Video First Wave P2: the「可以做视频」paste-prompt page.
          {
            title: t('Make videos'),
            url: '/video',
            icon: Clapperboard,
          },
          {
            title: t('Call history'),
            url: '/usage-logs/common',
            icon: FileText,
          },
          {
            title: t('Task Logs'),
            url: '/usage-logs/task',
            activeUrls: ['/usage-logs/drawing'],
            configUrls: ['/usage-logs/drawing', '/usage-logs/task'],
            icon: ListTodo,
          },
        ],
      },
      ...(seesOrg
        ? [
            {
              id: 'org',
              title: t('Organization'),
              items: [
                {
                  title: t('Members & departments'),
                  url: '/org/members',
                  icon: Building2,
                },
                {
                  title: t('Roles & permissions'),
                  url: '/org/roles',
                  icon: ShieldCheck,
                },
              ],
            },
          ]
        : []),
      {
        id: 'personal',
        title: t('Personal'),
        items: [
          {
            title: t('Home'),
            url: '/home',
            icon: Home,
          },
          {
            title: t('Wallet'),
            url: '/wallet',
            icon: Wallet,
          },
          {
            title: t('My Skills'),
            url: '/user/skills',
            icon: Sparkles,
          },
          ...(hasPurchases
            ? [
                {
                  title: t('Purchase History'),
                  url: '/user/purchases',
                  icon: Receipt,
                },
              ]
            : []),
          {
            title: t('Profile'),
            url: '/profile',
            icon: User,
          },
        ],
      },
      {
        id: 'admin',
        title: t('Admin'),
        items: [
          {
            title: t('Channels'),
            url: '/channels',
            icon: Radio,
          },
          {
            title: t('Models'),
            url: '/models/metadata',
            icon: Box,
          },
          {
            title: t('Users'),
            url: '/users',
            icon: Users,
          },
          {
            title: t('Redemption Codes'),
            url: '/redemption-codes',
            icon: Ticket,
          },
          {
            title: t('Subscription Management'),
            url: '/subscriptions',
            icon: CreditCard,
          },
          {
            title: t('Skills'),
            url: '/admin/skills',
            icon: Sparkles,
          },
          {
            title: t('System Settings'),
            url: '/system-settings/site',
            activeUrls: ['/system-settings'],
            icon: Settings,
          },
          // DeepRouter cheatsheet — keeps the Channel/Model/Group pricing
          // relationship a click away so the operator never has to re-derive
          // the quota formula from memory.
          {
            title: t('Pricing Cheatsheet'),
            url: '/help/pricing',
            icon: HelpCircle,
          },
        ],
      },
    ],
  }
}
