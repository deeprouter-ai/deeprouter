// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState, useEffect, useCallback, useMemo } from 'react'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { DEFAULT_DISCOUNT_RATE } from '../constants'
import {
  useTopupInfo,
  usePayment,
  useRedemption,
  useCreemPayment,
  useWaffoPayment,
  useWaffoPancakePayment,
  useAirwallexPayment,
} from '../hooks'
import {
  getDefaultPaymentType,
  getMinTopupAmount,
  isWaffoPancakePayment,
} from '../lib'
import type { PaymentMethod, PresetAmount, CreemProduct } from '../types'
import { CreemConfirmDialog } from './dialogs/creem-confirm-dialog'
import { PaymentConfirmDialog } from './dialogs/payment-confirm-dialog'
import { RechargeFormCard } from './recharge-form-card'

interface RechargePanelProps {
  /** Called after anything that may have changed the balance (paid, redeemed). */
  onBalanceChange?: () => void
  onOpenBilling?: () => void
}

/**
 * The whole add-funds flow: amount, every enabled payment gateway, redemption
 * codes and the confirmation dialogs. Lifted out of the wallet page so the
 * Simple console's top-up sheet runs the exact same payment logic instead of a
 * second copy — a gateway added here reaches both surfaces at once.
 */
export function RechargePanel({
  onBalanceChange,
  onOpenBilling,
}: RechargePanelProps) {
  const [topupAmount, setTopupAmount] = useState(0)
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null)
  const [selectedPaymentMethod, setSelectedPaymentMethod] =
    useState<PaymentMethod>()
  const [paymentLoading, setPaymentLoading] = useState<string | null>(null)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [creemDialogOpen, setCreemDialogOpen] = useState(false)
  const [selectedCreemProduct, setSelectedCreemProduct] =
    useState<CreemProduct | null>(null)

  const { status } = useStatus()
  const { currency } = useSystemConfig()
  const { topupInfo, presetAmounts, loading: topupLoading } = useTopupInfo()

  // Calculate effective exchange rate - when display type is USD, use rate of 1
  const effectiveUsdExchangeRate = useMemo(() => {
    return currency?.quotaDisplayType === 'USD'
      ? 1
      : currency?.usdExchangeRate || 1
  }, [currency?.quotaDisplayType, currency?.usdExchangeRate])
  const {
    amount: paymentAmount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
  } = usePayment()
  const { redeeming, redeemCode } = useRedemption()
  const { processing: creemProcessing, processCreemPayment } = useCreemPayment()
  const { processWaffoPayment } = useWaffoPayment()
  const { processing: pancakeProcessing, processWaffoPancakePayment } =
    useWaffoPancakePayment()
  const { processing: airwallexProcessing, processAirwallexPayment } =
    useAirwallexPayment()

  // Initialize topup amount when topup info is loaded
  useEffect(() => {
    if (topupInfo && topupAmount === 0) {
      const minTopup = getMinTopupAmount(topupInfo)
      setTopupAmount(minTopup)

      // Calculate initial payment amount with default payment type
      const defaultPaymentType = getDefaultPaymentType(topupInfo)
      calculatePaymentAmount(minTopup, defaultPaymentType)
    }
  }, [topupInfo, topupAmount, calculatePaymentAmount])

  // Get current payment type (selected or default)
  const getCurrentPaymentType = useCallback(() => {
    return selectedPaymentMethod?.type || getDefaultPaymentType(topupInfo)
  }, [selectedPaymentMethod, topupInfo])

  // Handle preset selection
  const handleSelectPreset = (preset: PresetAmount) => {
    setTopupAmount(preset.value)
    setSelectedPreset(preset.value)
    calculatePaymentAmount(preset.value, getCurrentPaymentType())
  }

  // Handle topup amount change
  const handleTopupAmountChange = (amount: number) => {
    setTopupAmount(amount)
    setSelectedPreset(null)
    calculatePaymentAmount(amount, getCurrentPaymentType())
  }

  // Handle payment method selection
  const handlePaymentMethodSelect = async (method: PaymentMethod) => {
    setSelectedPaymentMethod(method)
    setPaymentLoading(method.type)

    try {
      // Validate minimum topup
      const minTopup = getMinTopupAmount(topupInfo)
      if (topupAmount < minTopup) {
        return
      }

      // Calculate payment amount and show confirmation dialog
      await calculatePaymentAmount(topupAmount, method.type)
      setConfirmDialogOpen(true)
    } finally {
      setPaymentLoading(null)
    }
  }

  // Handle payment confirmation
  const handlePaymentConfirm = async () => {
    if (!selectedPaymentMethod) return

    const isPancake = isWaffoPancakePayment(selectedPaymentMethod.type)
    const success = isPancake
      ? await processWaffoPancakePayment(topupAmount)
      : await processPayment(topupAmount, selectedPaymentMethod.type)

    if (success) {
      setConfirmDialogOpen(false)
      onBalanceChange?.()
    }
  }

  // Handle redemption
  const handleRedeem = async () => {
    if (!redemptionCode) return

    const success = await redeemCode(redemptionCode)
    if (success) {
      setRedemptionCode('')
      onBalanceChange?.()
    }
  }

  // Handle Creem product selection
  const handleCreemProductSelect = (product: CreemProduct) => {
    setSelectedCreemProduct(product)
    setCreemDialogOpen(true)
  }

  // Handle Creem payment confirmation
  const handleCreemConfirm = async () => {
    if (!selectedCreemProduct) return

    const success = await processCreemPayment(selectedCreemProduct.productId)
    if (success) {
      setCreemDialogOpen(false)
      setSelectedCreemProduct(null)
      onBalanceChange?.()
    }
  }

  const handleAirwallexPay = useCallback(
    async (currency: string, saveForFuture = false) => {
      if (topupAmount <= 0) return
      await processAirwallexPayment(topupAmount, currency, saveForFuture)
    },
    [processAirwallexPayment, topupAmount]
  )

  const handleWaffoMethodSelect = async (_method: unknown, index: number) => {
    const loadingKey = `waffo-${index}`
    setPaymentLoading(loadingKey)

    try {
      await processWaffoPayment(topupAmount, index)
    } finally {
      setPaymentLoading(null)
    }
  }

  // Get discount rate for current topup amount
  const getDiscountRate = useCallback(() => {
    return topupInfo?.discount?.[topupAmount] || DEFAULT_DISCOUNT_RATE
  }, [topupInfo, topupAmount])

  return (
    <>
      <RechargeFormCard
        topupInfo={topupInfo}
        presetAmounts={presetAmounts}
        selectedPreset={selectedPreset}
        onSelectPreset={handleSelectPreset}
        topupAmount={topupAmount}
        onTopupAmountChange={handleTopupAmountChange}
        paymentAmount={paymentAmount}
        calculating={calculating}
        onPaymentMethodSelect={handlePaymentMethodSelect}
        paymentLoading={paymentLoading}
        redemptionCode={redemptionCode}
        onRedemptionCodeChange={setRedemptionCode}
        onRedeem={handleRedeem}
        redeeming={redeeming}
        topupLink={topupInfo?.topup_link}
        loading={topupLoading}
        priceRatio={(status?.price as number) || 1}
        usdExchangeRate={effectiveUsdExchangeRate}
        onOpenBilling={onOpenBilling}
        creemProducts={topupInfo?.creem_products}
        enableCreemTopup={topupInfo?.enable_creem_topup}
        onCreemProductSelect={handleCreemProductSelect}
        enableWaffoTopup={topupInfo?.enable_waffo_topup}
        waffoPayMethods={topupInfo?.waffo_pay_methods}
        waffoMinTopup={topupInfo?.waffo_min_topup}
        onWaffoMethodSelect={handleWaffoMethodSelect}
        enableWaffoPancakeTopup={topupInfo?.enable_waffo_pancake_topup}
        enableAirwallexTopup={topupInfo?.enable_airwallex_topup}
        airwallexAutoTopupEnabled={topupInfo?.airwallex_autotopup_enabled}
        airwallexCurrencies={topupInfo?.airwallex_currencies}
        onAirwallexPay={handleAirwallexPay}
        airwallexLoading={airwallexProcessing}
      />

      <PaymentConfirmDialog
        open={confirmDialogOpen}
        onOpenChange={setConfirmDialogOpen}
        onConfirm={handlePaymentConfirm}
        topupAmount={topupAmount}
        paymentAmount={paymentAmount}
        paymentMethod={selectedPaymentMethod}
        calculating={calculating}
        processing={processing || pancakeProcessing}
        discountRate={getDiscountRate()}
        usdExchangeRate={effectiveUsdExchangeRate}
      />

      <CreemConfirmDialog
        open={creemDialogOpen}
        onOpenChange={setCreemDialogOpen}
        onConfirm={handleCreemConfirm}
        product={selectedCreemProduct}
        processing={creemProcessing}
      />
    </>
  )
}
