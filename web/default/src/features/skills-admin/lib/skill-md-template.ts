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

// PRD §8.6 — prefilled scaffold for the SKILL.md textarea when uploading a
// new version, so every skill's SKILL.md follows the same structure.
export const SKILL_MD_TEMPLATE = `# Skill Name

## Description
Brief description of what this skill does.

## Usage
How to invoke this skill and what input it expects.

## Instructions
(System prompt for Claude — write the actual behavior directives here.)

## Output Format
Description of the expected output format and structure.
`

export const DEEPROUTER_ROUTING_ENDPOINT =
  'https://deeprouter.co/v1/routing/chat/completions'

// PRD §9 stage-1 required fields, prefilled with this skill's slug/version.
export function manifestTemplate(slug: string, version: string): string {
  return JSON.stringify(
    {
      slug,
      version,
      requires_deeprouter_key: true,
      deeprouter_routing_endpoint: DEEPROUTER_ROUTING_ENDPOINT,
    },
    null,
    2
  )
}
