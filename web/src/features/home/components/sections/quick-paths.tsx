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
import { ArrowUpRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

interface QuickPathsProps {
  docsUrl: string
  isAuthenticated: boolean
  registrationEnabled: boolean
}

type InternalPath = '/dashboard' | '/sign-up' | '/sign-in' | '/wallet'

const rowClassName =
  'editorial-rule group grid grid-cols-[2.75rem_minmax(0,1fr)_auto] items-center gap-4 border-t px-4 py-6 transition-colors duration-300 hover:bg-muted/35 sm:grid-cols-[4rem_minmax(0,1fr)_auto] sm:px-6'

export function QuickPaths(props: QuickPathsProps) {
  const { t } = useTranslation()
  let entryPath: InternalPath = '/sign-in'
  if (props.isAuthenticated) {
    entryPath = '/dashboard'
  } else if (props.registrationEnabled) {
    entryPath = '/sign-up'
  }
  const walletPath: InternalPath = props.isAuthenticated
    ? '/wallet'
    : '/sign-in'

  return (
    <section
      id='home-quick-paths'
      aria-labelledby='home-quick-paths-title'
      className='editorial-folio-section border-b px-5 sm:px-8 lg:px-12'
    >
      <div className='mx-auto grid w-full max-w-[90rem] lg:grid-cols-[0.75fr_1.25fr]'>
        <div className='editorial-reveal flex flex-col justify-center pb-10 lg:pr-14 lg:pb-0'>
          <p className='editorial-label'>§ 02 · {t('Quick paths')}</p>
          <h2
            id='home-quick-paths-title'
            className='mt-5 max-w-lg text-4xl leading-tight font-medium text-balance sm:text-5xl'
          >
            {t('Start with the workflow you need')}
          </h2>
          <p className='text-muted-foreground mt-6 max-w-md text-base leading-7'>
            {t('Build on your API gateway in minutes')}
          </p>
          <p className='editorial-colophon editorial-rule mt-10 hidden border-t pt-3 lg:flex'>
            <span>PATHS / 03</span>
            <span>EST. 2026</span>
          </p>
        </div>

        <div className='editorial-reveal editorial-rule border-b lg:border-l'>
          <Link to={entryPath} className={rowClassName}>
            <span className='text-muted-foreground font-mono text-xs transition-colors duration-300 group-hover:text-[var(--editorial-accent)]'>
              01
            </span>
            <div>
              <h3 className='text-lg font-medium'>{t('Open the console')}</h3>
              <p className='text-muted-foreground mt-1.5 text-sm leading-6'>
                {t('Create keys, inspect usage, and manage your account.')}
              </p>
            </div>
            <ArrowUpRight
              aria-hidden='true'
              className='text-muted-foreground size-4 transition-transform duration-300 group-hover:translate-x-1 group-hover:-translate-y-1'
            />
          </Link>
          <Link to={walletPath} className={rowClassName}>
            <span className='text-muted-foreground font-mono text-xs transition-colors duration-300 group-hover:text-[var(--editorial-accent)]'>
              02
            </span>
            <div>
              <h3 className='text-lg font-medium'>{t('Manage balance')}</h3>
              <p className='text-muted-foreground mt-1.5 text-sm leading-6'>
                {t('Review balance, top-ups, and usage details.')}
              </p>
            </div>
            <ArrowUpRight
              aria-hidden='true'
              className='text-muted-foreground size-4 transition-transform duration-300 group-hover:translate-x-1 group-hover:-translate-y-1'
            />
          </Link>
          <a
            href={props.docsUrl}
            target='_blank'
            rel='noopener noreferrer'
            className={rowClassName}
          >
            <span className='text-muted-foreground font-mono text-xs transition-colors duration-300 group-hover:text-[var(--editorial-accent)]'>
              03
            </span>
            <div>
              <h3 className='text-lg font-medium'>
                {t('Help and documentation')}
              </h3>
              <p className='text-muted-foreground mt-1.5 text-sm leading-6'>
                {t(
                  'Find setup guides, integration notes, and troubleshooting.'
                )}
              </p>
            </div>
            <ArrowUpRight
              aria-hidden='true'
              className='text-muted-foreground size-4 transition-transform duration-300 group-hover:translate-x-1 group-hover:-translate-y-1'
            />
          </a>
        </div>
      </div>
    </section>
  )
}
