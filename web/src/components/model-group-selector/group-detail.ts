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
export function formatGroupDescription(
  description: string,
  t: (key: string) => string
): string {
  // Only the gateway's built-in descriptions are translation keys.
  switch (description) {
    case 'Default group':
    case 'VIP group':
    case 'User group':
      return t(description)
    default:
      return description
  }
}

export interface GroupDetailSource {
  ratio?: number | string
  desc?: string
  description?: string
}

/**
 * The line under a group name: its description and billing ratio. Built-in
 * descriptions arrive as English source keys, and the auto group has a
 * non-numeric ratio.
 */
export function formatGroupDetail(
  group: GroupDetailSource,
  t: (key: string, options?: Record<string, unknown>) => string
): string {
  const description = group.desc
    ? formatGroupDescription(group.desc, t)
    : (group.description ?? '')
  if (!group.ratio) return description
  const value = typeof group.ratio === 'number' ? group.ratio : t('Auto')
  return `${description} · ${t('Ratio: {{value}}', { value })}`
}
