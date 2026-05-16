import { createFileRoute } from '@tanstack/react-router'
import { PricingCheatsheet } from '@/features/help/pricing-cheatsheet'

export const Route = createFileRoute('/_authenticated/help/pricing')({
  component: PricingCheatsheet,
})
