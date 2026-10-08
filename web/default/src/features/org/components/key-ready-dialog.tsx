// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import { useTranslation } from 'react-i18next'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Input } from '@/components/ui/input'
import { CopyButton } from '@/components/copy-button'
import type { OrgKeyGrant } from '../types'

/** What was just done to the key the dialog is about. */
export type KeyReadyReason = 'created' | 'rotated' | 'assigned'

type KeyReadyDialogProps = {
  /** The key that was just created, given a new value or handed over; null keeps this closed. */
  grant: OrgKeyGrant | null
  reason: KeyReadyReason
  onClose: () => void
}

/**
 * What follows making a key, giving it a new value or handing it to another
 * holder (Enterprise Org PRD D15).
 *
 * A service account's key comes with its value, and this is the one time
 * anybody sees it — so the dialog says plainly that there is no second look,
 * and goes away for one thing only: the button that says the key was saved. A
 * stray Escape or a click outside leaves it where it is. A person's key comes
 * with no value at all: the dialog says how its holder gets it instead, and
 * closes like any other.
 */
export function KeyReadyDialog({
  grant,
  reason,
  onClose,
}: KeyReadyDialogProps) {
  const { t } = useTranslation()
  const value = grant?.value ? `sk-${grant.value}` : ''
  const title = {
    created: t('Key created'),
    rotated: t('The key has a new value'),
    assigned: t('Key assigned'),
  }[reason]

  return (
    <AlertDialog
      open={grant !== null}
      onOpenChange={(open) => !open && !value && onClose()}
    >
      <AlertDialogContent className='sm:!max-w-lg'>
        {grant && (
          <>
            <AlertDialogHeader>
              <AlertDialogTitle>{title}</AlertDialogTitle>
              <AlertDialogDescription>
                {value
                  ? t(
                      'This is the full key of “{{holder}}”. It is shown this once: after you close this window nobody can see it again, and the only way to get one is to replace it.',
                      { holder: grant.holder }
                    )
                  : t(
                      '“{{name}}” is held by {{holder}}. Its value is not shown here: {{holder}} signs in, opens their API keys page and installs it into their tools with one-click setup.',
                      { name: grant.name, holder: grant.holder }
                    )}
              </AlertDialogDescription>
            </AlertDialogHeader>
            {value && (
              <div className='flex items-center gap-2'>
                <Input
                  readOnly
                  value={value}
                  aria-label={t('Full key')}
                  className='font-mono text-xs'
                  onFocus={(e) => e.currentTarget.select()}
                />
                <CopyButton
                  value={value}
                  variant='outline'
                  tooltip={t('Copy key')}
                  aria-label={t('Copy key')}
                />
              </div>
            )}
            <AlertDialogFooter>
              <AlertDialogAction onClick={onClose}>
                {value ? t('I have saved it') : t('Done')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </>
        )}
      </AlertDialogContent>
    </AlertDialog>
  )
}
