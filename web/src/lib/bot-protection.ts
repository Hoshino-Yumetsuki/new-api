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

export type BotProtectionProvider = 'turnstile' | 'capjs' | null

export type BotProtectionConfig = {
  enabled: boolean
  provider: BotProtectionProvider
  turnstileSiteKey: string
  capApiEndpoint: string
}

type StatusLike = Record<string, unknown> | null | undefined

export function getBotProtectionFromStatus(
  status: StatusLike
): BotProtectionConfig {
  const botEnabled =
    status?.bot_protection_enabled === true ||
    !!(status?.capjs_check || status?.turnstile_check)
  const providerRaw = status?.bot_protection_provider as string | undefined
  const capApiEndpoint = String(status?.capjs_api_endpoint || '').trim()
  const useCap =
    providerRaw === 'capjs' ||
    (!providerRaw && !!status?.capjs_check && capApiEndpoint.length > 0)
  const useTurnstile =
    providerRaw === 'turnstile' ||
    (!providerRaw &&
      !status?.capjs_check &&
      !!(status?.turnstile_check && status?.turnstile_site_key))
  const isCapJsEnabled = Boolean(
    botEnabled && useCap && capApiEndpoint.length > 0
  )
  const isTurnstileEnabled = Boolean(
    botEnabled && useTurnstile && status?.turnstile_site_key
  )
  const enabled = isCapJsEnabled || isTurnstileEnabled
  let provider: BotProtectionProvider = null
  if (isCapJsEnabled) {
    provider = 'capjs'
  } else if (isTurnstileEnabled) {
    provider = 'turnstile'
  }
  return {
    enabled,
    provider,
    turnstileSiteKey: String(status?.turnstile_site_key || ''),
    capApiEndpoint,
  }
}

/** Whether an API error means the client should show the bot-protection modal. */
export function shouldTriggerBotProtection(
  botProtectionEnabled: boolean,
  message?: string
): boolean {
  if (!botProtectionEnabled) return false
  if (typeof message !== 'string' || message.length === 0) return true
  return (
    message.includes('Turnstile') ||
    message.includes('Cap.js') ||
    message.includes('人机验证') ||
    message.includes('token 为空')
  )
}