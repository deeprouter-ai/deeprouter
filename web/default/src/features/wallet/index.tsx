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
import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { getSelf } from '@/lib/api'
import { SectionPageLayout } from '@/components/layout'
import { CompanyWalletNotice } from '@/features/org/components/company-wallet-notice'
import { useWalletView } from '@/features/org/hooks/use-wallet-view'
import { AffiliateRewardsCard } from './components/affiliate-rewards-card'
import { AutoTopupCard } from './components/auto-topup-card'
import { AutoTopupPromptDialog } from './components/dialogs/auto-topup-prompt-dialog'
import { BillingHistoryDialog } from './components/dialogs/billing-history-dialog'
import { TransferDialog } from './components/dialogs/transfer-dialog'
import { RechargePanel } from './components/recharge-panel'
import { SubscriptionPlansCard } from './components/subscription-plans-card'
import { WalletStatsCard } from './components/wallet-stats-card'
import { useTopupInfo, useAffiliate } from './hooks'
import type { UserWalletData } from './types'

interface WalletProps {
  initialShowHistory?: boolean
}

/**
 * The wallet page. DeepRouter Enterprise Org: a member of an organization has
 * no wallet of their own to top up — their keys spend the company's — so the
 * page tells them that instead. Every "top up" link in the console leads
 * here, which makes this the one place that has to get it right.
 */
export function Wallet(props: WalletProps) {
  const { t } = useTranslation()
  const walletView = useWalletView()
  if (walletView === 'pending') return null
  if (walletView === 'company') {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Wallet')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <CompanyWalletNotice className='mx-auto w-full max-w-2xl' />
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }
  return <OwnWallet {...props} />
}

function OwnWallet(props: WalletProps) {
  const { t } = useTranslation()
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [transferDialogOpen, setTransferDialogOpen] = useState(false)
  const [billingDialogOpen, setBillingDialogOpen] = useState(false)
  const [autoTopupPromptOpen, setAutoTopupPromptOpen] = useState(false)
  const [showSubscriptionPanel, setShowSubscriptionPanel] = useState(true)

  const { topupInfo } = useTopupInfo()
  const {
    affiliateLink,
    loading: affiliateLoading,
    transferQuota,
    transferring,
  } = useAffiliate()

  // Fetch and refresh user data
  const fetchUser = useCallback(async () => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch user data:', error)
    } finally {
      setUserLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  // One-time prompt right after the first card top-up: card saved but
  // auto-recharge not yet enabled → offer to turn it on.
  useEffect(() => {
    if (
      user?.stripe_customer &&
      !user.auto_topup_enabled &&
      !localStorage.getItem('auto_topup_prompt_seen')
    ) {
      setAutoTopupPromptOpen(true)
    }
  }, [user])

  useEffect(() => {
    if (props.initialShowHistory) {
      setBillingDialogOpen(true)
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [props.initialShowHistory])

  // Handle transfer
  const handleTransfer = async (amount: number) => {
    const success = await transferQuota(amount)
    if (success) {
      await fetchUser()
    }
    return success
  }

  const handleSubscriptionAvailabilityChange = useCallback(
    (available: boolean) => {
      setShowSubscriptionPanel(available)
    },
    []
  )

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Wallet')}</SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t(
            'Top up to call AI models. Your trial credit covers your first few requests so you can try things out.'
          )}
        </SectionPageLayout.Description>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <WalletStatsCard user={user} loading={userLoading} />

            <div
              className={
                showSubscriptionPanel
                  ? 'grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(360px,0.95fr)] xl:items-start'
                  : 'grid gap-4'
              }
            >
              <div id='wallet-add-funds' className='scroll-mt-4'>
                <RechargePanel
                  onBalanceChange={fetchUser}
                  onOpenBilling={() => setBillingDialogOpen(true)}
                />
              </div>

              <SubscriptionPlansCard
                topupInfo={topupInfo}
                onAvailabilityChange={handleSubscriptionAvailabilityChange}
              />
            </div>

            <AutoTopupCard user={user} onSaved={fetchUser} />

            <AffiliateRewardsCard
              user={user}
              affiliateLink={affiliateLink}
              onTransfer={() => setTransferDialogOpen(true)}
              loading={affiliateLoading}
            />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <TransferDialog
        open={transferDialogOpen}
        onOpenChange={setTransferDialogOpen}
        onConfirm={handleTransfer}
        availableQuota={user?.aff_quota ?? 0}
        transferring={transferring}
      />

      <BillingHistoryDialog
        open={billingDialogOpen}
        onOpenChange={setBillingDialogOpen}
      />

      <AutoTopupPromptDialog
        open={autoTopupPromptOpen}
        onOpenChange={(open) => {
          setAutoTopupPromptOpen(open)
          if (!open) localStorage.setItem('auto_topup_prompt_seen', '1')
        }}
        onEnabled={() => {
          localStorage.setItem('auto_topup_prompt_seen', '1')
          fetchUser()
        }}
      />
    </>
  )
}
