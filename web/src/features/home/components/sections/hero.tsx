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
import { Link } from '@tanstack/react-router'
import { ArrowRight, BookOpen, Check, Copy } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import { HeroTerminalDemo } from '../hero-terminal-demo'

type HeroPath = '/dashboard' | '/sign-up' | '/sign-in' | '/pricing'

interface HeroProps {
  docsUrl: string
  isAuthenticated: boolean
  registrationEnabled: boolean
  serverAddress: string
}

const ENDPOINT_PATHS = [
  '/v1/chat/completions',
  '/v1/models',
  '/v1/embeddings',
  '/v1/images/generations',
  '/v1/audio/transcriptions',
] as const
const ENDPOINT_CYCLE_INTERVAL = 3200

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard()
  const [endpointIndex, setEndpointIndex] = useState(0)
  const baseUrl =
    props.serverAddress.replace(/\/+$/, '').replace(/\/v1$/, '') ||
    'http://localhost:3000'

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    if (mediaQuery.matches) return

    const intervalId = window.setInterval(() => {
      setEndpointIndex((current) => (current + 1) % ENDPOINT_PATHS.length)
    }, ENDPOINT_CYCLE_INTERVAL)

    return () => window.clearInterval(intervalId)
  }, [])

  const endpointPath = ENDPOINT_PATHS[endpointIndex]
  let primaryPath: HeroPath = '/sign-in'
  let primaryLabel = t('Sign in')
  let secondaryPath: HeroPath = '/sign-in'
  let secondaryLabel = t('Sign in')
  if (props.isAuthenticated) {
    primaryPath = '/dashboard'
    primaryLabel = t('Go to Dashboard')
    secondaryPath = '/pricing'
    secondaryLabel = t('View Pricing')
  } else if (props.registrationEnabled) {
    primaryPath = '/sign-up'
    primaryLabel = t('Create account')
  }
  const copied = copiedText === baseUrl

  return (
    <section
      id='home-hero'
      aria-labelledby='home-hero-title'
      className='editorial-hero relative overflow-hidden border-b px-6 pt-24 pb-16 md:pt-32 md:pb-24 lg:pt-36 lg:pb-28'
    >
      <div className='editorial-hero-wash' aria-hidden='true' />
      <div className='editorial-hero-grid' aria-hidden='true' />

      <div className='relative z-10 mx-auto grid w-full max-w-6xl grid-cols-1 items-center gap-12 lg:grid-cols-12 lg:gap-8'>
        <div className='flex min-w-0 flex-col justify-center lg:col-span-6 lg:border-r lg:pr-8'>
          <p className='editorial-label'>§ 01 · {t('Unified AI access')}</p>
          <h1
            id='home-hero-title'
            className='editorial-hero-title mt-5 max-w-3xl text-4xl leading-[1.12] font-medium text-balance sm:text-5xl lg:text-6xl xl:text-[4.35rem]'
          >
            <span>{t('One gateway for AI coding')}</span>
            <span className='editorial-caret' aria-hidden='true' />
          </h1>
          <p className='text-muted-foreground mt-6 max-w-xl text-base leading-8 sm:text-lg'>
            {t(
              'Use the same endpoint and key across IDE extensions, CLI tools, and web clients.'
            )}
          </p>

          <div className='mt-9 flex flex-wrap items-center gap-3'>
            <Button
              className='editorial-primary-button group inline-flex h-[3.25rem] rounded-[1px] px-7 text-[15px] font-medium'
              render={<Link to={primaryPath} />}
            >
              <span aria-hidden='true' className='editorial-cta-ornament' />
              <span>{primaryLabel}</span>
              <ArrowRight
                aria-hidden='true'
                className='size-6 transition-transform duration-200 group-hover:translate-x-1'
              />
            </Button>
            <Button
              variant='ghost'
              className='editorial-secondary-button group h-auto rounded-none px-0 text-[15px] font-medium'
              render={<Link to={secondaryPath} />}
            >
              {secondaryLabel}
              <ArrowRight
                aria-hidden='true'
                className='size-4 transition-transform duration-200 group-hover:translate-x-1'
              />
            </Button>
          </div>

          <div className='editorial-endpoint-panel editorial-rule mt-10 max-w-2xl border p-4 sm:p-5'>
            <p className='text-muted-foreground font-mono text-[10px] tracking-[0.18em] uppercase'>
              {t('Replace the base URL to connect')}
            </p>
            <div className='editorial-endpoint-input'>
              <code
                className='editorial-endpoint-code text-sm font-semibold sm:text-base'
                aria-label={`${baseUrl}${endpointPath}`}
              >
                <span className='truncate'>{baseUrl}</span>
                <span
                  key={endpointPath}
                  className='editorial-endpoint-path text-[var(--editorial-endpoint-blue)]'
                >
                  {endpointPath}
                </span>
              </code>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='editorial-endpoint-copy size-9 shrink-0'
                aria-label={
                  copied ? t('Copied to clipboard') : t('Copy API base URL')
                }
                title={
                  copied ? t('Copied to clipboard') : t('Copy API base URL')
                }
                onClick={() => void copyToClipboard(baseUrl)}
              >
                {copied ? (
                  <Check aria-hidden='true' className='text-success size-4' />
                ) : (
                  <Copy aria-hidden='true' className='size-4' />
                )}
              </Button>
            </div>
          </div>

          <div className='mt-4 flex flex-wrap items-center gap-4'>
            <Button
              variant='ghost'
              className='group h-8 gap-1.5 px-2.5 text-sm'
              render={<Link to='/pricing' />}
            >
              {t('View Pricing')}
              <ArrowRight
                aria-hidden='true'
                className='size-4 transition-transform group-hover:translate-x-1'
              />
            </Button>
            <Button
              variant='ghost'
              className='h-8 gap-1.5 px-2.5 text-sm'
              render={
                <a
                  href={props.docsUrl}
                  target='_blank'
                  rel='noopener noreferrer'
                />
              }
            >
              <BookOpen aria-hidden='true' className='size-4' />
              {t('Docs')}
            </Button>
          </div>
        </div>

        <div className='editorial-hero-card flex min-w-0 flex-col justify-center lg:col-span-6 lg:pl-8'>
          <div className='text-muted-foreground mb-4 flex items-center gap-2.5 font-mono text-[9px] tracking-[0.18em] uppercase'>
            <span className='text-foreground font-medium'>PL. I</span>
            <span
              className='editorial-rule h-px flex-1 border-t'
              aria-hidden='true'
            />
            <span>{t('API protocol preview')}</span>
          </div>
          <HeroTerminalDemo />
          <p className='editorial-rule text-muted-foreground mt-4 border-t pt-3 font-mono text-[9px] tracking-[0.12em] uppercase'>
            <span>FIG. 0.1 — </span>
            <span className='text-foreground text-[11px] tracking-[0.04em] normal-case'>
              {t('One card for Chat, Responses, Claude, and Gemini')}
            </span>
          </p>
        </div>
      </div>

      <a
        href='#home-quick-paths'
        className='text-muted-foreground hover:text-foreground absolute right-5 bottom-3 z-10 hidden items-center gap-2 font-mono text-[9px] tracking-[0.18em] uppercase transition-colors sm:flex lg:right-12'
      >
        {t('Quick paths')}
        <ArrowRight aria-hidden='true' className='size-3 rotate-90' />
      </a>
    </section>
  )
}
