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
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { SimpleBrand } from '../types'

const BRAND_LABELS: Record<SimpleBrand, string> = {
  claude: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini',
  deepseek: 'DeepSeek',
}

type ApiKeyBrandFilterProps = {
  availableBrands: SimpleBrand[]
  value?: SimpleBrand
  onValueChange: (value: SimpleBrand | undefined) => void
}

/**
 * Optional brand-preference chip row shown under the purpose picker.
 * Empty selection ("No preference") means the backend uses the recommended
 * default brand for the chosen purpose.
 */
export function ApiKeyBrandFilter({
  availableBrands,
  value,
  onValueChange,
}: ApiKeyBrandFilterProps) {
  const { t } = useTranslation()
  if (availableBrands.length === 0) return null
  return (
    <div className='flex flex-wrap gap-2'>
      <BrandChip
        label={t('No preference')}
        selected={!value}
        onSelect={() => onValueChange(undefined)}
      />
      {availableBrands.map((brand) => (
        <BrandChip
          key={brand}
          label={BRAND_LABELS[brand] ?? brand}
          selected={value === brand}
          onSelect={() => onValueChange(brand)}
        />
      ))}
    </div>
  )
}

function BrandChip({
  label,
  selected,
  onSelect,
}: {
  label: string
  selected: boolean
  onSelect: () => void
}) {
  return (
    <Button
      type='button'
      size='sm'
      variant={selected ? 'default' : 'outline'}
      onClick={onSelect}
      className={cn('rounded-full px-3 text-xs', selected && 'shadow-sm')}
    >
      {label}
    </Button>
  )
}
