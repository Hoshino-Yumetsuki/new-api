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
import { useCallback, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import { useTheme } from '@/context/theme-provider'
import type { SystemStatus } from '@/features/auth/types'
import { useStatus } from '@/hooks/use-status'
import { isLikelyHtml } from '@/lib/content-format'
import { useAuthStore } from '@/stores/auth-store'

import { Advantages } from './components/sections/advantages'
import { CTA } from './components/sections/cta'
import { Ecosystem } from './components/sections/ecosystem'
import { Hero } from './components/sections/hero'
import { QuickPaths } from './components/sections/quick-paths'
import { useHomePageContent } from './hooks'
import { useHomeScrollSnap } from './hooks/use-home-scroll-snap'

function readStatusString(
  status: SystemStatus | null,
  key: string
): string | undefined {
  const directValue = status?.[key]
  if (typeof directValue === 'string' && directValue.trim()) {
    return directValue.trim()
  }

  const nestedValue = status?.data?.[key]
  if (typeof nestedValue === 'string' && nestedValue.trim()) {
    return nestedValue.trim()
  }

  return undefined
}

function isRegistrationEnabled(status: SystemStatus | null): boolean {
  return (
    status?.register_enabled !== false &&
    status?.data?.register_enabled !== false
  )
}

export function Home() {
  const { i18n, t } = useTranslation()
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const { resolvedTheme } = useTheme()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent()
  const { status } = useStatus()
  const homeRef = useHomeScrollSnap(isLoaded && !content)

  const syncIframePreferences = useCallback(() => {
    try {
      iframeRef.current?.contentWindow?.postMessage(
        { themeMode: resolvedTheme },
        '*'
      )
      iframeRef.current?.contentWindow?.postMessage(
        { lang: i18n.language },
        '*'
      )
    } catch {
      // Cross-origin frames may reject access while navigating.
    }
  }, [i18n.language, resolvedTheme])

  useEffect(() => {
    if (isUrl) {
      syncIframePreferences()
    }
  }, [isUrl, syncIframePreferences])

  if (!isLoaded) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='flex min-h-screen items-center justify-center'>
          <div className='text-muted-foreground'>{t('Loading...')}</div>
        </main>
      </PublicLayout>
    )
  }

  if (content) {
    if (isUrl) {
      return (
        <PublicLayout showMainContainer={false}>
          {/*
            allow-top-navigation-by-user-activation: the custom home page URL is
            admin-configured (trusted); this lets its target="_top" nav/menu links
            navigate the top-level window on user click. The default sandbox blocks
            this on desktop, while some mobile browsers allow it via allow-popups,
            causing inconsistent behavior. This token only permits user-activated
            top-level navigation and does NOT grant same-origin access.
          */}
          <iframe
            ref={iframeRef}
            src={content}
            className='h-screen w-full border-none'
            title={t('Custom Home Page')}
            sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts allow-top-navigation-by-user-activation'
            onLoad={syncIframePreferences}
          />
        </PublicLayout>
      )
    }

    const contentIsHtml = isLikelyHtml(content)

    if (contentIsHtml) {
      return (
        <PublicLayout showMainContainer={false}>
          <RichContent
            mode='html'
            htmlVariant='isolated'
            content={content}
            className='custom-home-content'
          />
        </PublicLayout>
      )
    }

    return (
      <PublicLayout>
        <div className='mx-auto max-w-6xl px-4 py-8'>
          <RichContent
            mode='markdown'
            content={content}
            className='custom-home-content'
          />
        </div>
      </PublicLayout>
    )
  }

  const docsUrl =
    readStatusString(status, 'docs_link') ?? 'https://docs.newapi.pro'
  const serverAddress =
    readStatusString(status, 'server_address') ?? window.location.origin
  const registrationEnabled = isRegistrationEnabled(status)

  return (
    <PublicLayout showMainContainer={false}>
      <main ref={homeRef} className='editorial-home'>
        <Hero
          docsUrl={docsUrl}
          isAuthenticated={isAuthenticated}
          registrationEnabled={registrationEnabled}
          serverAddress={serverAddress}
        />
        <QuickPaths
          docsUrl={docsUrl}
          isAuthenticated={isAuthenticated}
          registrationEnabled={registrationEnabled}
        />
        <Advantages />
        <Ecosystem />
        <CTA
          docsUrl={docsUrl}
          isAuthenticated={isAuthenticated}
          registrationEnabled={registrationEnabled}
          serverAddress={serverAddress}
        />
      </main>
    </PublicLayout>
  )
}
