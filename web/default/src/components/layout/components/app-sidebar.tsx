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
import { useMemo, useState } from 'react'
import { useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Search, X } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { useLayout } from '@/context/layout-provider'
import { useSidebarConfig } from '@/hooks/use-sidebar-config'
import { useSidebarData } from '@/hooks/use-sidebar-data'
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
  SidebarInput,
  SidebarRail,
} from '@/components/ui/sidebar'
import { Button } from '@/components/ui/button'
import { getNavGroupsForPath } from '../lib/workspace-registry'
import type { NavGroup as NavGroupType, NavItem } from '../types'
import { NavGroup } from './nav-group'

/**
 * Application sidebar component
 * Fetches corresponding navigation menu from workspace registry based on current path
 * Dynamically filters navigation items based on backend SidebarModulesAdmin configuration
 *
 * Automatically matches workspace configuration for current path through workspace registry system
 * Adding new workspaces only requires registration in workspace-registry.ts
 */
export function AppSidebar() {
  const { t } = useTranslation()
  const { collapsible, variant } = useLayout()
  const { pathname } = useLocation()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const sidebarData = useSidebarData()
  const [searchQuery, setSearchQuery] = useState('')

  // Get navigation group configuration corresponding to current path from workspace registry
  const allNavGroups = getNavGroupsForPath(pathname, t) || sidebarData.navGroups

  // Filter sidebar navigation items based on backend configuration
  const configFilteredNavGroups = useSidebarConfig(allNavGroups)

  // Filter navigation groups based on user role
  // Non-Admin users cannot see Admin navigation group
  const roleFilteredNavGroups = useMemo(() => {
    const isAdmin = userRole && userRole >= ROLE.ADMIN
    return configFilteredNavGroups.filter((group) => {
      if (group.id === 'admin') {
        return isAdmin
      }
      return true
    })
  }, [configFilteredNavGroups, userRole])

  // Search filter: case-insensitive substring match on item titles + sub-item
  // titles. While searching, matched sub-items get promoted to flat NavLinks
  // so the user doesn't have to expand a collapsible to see them. Empty
  // groups are hidden.
  const currentNavGroups = useMemo<NavGroupType[]>(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q) return roleFilteredNavGroups

    const matches = (s: string) => s.toLowerCase().includes(q)

    return roleFilteredNavGroups
      .map((group) => {
        const items: NavItem[] = []
        for (const item of group.items) {
          const titleMatches = matches(item.title)
          const hasSubItems = 'items' in item && Array.isArray(item.items)

          if (hasSubItems) {
            // Promote sub-items as flat NavLinks (skip collapsible nesting
            // during search so all matches are visible without expand clicks).
            const subItems = (item as { items: Array<{ title: string; url: string; icon?: React.ElementType; badge?: string }> }).items
            for (const sub of subItems) {
              if (titleMatches || matches(sub.title)) {
                items.push({
                  title: sub.title,
                  url: sub.url,
                  icon: sub.icon,
                  badge: sub.badge,
                } as NavItem)
              }
            }
            continue
          }
          if (titleMatches) items.push(item)
        }
        return items.length > 0 ? { ...group, items } : null
      })
      .filter((g): g is NavGroupType => g !== null)
  }, [roleFilteredNavGroups, searchQuery])

  const trimmedQuery = searchQuery.trim()

  return (
    <Sidebar collapsible={collapsible} variant={variant}>
      <SidebarHeader>
        <div className='relative'>
          <Search
            className='text-muted-foreground pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2'
            aria-hidden='true'
          />
          <SidebarInput
            type='search'
            placeholder={t('Search menu...')}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            aria-label={t('Search menu')}
            className='pr-7 pl-7'
          />
          {searchQuery && (
            <Button
              type='button'
              variant='ghost'
              size='icon'
              onClick={() => setSearchQuery('')}
              aria-label={t('Clear search')}
              className='absolute top-1/2 right-1 size-6 -translate-y-1/2'
            >
              <X className='size-3.5' />
            </Button>
          )}
        </div>
      </SidebarHeader>
      <SidebarContent className='py-2'>
        {currentNavGroups.length === 0 && trimmedQuery ? (
          <p className='text-muted-foreground px-4 py-6 text-center text-xs'>
            {t('No menu items match "{{q}}"', { q: trimmedQuery })}
          </p>
        ) : (
          currentNavGroups.map((props) => {
            const key = props.id || props.title
            return <NavGroup key={key} {...props} />
          })
        )}
      </SidebarContent>
      <SidebarRail />
    </Sidebar>
  )
}
