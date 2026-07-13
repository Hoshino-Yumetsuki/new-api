/*
Copyright (C) 2025 QuantumNous

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

/**
 * @param {Record<string, unknown>} status
 */
export function getBotProtectionFromStatus(status = {}) {
  const botEnabled =
    status.bot_protection_enabled === true ||
    !!(status.capjs_check || status.turnstile_check);
  const providerRaw = status.bot_protection_provider;
  const capApiEndpoint = String(status.capjs_api_endpoint || '').trim();
  const useCap =
    providerRaw === 'capjs' ||
    (!providerRaw && status.capjs_check && capApiEndpoint.length > 0);
  const useTurnstile =
    providerRaw === 'turnstile' ||
    (!providerRaw &&
      !status.capjs_check &&
      !!(status.turnstile_check && status.turnstile_site_key));
  const isCapJsEnabled = botEnabled && useCap && capApiEndpoint.length > 0;
  const isTurnstileEnabled =
    botEnabled && useTurnstile && !!status.turnstile_site_key;
  const enabled = isCapJsEnabled || isTurnstileEnabled;
  const provider = isCapJsEnabled
    ? 'capjs'
    : isTurnstileEnabled
      ? 'turnstile'
      : null;
  return {
    enabled,
    provider,
    turnstileSiteKey: String(status.turnstile_site_key || ''),
    capApiEndpoint,
  };
}