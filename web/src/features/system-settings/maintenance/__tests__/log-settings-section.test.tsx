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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { LogSettingsSection } from '../log-settings-section'

const { mutateAsync } = vi.hoisted(() => ({ mutateAsync: vi.fn() }))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({ isPending: false, mutateAsync }),
}))

vi.mock('../api', () => ({
  getCurrentLogCleanupTask: vi
    .fn()
    .mockRejectedValue(new Error('not available')),
  getSystemTask: vi.fn(),
  startLogCleanupTask: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn().mockRejectedValue(new Error('not available')) },
}))

describe('log settings save', () => {
  beforeEach(() => {
    mutateAsync.mockReset()
    mutateAsync.mockResolvedValue({ success: true })
  })

  test('submits the nested redirect setting instead of failing validation', async () => {
    const actionsContainer = document.createElement('div')
    document.body.append(actionsContainer)

    render(
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <LogSettingsSection
          defaultEnabled={false}
          defaultHideModelRedirectForNonAdmin={false}
        />
      </SettingsPageProvider>
    )

    const redirectSwitch = await screen.findByRole('switch', {
      name: 'Hide model redirect from non-admin users',
    })
    fireEvent.click(redirectSwitch)
    fireEvent.click(screen.getByRole('button', { name: 'Save log settings' }))

    await waitFor(() => {
      expect(mutateAsync).toHaveBeenCalledWith({
        key: 'general_setting.hide_model_redirect_for_non_admin',
        value: true,
      })
    })

    actionsContainer.remove()
  })
})
