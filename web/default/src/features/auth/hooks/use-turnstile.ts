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
import i18next from 'i18next'
import { useState } from 'react'
import { toast } from 'sonner'

import { useStatus } from '@/hooks/use-status'

export type BotProtectionProvider = 'turnstile' | 'capjs' | null

/**
 * Hook for managing Turnstile or Cap.js bot protection on auth forms.
 */
export function useTurnstile() {
  const { status } = useStatus()
  const [turnstileToken, setTurnstileToken] = useState('')

  const botEnabled =
    status?.bot_protection_enabled === true ||
    !!(status?.capjs_check || status?.turnstile_check)
  const providerRaw = status?.bot_protection_provider as string | undefined
  const capApiEndpoint = (status?.capjs_api_endpoint as string | undefined) || ''
  const useCap =
    providerRaw === 'capjs' ||
    (!providerRaw && status?.capjs_check && capApiEndpoint.trim().length > 0)
  const useTurnstile =
    providerRaw === 'turnstile' ||
    (!providerRaw &&
      !status?.capjs_check &&
      !!(status?.turnstile_check && status?.turnstile_site_key))

  const isCapJsEnabled = botEnabled && useCap && capApiEndpoint.trim().length > 0
  const isTurnstileEnabled =
    botEnabled && useTurnstile && !!(status?.turnstile_site_key as string)
  const isBotProtectionEnabled = isCapJsEnabled || isTurnstileEnabled
  const provider: BotProtectionProvider = isCapJsEnabled
    ? 'capjs'
    : isTurnstileEnabled
      ? 'turnstile'
      : null
  const turnstileSiteKey = (status?.turnstile_site_key as string) || ''

  const validateTurnstile = (): boolean => {
    if (isBotProtectionEnabled && !turnstileToken) {
      toast.info(
        i18next.t('Please wait a moment, human check is initializing...')
      )
      return false
    }
    return true
  }

  return {
    isTurnstileEnabled: isBotProtectionEnabled,
    isBotProtectionEnabled,
    provider,
    turnstileSiteKey,
    capApiEndpoint,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
  }
}