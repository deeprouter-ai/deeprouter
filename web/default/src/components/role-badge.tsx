/*
RoleBadge — small chip rendered next to the logo in the top app bar when
the logged-in user has elevated privileges. Lets the operator tell at a
glance whether they're signed in as Super Admin, Admin, or a regular user
(no chip shown for regular users to keep the UI uncluttered).

Reads role from useAuthStore. Lookup uses getRoleLabel which is i18n-aware,
so the label translates with the active locale.
*/
import { useTranslation } from 'react-i18next'
import { ShieldCheck, Shield } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { ROLE, getRoleLabel } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'

export interface RoleBadgeProps {
  className?: string
}

export function RoleBadge({ className }: RoleBadgeProps) {
  useTranslation() // ensure component re-renders when language changes
  const role = useAuthStore((state) => state.auth.user?.role)

  // Only surface for admins. Regular users / guests see nothing.
  if (role !== ROLE.ADMIN && role !== ROLE.SUPER_ADMIN) {
    return null
  }

  const isSuper = role === ROLE.SUPER_ADMIN
  const Icon = isSuper ? ShieldCheck : Shield

  return (
    <Badge
      variant={isSuper ? 'default' : 'outline'}
      className={cn(
        'ms-1 hidden h-5 gap-1 px-1.5 sm:inline-flex',
        // Super Admin: charcoal pill with white text + Lovable inset shadow
        // Admin: outline chip with warm border
        isSuper &&
          'shadow-[inset_0_1px_0_rgb(255_255_255/0.18),inset_0_0_0_1px_rgb(0_0_0/0.2),0_1px_2px_rgb(0_0_0/0.06)]',
        className
      )}
      aria-label={getRoleLabel(role)}
      title={getRoleLabel(role)}
    >
      <Icon className='size-3' aria-hidden='true' />
      <span>{getRoleLabel(role)}</span>
    </Badge>
  )
}
