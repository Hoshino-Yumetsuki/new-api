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
import { describe, test } from 'node:test'

import { nextApiKeyGroupValue } from '../api-key-group-selection'

describe('API key group selection', () => {
  test('clears the group when the selected group is clicked again', () => {
    assert.equal(nextApiKeyGroupValue('vip', 'vip'), '')
  })

  test('selects a different group when it is clicked', () => {
    assert.equal(nextApiKeyGroupValue('vip', 'default'), 'default')
  })
})
