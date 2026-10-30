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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import type { PlanRecord } from '../../../types'
import { SubscriptionPurchaseDialog } from '../subscription-purchase-dialog'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const plan: PlanRecord = {
  plan: {
    id: 1,
    title: 'Starter',
    price_amount: 5,
    currency: 'USD',
    duration_unit: 'month',
    duration_value: 1,
    quota_reset_period: 'monthly',
    enabled: true,
    sort_order: 0,
    allow_balance_pay: false,
    allow_wallet_overflow: true,
    max_purchase_per_user: 0,
    total_amount: 100,
  },
}

function renderCheckout() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <SubscriptionPurchaseDialog
        open
        onOpenChange={vi.fn()}
        plan={plan}
        enableOnlineTopUp
        epayMethods={[{ type: 'epay.alipay', name: 'Alipay' }]}
      />
    </QueryClientProvider>
  )
}

function stubNavigation() {
  const location = { href: 'https://app.test/subscriptions' }
  const fakeWindow = Object.create(window)
  Object.defineProperty(fakeWindow, 'location', {
    configurable: true,
    value: location,
  })
  vi.stubGlobal('window', fakeWindow)
  return location
}

function mockPaymentResponse(response: Record<string, unknown>) {
  vi.spyOn(api, 'post').mockResolvedValue({ data: response } as never)
}

async function clickPay() {
  const button = await screen.findByRole('button', { name: /^Pay$/ })
  await waitFor(() => expect(button).not.toBeDisabled())
  fireEvent.click(button)
  await waitFor(() => expect(button).not.toBeDisabled())
}

describe('subscription Epay checkout responses', () => {
  test('navigates directly for a successful redirect response', async () => {
    renderCheckout()
    const location = stubNavigation()
    mockPaymentResponse({
      success: true,
      message: 'success',
      url: 'https://pay.example/qr',
      redirect: true,
    })
    const submitForm = vi
      .spyOn(HTMLFormElement.prototype, 'submit')
      .mockImplementation(() => undefined)

    await clickPay()

    expect(location.href).toBe('https://pay.example/qr')
    expect(submitForm).not.toHaveBeenCalled()
  })

  test('keeps standard successful responses on the POST form path', async () => {
    renderCheckout()
    const location = stubNavigation()
    const url = 'https://pay.example/submit.php'
    mockPaymentResponse({
      success: true,
      message: 'success',
      url,
      data: { pid: '1', sign: 'signed' },
    })
    const submitForm = vi
      .spyOn(HTMLFormElement.prototype, 'submit')
      .mockImplementation(function (this: HTMLFormElement) {
        expect(this.getAttribute('action')).toBe(url)
        expect(this.method).toBe('post')
        expect(this.target).toBe('_blank')
        expect(this.elements).toHaveLength(2)
      })

    await clickPay()

    expect(location.href).toBe('https://app.test/subscriptions')
    expect(submitForm).toHaveBeenCalledOnce()
  })

  test('does not navigate or submit when the payment response is unsuccessful', async () => {
    renderCheckout()
    const location = stubNavigation()
    mockPaymentResponse({ success: false, message: 'error', data: {} })
    const submitForm = vi
      .spyOn(HTMLFormElement.prototype, 'submit')
      .mockImplementation(() => undefined)

    await clickPay()

    expect(location.href).toBe('https://app.test/subscriptions')
    expect(submitForm).not.toHaveBeenCalled()
  })
})
