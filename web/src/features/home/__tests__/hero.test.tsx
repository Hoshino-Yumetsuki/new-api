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
import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { Hero } from '../components/sections/hero'

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    children,
    to,
  }: {
    children?: ReactNode
    to: string
  }) => <a href={to}>{children}</a>,
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/hooks/use-copy-to-clipboard', () => ({
  useCopyToClipboard: () => ({
    copiedText: null,
    copyToClipboard: vi.fn(),
  }),
}))

describe('Hero actions', () => {
  test('closed registration keeps dashboard and sign-in actions pointed at sign-in', () => {
    render(
      <Hero
        docsUrl='https://docs.example.com'
        isAuthenticated={false}
        registrationEnabled={false}
        serverAddress='https://api.example.com'
      />
    )
    const dashboardLink = screen.getByRole('link', { name: /Go to Dashboard/ })
    const signInLink = screen.getByRole('link', { name: /^Sign in/ })

    expect(dashboardLink).toHaveAttribute('href', '/sign-in')
    expect(signInLink).toHaveAttribute('href', '/sign-in')
  })
})
