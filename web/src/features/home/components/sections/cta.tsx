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
import { ArrowRight, BookOpen } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Footer } from '@/components/layout/components/footer'
import { Button } from '@/components/ui/button'

type CTAPath = '/dashboard' | '/sign-up' | '/sign-in'

interface CTAProps {
  docsUrl: string
  isAuthenticated: boolean
  registrationEnabled: boolean
  serverAddress: string
}

export function CTA(props: CTAProps) {
  const baseUrl =
    props.serverAddress.replace(/\/+$/, '').replace(/\/v1$/, '') ||
    'http://localhost:3000'
  const { t } = useTranslation()
  let primaryPath: CTAPath = '/sign-in'
  let primaryLabel = t('Sign in')
  if (props.isAuthenticated) {
    primaryPath = '/dashboard'
    primaryLabel = t('Go to Dashboard')
  } else if (props.registrationEnabled) {
    primaryPath = '/sign-up'
    primaryLabel = t('Get Started')
  }

  return (
    <section
      id='home-cta'
      aria-labelledby='home-cta-title'
      className='editorial-folio-section editorial-folio-final-page border-b'
    >
      <div className='editorial-folio-final-content px-5 sm:px-8 lg:px-12'>
        <div className='mx-auto grid w-full max-w-[90rem] lg:grid-cols-[1fr_0.78fr] lg:items-center'>
          <div className='editorial-reveal max-w-3xl pb-10 lg:pr-16 lg:pb-0'>
            <p className='editorial-label'>§ 05 · {t('Get Started')}</p>
            <h2
              id='home-cta-title'
              className='mt-5 text-4xl leading-tight font-medium text-balance sm:text-5xl lg:text-6xl'
            >
              {t('Ready to connect your tools?')}
            </h2>
            <p className='text-muted-foreground mt-6 max-w-xl text-base leading-7'>
              {t(
                'Create one key and use it across your AI development workflow.'
              )}
            </p>
            <p className='editorial-colophon editorial-rule mt-10 border-t pt-3'>
              <span>{baseUrl}/v1</span>
              <span>VOL. I / ISSUE 01</span>
            </p>
          </div>

          <div className='editorial-reveal editorial-rule flex flex-col gap-5 border-y py-8 lg:border-y-0 lg:border-l lg:py-0 lg:pl-10'>
            <Button
              className='editorial-primary-button group h-14 justify-between rounded-md px-5 text-left text-base font-medium'
              render={<Link to={primaryPath} />}
            >
              <span className='flex items-center gap-3'>
                <span className='editorial-cta-ornament' aria-hidden='true' />
                {primaryLabel}
              </span>
              <ArrowRight className='size-5 transition-transform duration-200 group-hover:translate-x-1' />
            </Button>
            <div className='flex flex-wrap items-center gap-5'>
              <Button
                variant='ghost'
                className='group h-9 px-0 text-sm'
                render={<Link to='/pricing' />}
              >
                {t('View Pricing')}
                <ArrowRight className='size-4 transition-transform duration-200 group-hover:translate-x-1' />
              </Button>
              <Button
                variant='ghost'
                className='h-9 px-0 text-sm'
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
        </div>
      </div>
      <Footer className='editorial-home-footer shrink-0' />
    </section>
  )
}
