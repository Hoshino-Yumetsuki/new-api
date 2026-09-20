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
import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  test,
} from 'vitest'

import { useSystemConfigStore } from '@/stores/system-config-store'

import { CacheHitRateCell } from '../cache-hit-rate-cell'
import { LogCostDisplay } from '../log-cost-display'

function renderCost(
  props: React.ComponentProps<typeof LogCostDisplay>
): ReturnType<typeof render> {
  return render(<LogCostDisplay {...props} />)
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
  test('shows Anthropic cache reads over uncached plus read input', () => {
    const rendered = renderCacheHitRate(
      24,
      28,
      16177,
      'billing-usage-anthropic'
    )

    expect(rendered.container).toHaveTextContent('53.8%')
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
      Wallet: 'Wallet',
      'Includes tool-call surcharge': 'Includes tool-call surcharge',
    })
  })

  beforeEach(() => {
    useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  })

  afterEach(() => {
    useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
    localStorage.clear()
  })

  test.each([
    { consumed: 12500, expected: '$0.025' },
    { consumed: 0, expected: '$0' },
    { consumed: 1, expected: '$0.000002' },
    { consumed: undefined, expected: '$0.01' },
  ])(
    'shows subscription deduction $consumed without hover, falling back only when absent',
    ({ consumed, expected }) => {
      renderCost({
        quota: 5000,
        other: {
          billing_source: 'subscription',
          subscription_consumed: consumed,
        },
      })

      expect(screen.getByText(expected)).toBeVisible()
      expect(screen.getByText('Subscription')).toBeVisible()
      expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
    }
  )

  test('shows wallet cost and source without using subscription metadata', () => {
    renderCost({
      quota: 5000,
      other: { billing_source: 'wallet', subscription_consumed: 12500 },
      showWalletSource: true,
    })

    expect(screen.getByText('$0.01')).toBeVisible()
    expect(screen.getByText('Wallet')).toBeVisible()
    expect(screen.queryByText('Subscription')).not.toBeInTheDocument()
  })

  test('hides the wallet label when subscriptions are unavailable', () => {
    renderCost({
      quota: 5000,
      other: { billing_source: 'wallet' },
      showWalletSource: false,
    })

    expect(screen.getByText('$0.01')).toBeVisible()
    expect(screen.queryByText('Wallet')).not.toBeInTheDocument()
  })

  test('keeps legacy cost visible without inventing a funding source', () => {
    renderCost({ quota: 5000, other: null })

    expect(screen.getByText('$0.01')).toBeVisible()
    expect(screen.queryByText('Wallet')).not.toBeInTheDocument()
    expect(screen.queryByText('Subscription')).not.toBeInTheDocument()
  })

  test('keeps a large amount unabridged above the funding source', () => {
    const rendered = renderCost({
      quota: 2147483647,
      other: { billing_source: 'subscription' },
    })

    const amount = screen.getByText('$4,294.9673')
    expect(amount).toBeVisible()
    expect(amount).toHaveClass('tabular-nums', 'whitespace-nowrap')
    expect(rendered.container.firstElementChild).toHaveClass('flex-col')
    expect(
      amount.compareDocumentPosition(screen.getByText('Subscription')) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).not.toBe(0)
  })

  test('keeps the regular cost visible and adds an accessible surcharge marker', () => {
    renderCost({
      quota: 12500,
      other: {
        tool_surcharges: [{ name: 'lookup_customer', count: 1, price: 5 }],
      },
    })

    expect(screen.getByText('$0.025')).toBeVisible()
    const marker = screen.getByRole('img', {
      name: 'Includes tool-call surcharge',
    })
    expect(marker).toHaveAttribute('data-tool-surcharge-indicator', 'true')
    expect(marker).toHaveAttribute('tabindex', '0')
  })

  test('shows subscription cost and source alongside the legacy surcharge marker', () => {
    renderCost({
      quota: 5000,
      other: {
        billing_source: 'subscription',
        web_search: true,
        web_search_call_count: 1,
        web_search_price: 10,
      },
    })

    expect(screen.getByText('$0.01')).toBeVisible()
    expect(screen.getByText('Subscription')).toBeVisible()
    expect(
      screen.getByRole('img', { name: 'Includes tool-call surcharge' })
    ).toHaveAttribute('data-tool-surcharge-indicator', 'true')
  })
})
