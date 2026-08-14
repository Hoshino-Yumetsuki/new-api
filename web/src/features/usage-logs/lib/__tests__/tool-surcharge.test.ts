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

import type { LogOtherData } from '../../types'
import { getCacheHitRate, hasToolSurcharge } from '../format'

describe('cache hit rate', () => {
  test('uses cache reads over total input tokens', () => {
    assert.equal((getCacheHitRate(120632, 112128) * 100).toFixed(1), '93.0')
    assert.equal((getCacheHitRate(142007, 133632) * 100).toFixed(1), '94.1')
  })

  test('returns zero when total input or cache reads are unavailable', () => {
    assert.equal(getCacheHitRate(0, 0), 0)
    assert.equal(getCacheHitRate(0, 10), 0)
  })

  test('normalizes invalid values and caps inconsistent data', () => {
    assert.equal(getCacheHitRate(Number.NaN, 10), 0)
    assert.equal(getCacheHitRate(10, Number.POSITIVE_INFINITY), 0)
    assert.equal(getCacheHitRate(10, -1), 0)
    assert.equal(getCacheHitRate(100, 200), 1)
  })
})

describe('tool surcharge detection', () => {
  test('shows the marker for a charged structured tool surcharge', () => {
    assert.equal(
      hasToolSurcharge({
        tool_surcharges: [{ name: 'lookup_customer', count: 2, price: 5 }],
      }),
      true
    )
  })

  const legacyCases: Array<{
    name: string
    other: LogOtherData
  }> = [
    {
      name: 'Web Search',
      other: {
        web_search: true,
        web_search_call_count: 1,
        web_search_price: 10,
      },
    },
    {
      name: 'File Search',
      other: {
        file_search: true,
        file_search_call_count: 2,
        file_search_price: 2.5,
      },
    },
    {
      name: 'Image Generation',
      other: {
        image_generation_call: true,
        image_generation_call_price: 0.04,
      },
    },
  ]

  for (const scenario of legacyCases) {
    test(`keeps the marker visible for legacy ${scenario.name} charges`, () => {
      assert.equal(hasToolSurcharge(scenario.other), true)
    })
  }

  test('hides the marker when surcharge entries are empty or not chargeable', () => {
    const invalidCases: Array<LogOtherData | null> = [
      null,
      {},
      { tool_surcharges: [] },
      {
        tool_surcharges: [{ name: 'lookup_customer', count: 0, price: 5 }],
      },
      {
        tool_surcharges: [{ name: 'lookup_customer', count: 1, price: 0 }],
      },
      {
        tool_surcharges: [{ name: ' ', count: 1, price: 5 }],
      },
      {
        web_search: true,
        web_search_call_count: 1,
        web_search_price: 0,
      },
      {
        image_generation_call: false,
        image_generation_call_price: 0.04,
      },
    ]

    for (const other of invalidCases) {
      assert.equal(hasToolSurcharge(other), false)
    }
  })
})
