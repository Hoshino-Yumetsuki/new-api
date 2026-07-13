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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'

import { useStatus } from '@/hooks/use-status'
import {
  getBotProtectionFromStatus,
  type BotProtectionProvider,
} from '@/lib/bot-protection'

export type { BotProtectionProvider }

/**
 * Hook for managing Turnstile or Cap.js bot protection on auth forms.
 */
export function useTurnstile() {
  const { status } = useStatus()
  const [turnstileToken, setTurnstileToken] = useState('')
  const [botProtectionReady, setBotProtectionReady] = useState(false)

  const markBotProtectionReady = useCallback(() => {
    setBotProtectionReady(true)
  }, [])

  const bp = useMemo(() => getBotProtectionFromStatus(status), [status])
  const {
    enabled: isBotProtectionEnabled,
    provider,
    turnstileSiteKey,
    capApiEndpoint,
  } = bp

  useEffect(() => {
    setBotProtectionReady(false)
    setTurnstileToken('')
  }, [isBotProtectionEnabled, provider, capApiEndpoint, turnstileSiteKey])

  const validateTurnstile = (): boolean => {
    if (!isBotProtectionEnabled || turnstileToken) return true
    toast.info(
      i18next.t(
        botProtectionReady
          ? 'Please complete the human verification before continuing.'
          : 'Please wait a moment, human check is initializing...'
      )
    )
    return false
  }

  return {
    isTurnstileEnabled: isBotProtectionEnabled,
    isBotProtectionEnabled,
    provider,
    turnstileSiteKey,
    capApiEndpoint,
    turnstileToken,
    setTurnstileToken,
    markBotProtectionReady,
    validateTurnstile,
  }
}