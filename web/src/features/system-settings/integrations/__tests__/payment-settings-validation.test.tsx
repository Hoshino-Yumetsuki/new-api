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
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { PaymentSettingsSection } from '../payment-settings-section'

let actionsContainer: HTMLDivElement

afterEach(() => {
  cleanup()
  actionsContainer?.remove()
})

function renderSettings(name: string) {
  actionsContainer = document.createElement('div')
  document.body.append(actionsContainer)
  render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: {
            queries: { retry: false },
            mutations: { retry: false },
          },
        })
      }
    >
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <PaymentSettingsSection
          defaultValues={{
            EpayChannels: JSON.stringify([
              {
                name,
                pay_address: 'https://pay.example.com',
                epay_id: '10001',
                epay_key: '',
              },
            ]),
            Price: 7.3,
            MinTopUp: 1,
            CustomCallbackAddress: '',
            PayMethods: '[]',
            AmountOptions: '[]',
            AmountDiscount: '{}',
            StripeApiSecret: '',
            StripeWebhookSecret: '',
            StripePriceId: '',
            StripeUnitPrice: 1,
            StripeMinTopUp: 1,
            StripePromotionCodesEnabled: false,
            CreemApiKey: '',
            CreemWebhookSecret: '',
            CreemTestMode: false,
            CreemProducts: '[]',
          }}
          waffoDefaultValues={{
            WaffoEnabled: false,
            WaffoApiKey: '',
            WaffoPrivateKey: '',
            WaffoPublicCert: '',
            WaffoSandboxPublicCert: '',
            WaffoSandboxApiKey: '',
            WaffoSandboxPrivateKey: '',
            WaffoSandbox: false,
            WaffoMerchantId: '',
            WaffoCurrency: 'USD',
            WaffoUnitPrice: 1,
            WaffoMinTopUp: 1,
            WaffoNotifyUrl: '',
            WaffoReturnUrl: '',
            WaffoPayMethods: '[]',
          }}
          waffoPancakeDefaultValues={{
            WaffoPancakeMerchantID: '',
            WaffoPancakePrivateKey: '',
            WaffoPancakeReturnURL: '',
          }}
          complianceDefaults={{
            confirmed: true,
            termsVersion: 'v1',
            confirmedAt: 0,
            confirmedBy: 1,
          }}
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

describe('payment settings validation visibility', () => {
  test('saving from General reveals a hidden invalid channel', async () => {
    renderSettings('invalid1')
    expect(screen.getByRole('tab', { name: 'General' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save all settings' }))
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: 'Epay' })).toHaveAttribute(
        'aria-selected',
        'true'
      )
    )
    expect(screen.getByLabelText('Channel name')).toBeVisible()
    expect(screen.getByLabelText('Channel name')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(screen.getByLabelText('Channel name')).toHaveAccessibleDescription(
      'Channel names must contain 1–48 English letters only.'
    )
  })

  test('saving a renamed channel without a key reveals the secret error', async () => {
    renderSettings('Alpha')
    fireEvent.click(screen.getByRole('tab', { name: 'Epay' }))
    fireEvent.change(screen.getByLabelText('Channel name'), {
      target: { value: 'Beta' },
    })
    fireEvent.click(screen.getByRole('tab', { name: 'General' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save all settings' }))
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: 'Epay' })).toHaveAttribute(
        'aria-selected',
        'true'
      )
    )
    expect(screen.getByLabelText('Epay secret key')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(
      screen.getByLabelText('Epay secret key')
    ).toHaveAccessibleDescription(
      /New or renamed Epay channels require a secret key\./
    )
  })
})
