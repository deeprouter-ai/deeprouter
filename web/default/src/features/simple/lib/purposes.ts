// Copyright (C) 2026 DeepRouter
// SPDX-License-Identifier: AGPL-3.0-or-later
import {
  Clapperboard,
  Code2,
  Image,
  MessageCircle,
  Mic,
  type LucideIcon,
} from 'lucide-react'

/** Same ids as the key form's Simple purposes, so keys and cards line up. */
export type SimplePurposeId = 'video' | 'image' | 'chat' | 'voice' | 'coding'

export interface SimplePurpose {
  id: SimplePurposeId
  icon: LucideIcon
  /** English i18n source strings; zh lives in the locale file. */
  title: string
  blurb: string
}

/** Order is the order of the home grid: the class demo (video) first. */
export const SIMPLE_PURPOSES: SimplePurpose[] = [
  {
    id: 'video',
    icon: Clapperboard,
    title: 'Make a video',
    blurb: 'Describe a scene, get a short clip',
  },
  {
    id: 'image',
    icon: Image,
    title: 'Make a picture',
    blurb: 'Posters, covers, illustrations',
  },
  {
    id: 'chat',
    icon: MessageCircle,
    title: 'Chat & write',
    blurb: 'Write, translate, summarise',
  },
  {
    id: 'voice',
    icon: Mic,
    title: 'Voice-over',
    blurb: 'Turn text into speech',
  },
  {
    id: 'coding',
    icon: Code2,
    title: 'Write code',
    blurb: 'Let your AI build things for you',
  },
]

export function findPurpose(id: string): SimplePurpose | undefined {
  return SIMPLE_PURPOSES.find((p) => p.id === id)
}

/**
 * Best-effort purpose of a spend record, from the key that made it. Simple
 * keys are auto-named `my-<purpose>-key`; anything else is just "AI usage".
 */
export function purposeFromKeyName(
  name: string | undefined | null
): SimplePurposeId | undefined {
  const match = /^my-([a-z]+)-key$/.exec(name ?? '')
  return match ? findPurpose(match[1])?.id : undefined
}
