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
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import type React from 'react'
import { beforeAll, describe, expect, test } from 'vitest'

import { formatLogQuota } from '@/lib/format'

import { CacheHitRateCell } from '../cache-hit-rate-cell'
import { LogCostDisplay } from '../log-cost-display'

function renderCost(
  props: React.ComponentProps<typeof LogCostDisplay>
): ReturnType<typeof render> {
  return render(<LogCostDisplay {...props} />)
}

function normalizedText(value: string | null): string {
  return (value ?? '').replaceAll(/\s/g, '')
}

function renderCacheHitRate(
  promptTokens: number,
  cacheReadTokens: number,
  cacheWriteTokens?: number,
  billingPath?: string
) {
  return render(
    <CacheHitRateCell
      promptTokens={promptTokens}
      cacheReadTokens={cacheReadTokens}
      cacheWriteTokens={cacheWriteTokens}
      billingPath={billingPath}
    />
  )
}

describe('cache hit rate display', () => {
  test('shows cache reads over total input with one decimal and emerald color', () => {
    const rendered = renderCacheHitRate(120632, 112128)
    const value = rendered.container.querySelector('span')

    expect(value).toHaveTextContent('93.0%')
    expect(value).toHaveStyle({ color: 'var(--color-emerald-600)' })
  })
  test('shows the Anthropic cache rate from uncached, read, and created input', () => {
    const rendered = renderCacheHitRate(
      2,
      97147,
      11269,
      'billing-usage-anthropic'
    )

    expect(rendered.container).toHaveTextContent('89.6%')
  })

  test('shows an em dash when the cache rate is zero', () => {
    const rendered = renderCacheHitRate(100, 0)

    expect(rendered.container).toHaveTextContent('—')
  })

  test('caps inconsistent cache tokens at 100 percent', () => {
    const rendered = renderCacheHitRate(100, 200)

    expect(rendered.container).toHaveTextContent('100.0%')
  })
})

describe('log cost display', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      Subscription: 'Subscription',
      'Deducted by subscription': 'Deducted by subscription',
      'Includes tool-call surcharge': 'Includes tool-call surcharge',
    })
  })

  test('keeps the regular cost visible and adds an accessible surcharge marker', () => {
    const rendered = renderCost({
      quota: 12500,
      other: {
        tool_surcharges: [{ name: 'lookup_customer', count: 1, price: 5 }],
      },
    })

    expect(
      normalizedText(rendered.container.textContent).includes(
        normalizedText(formatLogQuota(12500))
      )
    ).toBe(true)
    const marker = screen.getByRole('img', {
      name: 'Includes tool-call surcharge',
    })
    expect(marker).toHaveAttribute('data-tool-surcharge-indicator', 'true')
    expect(marker).toHaveAttribute('tabindex', '0')
  })

  test('preserves the subscription badge and adds the same legacy surcharge marker', () => {
    renderCost({
      quota: 5000,
      other: {
        billing_source: 'subscription',
        web_search: true,
        web_search_call_count: 1,
        web_search_price: 10,
      },
    })

    expect(screen.getByText('Subscription')).toBeInTheDocument()
    expect(
      screen.getByRole('img', { name: 'Includes tool-call surcharge' })
    ).toHaveAttribute('data-tool-surcharge-indicator', 'true')
  })
})
