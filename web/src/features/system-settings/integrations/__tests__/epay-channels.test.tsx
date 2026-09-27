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
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, test } from 'vitest'

import { getEpayChannelsError, getPaymentMethodError } from '../epay-channels'
import { EpayChannelsEditor } from '../epay-channels-editor'

const savedChannel = {
  name: 'Alpha',
  pay_address: 'https://pay.example.com',
  epay_id: '10001',
  epay_key: '',
}

function ChannelEditor() {
  const [value, setValue] = useState(JSON.stringify([savedChannel]))
  return <EpayChannelsEditor value={value} onChange={setValue} />
}

describe('Epay channel validation', () => {
  test.each([
    '',
    'alpha1',
    'alpha-beta',
    'alpha.beta',
    'Alpha\n',
    '支付',
    'A'.repeat(49),
  ])('rejects invalid channel name %s before saving', (name) => {
    expect(
      getEpayChannelsError(JSON.stringify([{ ...savedChannel, name }]))
    ).toBeDefined()
  })

  test('rejects duplicate names without folding case', () => {
    expect(
      getEpayChannelsError(JSON.stringify([savedChannel, savedChannel]))
    ).toBeDefined()
    expect(
      getEpayChannelsError(
        JSON.stringify([savedChannel, { ...savedChannel, name: 'alpha' }])
      )
    ).toBeUndefined()
  })

  test('retains blank secrets only for names already saved', () => {
    const savedNames = new Set(['Alpha'])
    expect(
      getEpayChannelsError(JSON.stringify([savedChannel]), savedNames)
    ).toBeUndefined()
    expect(
      getEpayChannelsError(
        JSON.stringify([{ ...savedChannel, name: 'Beta' }]),
        savedNames
      )
    ).toBeDefined()
    expect(
      getEpayChannelsError(
        JSON.stringify([
          { ...savedChannel, name: 'Beta', epay_key: 'new-secret' },
        ]),
        savedNames
      )
    ).toBeUndefined()
    expect(getEpayChannelsError('[]', savedNames)).toBeUndefined()
  })

  test('accepts custom upstream types while rejecting missing prefixes and oversized identifiers', () => {
    expect(getPaymentMethodError('Alpha.bank_card')).toBeUndefined()
    expect(getPaymentMethodError(`Alpha.${'x'.repeat(249)}`)).toBeUndefined()
    expect(getPaymentMethodError(`Alpha.${'x'.repeat(250)}`)).toBeDefined()
    expect(getPaymentMethodError(`Alpha.${'支'.repeat(83)}`)).toBeUndefined()
    expect(getPaymentMethodError(`Alpha.${'支'.repeat(84)}`)).toBeDefined()
    expect(getPaymentMethodError('alipay')).toBeDefined()
    expect(getPaymentMethodError('Alpha.')).toBeDefined()
    expect(getPaymentMethodError('Alpha.wx.pay')).toBeUndefined()
    for (const type of ['stripe', 'creem', 'waffo', 'waffo_pancake']) {
      expect(getPaymentMethodError(type)).toBeUndefined()
    }
  })

  test.each([
    'https://user:password@pay.example.com',
    'https://pay.example.com/#callback',
  ])('rejects unsupported endpoint %s', (pay_address) => {
    expect(
      getEpayChannelsError(JSON.stringify([{ ...savedChannel, pay_address }]))
    ).toBeDefined()
  })
})

describe('Epay channel editing', () => {
  test('typing a channel name retains focus across controlled updates', async () => {
    const user = userEvent.setup()
    render(<ChannelEditor />)
    const name = screen.getByLabelText('Channel name')
    await user.click(name)
    await user.clear(name)
    await user.type(name, 'Beta')
    expect(name).toHaveValue('Beta')
    expect(name).toHaveFocus()
  })

  test('channel validation errors are described by the affected inputs', () => {
    render(
      <EpayChannelsEditor
        value={JSON.stringify([savedChannel])}
        onChange={() => {}}
        error='Channel names must contain 1–48 English letters only.'
      />
    )
    expect(screen.getByLabelText('Channel name')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(screen.getByLabelText('Channel name')).toHaveAccessibleDescription(
      'Channel names must contain 1–48 English letters only.'
    )
  })
  test('adding and removing rows keeps the remaining channel and its unsaved key intact', () => {
    render(<ChannelEditor />)
    expect(screen.getByLabelText('Epay secret key')).toHaveValue('')
    fireEvent.click(screen.getByRole('button', { name: 'Add Epay channel' }))
    const second = within(screen.getByRole('group', { name: 'Epay channel 2' }))
    fireEvent.change(second.getByLabelText('Channel name'), {
      target: { value: 'Beta' },
    })
    fireEvent.change(second.getByLabelText('Epay secret key'), {
      target: { value: 'new-secret' },
    })
    const remainingInput = second.getByLabelText('Channel name')
    remainingInput.focus()
    fireEvent.click(
      within(screen.getByRole('group', { name: 'Epay channel 1' })).getByRole(
        'button',
        { name: 'Remove channel' }
      )
    )
    expect(screen.getByLabelText('Channel name')).toHaveValue('Beta')
    expect(screen.getByLabelText('Epay secret key')).toHaveValue('new-secret')
    expect(remainingInput).toHaveFocus()
    fireEvent.click(screen.getByRole('button', { name: 'Remove channel' }))
    expect(screen.queryByLabelText('Channel name')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Add Epay channel' }))
    expect(screen.getByLabelText('Epay secret key')).toHaveValue('')
  })
})
