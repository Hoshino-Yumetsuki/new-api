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
import { render, screen, within } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { DynamicPricingBreakdown } from '../dynamic-pricing-breakdown'

describe('dynamic pricing breakdown', () => {
  test('merges time multipliers into the tier table with adjusted prices', () => {
    render(
      <DynamicPricingBreakdown
        billingExpr='tier("time_based", p * 1.5 + c * 4.5 + cr * 0.05)'
        requestRules={[
          {
            cond: 'hour("Asia/Shanghai") >= 9',
            multiplier: 2,
            matched: false,
          },
        ]}
      />
    )

    expect(screen.queryByText('Conditional multipliers')).not.toBeInTheDocument()

    const table = screen.getByRole('table')
    expect(within(table).getByRole('columnheader', { name: 'Multiplier' })).toBeInTheDocument()

    const rows = within(table).getAllByRole('row')
    expect(rows).toHaveLength(3)
    expect(within(rows[1]).getByText('1x')).toBeInTheDocument()
    expect(within(rows[1]).getByText('$1.5000')).toBeInTheDocument()
    expect(within(rows[1]).getByText('$4.5000')).toBeInTheDocument()
    expect(within(rows[2]).getByText('2x')).toBeInTheDocument()
    expect(within(rows[2]).getByText('$3.0000')).toBeInTheDocument()
    expect(within(rows[2]).getByText('$9.0000')).toBeInTheDocument()
  })
})
