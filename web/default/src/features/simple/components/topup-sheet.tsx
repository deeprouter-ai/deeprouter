// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useTranslation } from 'react-i18next'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '@/components/ui/drawer'
import { RechargePanel } from '@/features/wallet/components/recharge-panel'

/**
 * Top-up as a native bottom sheet. The payment flow inside is the wallet
 * page's own `RechargePanel` — every enabled gateway, same code path.
 */
export function TopupSheet(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onBalanceChange: () => void
}) {
  const { t } = useTranslation()
  return (
    <Drawer open={props.open} onOpenChange={props.onOpenChange}>
      <DrawerContent className='mx-auto max-w-[480px] data-[vaul-drawer-direction=bottom]:max-h-[90dvh]'>
        <DrawerHeader>
          <DrawerTitle className='text-lg'>{t('Add credit')}</DrawerTitle>
          <DrawerDescription>
            {t('Pick an amount and pay. Your balance updates right after.')}
          </DrawerDescription>
        </DrawerHeader>
        <div className='overflow-y-auto px-4 pb-[calc(1rem+env(safe-area-inset-bottom))]'>
          <RechargePanel onBalanceChange={props.onBalanceChange} />
        </div>
      </DrawerContent>
    </Drawer>
  )
}
