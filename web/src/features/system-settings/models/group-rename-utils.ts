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
import type { GroupRename } from '../types'
import { safeJsonParse } from '../utils/json-parser'

export type GroupSettingsValues = {
  GroupRatio: string
  TopupGroupRatio: string
  UserUsableGroups: string
}

export function getGroupNames(values: Partial<GroupSettingsValues>): string[] {
  const maps = [
    safeJsonParse<Record<string, unknown>>(values.GroupRatio ?? '{}', {
      fallback: {},
      silent: true,
    }),
    safeJsonParse<Record<string, unknown>>(values.UserUsableGroups ?? '{}', {
      fallback: {},
      silent: true,
    }),
    safeJsonParse<Record<string, unknown>>(values.TopupGroupRatio ?? '{}', {
      fallback: {},
      silent: true,
    }),
  ]
  return [...new Set(maps.flatMap((map) => Object.keys(map)))].sort()
}

export function isValidGroupRenames(
  removed: string[],
  added: string[],
  renames: GroupRename[]
) {
  return (
    new Set(renames.map(({ from }) => from)).size === renames.length &&
    new Set(renames.map(({ to }) => to)).size === renames.length &&
    renames.every(
      ({ from, to }) => removed.includes(from) && added.includes(to)
    )
  )
}

export function getVisualGroupRenames(
  rows: Array<{ originalName?: string; name: string }>
): GroupRename[] {
  return rows.flatMap((row) => {
    const to = row.name.trim()
    return row.originalName && to && row.originalName !== to
      ? [{ from: row.originalName, to }]
      : []
  })
}
