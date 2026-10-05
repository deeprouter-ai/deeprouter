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
  /** What to say to the AI once it is set up — shown under "how to use it". */
  example: string
}

/** Order is the order of the home grid: the class demo (video) first. */
export const SIMPLE_PURPOSES: SimplePurpose[] = [
  {
    id: 'video',
    icon: Clapperboard,
    title: 'Make a video',
    blurb: 'Describe a scene, get a short clip',
    example:
      'Make a video: a cup of milk tea spinning in the sunlight, 10 seconds',
  },
  {
    id: 'image',
    icon: Image,
    title: 'Make a picture',
    blurb: 'Posters, covers, illustrations',
    example: 'Make a poster: summer special, iced lemon tea, bright colours',
  },
  {
    id: 'chat',
    icon: MessageCircle,
    title: 'Chat & write',
    blurb: 'Write, translate, summarise',
    example: 'Write a short shop notice about our new summer opening hours',
  },
  {
    id: 'voice',
    icon: Mic,
    title: 'Voice-over',
    blurb: 'Turn text into speech',
    example: "Make a voice-over: Welcome! Today's special is iced lemon tea",
  },
  {
    id: 'coding',
    icon: Code2,
    title: 'Write code',
    blurb: 'Let your AI build things for you',
    example:
      'Build a one-page website for my shop with the menu and opening hours',
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
