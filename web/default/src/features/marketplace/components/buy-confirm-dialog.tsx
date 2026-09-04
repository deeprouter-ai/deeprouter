/*
Copyright (C) 2026 DeepRouter

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
*/
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { fetchMarketplaceSkill, marketplaceQueryKeys } from '../api'

interface Props {
  slug: string
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  confirming: boolean
}

/**
 * Purchase confirmation for a paid skill. PRD §8.2: the price shown here is
 * re-fetched from the backend the moment the dialog opens — never the price
 * the page was rendered with — so the user always confirms the amount the
 * admin has set right now.
 */
export function BuyConfirmDialog(props: Props) {
  const { t } = useTranslation()

  const priceQuery = useQuery({
    queryKey: [...marketplaceQueryKeys.detail(props.slug), 'live-price'],
    queryFn: () => fetchMarketplaceSkill(props.slug),
    enabled: props.open,
    staleTime: 0,
    gcTime: 0,
    refetchOnMount: 'always',
  })
  const price = priceQuery.data?.price_usd

  return (
    <ConfirmDialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Confirm purchase')}
      desc={
        priceQuery.isLoading
          ? t('Checking the latest price...')
          : priceQuery.isError || price == null
            ? t('Could not fetch the latest price. Close this dialog and try again.')
            : t(
                'Confirm payment of {{price}}? One purchase unlocks this skill for good — future version updates are free.',
                { price: `$${price.toFixed(2)}` }
              )
      }
      confirmText={t('Confirm and download')}
      cancelBtnText={t('Cancel')}
      disabled={priceQuery.isLoading || priceQuery.isError || price == null}
      isLoading={props.confirming}
      handleConfirm={props.onConfirm}
    />
  )
}
