// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useTranslation } from 'react-i18next'
import { useIsMobile } from '@/hooks/use-mobile'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from '@/components/ui/drawer'
import { RechargePanel } from '@/features/wallet/components/recharge-panel'

export function TopupSheet(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onBalanceChange: () => void
}) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const title = t('Add credit')
  const description = t(
    'Pick an amount and pay. Your balance updates right after.'
  )
  const panel = <RechargePanel onBalanceChange={props.onBalanceChange} />

  if (!isMobile) {
    return (
      <Dialog open={props.open} onOpenChange={props.onOpenChange}>
        <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>
          {panel}
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Drawer open={props.open} onOpenChange={props.onOpenChange}>
      <DrawerContent className='mx-auto max-w-[480px] data-[vaul-drawer-direction=bottom]:max-h-[90dvh]'>
        <DrawerHeader>
          <DrawerTitle className='text-lg'>{title}</DrawerTitle>
          <DrawerDescription>{description}</DrawerDescription>
        </DrawerHeader>
        <div className='overflow-y-auto px-4 pb-[calc(1rem+env(safe-area-inset-bottom))]'>
          {panel}
        </div>
      </DrawerContent>
    </Drawer>
  )
}
