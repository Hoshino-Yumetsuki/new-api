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
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  getGroupNames,
  getVisualGroupRenames,
  isValidGroupRenames,
} from '../group-rename-utils'

describe('group rename utilities', () => {
  it('collects canonical group keys across the editable maps', () => {
    assert.deepEqual(
      getGroupNames({
        GroupRatio: '{"standard":1,"vip":0.8}',
        UserUsableGroups: '{"vip":"VIP"}',
        TopupGroupRatio: '{"partner":1.2}',
      }),
      ['partner', 'standard', 'vip']
    )
  })

  it('collapses a visual rename and removes it when reverted', () => {
    assert.deepEqual(
      getVisualGroupRenames([
        { originalName: 'legacy', name: 'pro' },
        { originalName: 'standard', name: 'standard' },
      ]),
      [{ from: 'legacy', to: 'pro' }]
    )
    assert.deepEqual(
      getVisualGroupRenames([{ originalName: 'legacy', name: 'legacy' }]),
      []
    )
  })

  it('accepts only one-to-one mappings between removed and added keys', () => {
    assert.equal(
      isValidGroupRenames(
        ['legacy', 'premium'],
        ['pro', 'enterprise'],
        [
          { from: 'legacy', to: 'pro' },
          { from: 'premium', to: 'enterprise' },
        ]
      ),
      true
    )
    assert.equal(
      isValidGroupRenames(
        ['legacy', 'premium'],
        ['pro', 'enterprise'],
        [
          { from: 'legacy', to: 'pro' },
          { from: 'premium', to: 'pro' },
        ]
      ),
      false
    )
  })
})
