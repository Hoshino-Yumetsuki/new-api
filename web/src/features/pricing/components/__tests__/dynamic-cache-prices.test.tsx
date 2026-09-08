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
import { afterEach, beforeEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { PricingModel } from '../../types'
import { ModelCard } from '../model-card'
import { PricingTable } from '../pricing-table'

const originalCurrency = useSystemConfigStore.getState().config.currency

beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'USD' },
  })
})

afterEach(() => {
  useSystemConfigStore.getState().setConfig({ currency: originalCurrency })
})

const model: PricingModel = {
  id: 1,
  model_name: 'dynamic-cache-model',
  quota_type: 0,
  model_ratio: 1,
  completion_ratio: 1,
  enable_groups: ['default'],
  billing_mode: 'tiered_expr',
  billing_expr:
    'len <= 1000 ? tier("short", p * 1 + c * 4 + cr * 0.1 + cc * 1.25 + cc1h * 2) : tier("long", p * 3 + c * 2 + cr * 0.3 + cc * 1.5 + cc1h * 0.75)',
}

describe('dynamic cache prices in model square', () => {
  test('card view shows the lowest input, output, and cached price across tiers', () => {
    render(<ModelCard model={model} onClick={() => {}} tokenUnit='M' />)
    const pricing = within(screen.getByRole('group', { name: 'Pricing' }))

    expect(pricing.getByText('Input').parentElement).toHaveTextContent(
      /Input\s*\$1/
    )
    expect(pricing.getByText('Output').parentElement).toHaveTextContent(
      /Output\s*\$2/
    )
    expect(pricing.getByText('Cached').parentElement).toHaveTextContent(
      /Cached\s*\$0\.1/
    )
    expect(
      pricing.queryByText(/Cache Read|Cache Write|1h/)
    ).not.toBeInTheDocument()
  })

  test('table view shows the same lowest input, output, and cached prices', () => {
    render(<PricingTable models={[model]} tokenUnit='M' />)
    const table = screen.getByRole('table')

    expect(within(table).getByText('Input').parentElement).toHaveTextContent(
      /Input\s*1$/
    )
    expect(within(table).getByText('Output').parentElement).toHaveTextContent(
      /Output\s*2$/
    )
    expect(table).toHaveTextContent(/Cached/)
    expect(table).toHaveTextContent(/\$0\.1/)
    expect(table).not.toHaveTextContent(/Cache Read|Cache Write|1h/)
  })
})
