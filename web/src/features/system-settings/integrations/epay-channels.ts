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
import * as z from 'zod'

const epayChannelSchema = z.object({
  name: z.string(),
  pay_address: z.string(),
  epay_id: z.string(),
  epay_key: z.string(),
})

export type EpayChannel = z.infer<typeof epayChannelSchema>

export function parseEpayChannels(value: string): EpayChannel[] {
  return z.array(epayChannelSchema).parse(JSON.parse(value || '[]'))
}

export function getEpayChannelsError(
  value: string,
  savedNames?: ReadonlySet<string>
): string | undefined {
  let channels: EpayChannel[]
  try {
    channels = parseEpayChannels(value)
  } catch {
    return 'Epay channels must be a JSON array of channel configurations.'
  }
  const names = new Set<string>()
  for (const channel of channels) {
    if (
      !channel.name ||
      channel.name.length > 48 ||
      /[^A-Za-z]/.test(channel.name)
    ) {
      return 'Channel names must contain 1–48 English letters only.'
    }
    if (names.has(channel.name)) {
      return 'Epay channel names must be unique (case-sensitive).'
    }
    names.add(channel.name)
    try {
      const url = new URL(channel.pay_address)
      if (
        (url.protocol !== 'http:' && url.protocol !== 'https:') ||
        url.username ||
        url.password ||
        url.hash
      ) {
        return 'Provide a valid callback URL starting with http:// or https://'
      }
    } catch {
      return 'Provide a valid callback URL starting with http:// or https://'
    }
    if (!channel.epay_id.trim()) return 'Merchant ID is required'
    if (
      savedNames &&
      !savedNames.has(channel.name) &&
      !channel.epay_key.trim()
    ) {
      return 'New or renamed Epay channels require a secret key.'
    }
  }
}

export function getPaymentMethodError(type: string): string | undefined {
  if (['stripe', 'creem', 'waffo', 'waffo_pancake'].includes(type)) return
  if (
    !/^[A-Za-z]{1,48}\.[\s\S]+$/.test(type) ||
    new TextEncoder().encode(type).byteLength > 255
  ) {
    return 'Use channel.type for Epay (for example achannel.wxpay), up to 255 UTF-8 bytes.'
  }
}
