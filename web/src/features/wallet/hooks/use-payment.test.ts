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
import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { PAYMENT_TYPES } from '../constants'
import { requestPaymentAmount, usePayment } from './use-payment'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('payment amount routing', () => {
  test('uses the dedicated Waffo amount calculator', async () => {
    const calls: string[] = []
    const amount = await requestPaymentAmount(120, PAYMENT_TYPES.WAFFO, {
      regular: async () => {
        calls.push('regular')
        return { success: true, data: '1' }
      },
      stripe: async () => {
        calls.push('stripe')
        return { success: true, data: '2' }
      },
      waffo: async (request) => {
        calls.push(`waffo:${request.amount}`)
        return { success: true, data: '18.75' }
      },
      waffoPancake: async () => {
        calls.push('pancake')
        return { success: true, data: '4' }
      },
    })

    expect(amount).toBe(18.75)
    expect(calls).toEqual(['waffo:120'])
  })
})

describe('Epay checkout responses', () => {
  function stubNavigation() {
    const location = { href: 'https://app.test/wallet' }
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

  test('navigates directly for a successful redirect response', async () => {
    const url = 'https://pay.example/qr'
    mockPaymentResponse({ message: 'success', url, redirect: true })
    const { result } = renderHook(() => usePayment())
    const location = stubNavigation()
    const submitForm = vi
      .spyOn(HTMLFormElement.prototype, 'submit')
      .mockImplementation(() => undefined)

    let processed = false
    await act(async () => {
      processed = await result.current.processPayment(20, 'epay.alipay')
    })

    expect(processed).toBe(true)
    expect(location.href).toBe(url)
    expect(submitForm).not.toHaveBeenCalled()
  })

  test('keeps standard successful responses on the POST form path', async () => {
    const url = 'https://pay.example/submit.php'
    const params = { pid: '1', sign: 'signed' }
    mockPaymentResponse({ message: 'success', url, data: params })
    const { result } = renderHook(() => usePayment())
    const location = stubNavigation()
    let submittedForm:
      | { action: string; method: string; target: string; params: object }
      | undefined
    const submitForm = vi
      .spyOn(HTMLFormElement.prototype, 'submit')
      .mockImplementation(function (this: HTMLFormElement) {
        submittedForm = {
          action: this.action,
          method: this.method,
          target: this.target,
          params: Object.fromEntries(new FormData(this)),
        }
      })

    await act(async () => {
      await result.current.processPayment(20, 'epay.alipay')
    })

    expect(location.href).toBe('https://app.test/wallet')
    expect(submitForm).toHaveBeenCalledOnce()
    expect(submittedForm?.action).toBe(url)
    expect(submittedForm?.method).toBe('post')
    expect(submittedForm?.target).toBe('_blank')
    expect(submittedForm?.params).toEqual(params)
  })

  test('does not navigate or submit when the payment response is unsuccessful', async () => {
    mockPaymentResponse({ message: 'error', data: {} })
    const { result } = renderHook(() => usePayment())
    const location = stubNavigation()
    const submitForm = vi
      .spyOn(HTMLFormElement.prototype, 'submit')
      .mockImplementation(() => undefined)

    let processed = true
    await act(async () => {
      processed = await result.current.processPayment(20, 'epay.alipay')
    })

    expect(processed).toBe(false)
    expect(location.href).toBe('https://app.test/wallet')
    expect(submitForm).not.toHaveBeenCalled()
  })
})
