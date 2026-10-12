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
          defaultValues={{
            LogConsumeEnabled: false,
            'general_setting.hide_model_redirect_for_non_admin': false,
            'log_setting.user_export_enabled': true,
            'log_setting.max_range_days': 365,
          }}
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
    expect(mutateAsync).toHaveBeenCalledTimes(1)

    actionsContainer.remove()
  })

  test('refreshing defaults resets all log settings controls', async () => {
    const view = render(
      <SettingsPageProvider actionsContainer={null}>
        <LogSettingsSection
          defaultValues={{
            LogConsumeEnabled: false,
            'general_setting.hide_model_redirect_for_non_admin': false,
            'log_setting.user_export_enabled': true,
            'log_setting.max_range_days': 365,
          }}
        />
      </SettingsPageProvider>
    )

    view.rerender(
      <SettingsPageProvider actionsContainer={null}>
        <LogSettingsSection
          defaultValues={{
            LogConsumeEnabled: true,
            'general_setting.hide_model_redirect_for_non_admin': true,
            'log_setting.user_export_enabled': false,
            'log_setting.max_range_days': 30,
          }}
        />
      </SettingsPageProvider>
    )

    await waitFor(() => {
      expect(
        screen.getByRole('switch', { name: 'Record quota usage' })
      ).toBeChecked()
      expect(
        screen.getByRole('switch', {
          name: 'Hide model redirect from non-admin users',
        })
      ).toBeChecked()
      expect(
        screen.getByRole('switch', { name: 'Allow users to export logs' })
      ).not.toBeChecked()
      expect(
        screen.getByRole('spinbutton', {
          name: 'Longest log time range (days)',
        })
      ).toHaveValue(30)
    })
    expect(mutateAsync).not.toHaveBeenCalled()
  })
})
