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
import {
  BadgeDollarSign,
  LifeBuoy,
  Network,
  Workflow,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

type Advantage = {
  description: string
  icon: LucideIcon
  title: string
}

export function Advantages() {
  const { t } = useTranslation()
  const advantages: Advantage[] = [
    {
      title: t('Direct access without proxy setup'),
      description: t(
        'Use leading models from an ordinary network connection with low-latency routing.'
      ),
      icon: Network,
    },
    {
      title: t('Cost-efficient model access'),
      description: t(
        'Centralized purchasing and routing keep model access practical for daily development.'
      ),
      icon: BadgeDollarSign,
    },
    {
      title: t('Maintained by a dedicated team'),
      description: t(
        'Service health is monitored continuously, with integration support when issues appear.'
      ),
      icon: LifeBuoy,
    },
    {
      title: t('One gateway for every workflow'),
      description: t(
        'Use the same endpoint and key across IDE extensions, CLI tools, and web clients.'
      ),
      icon: Workflow,
    },
  ]

  return (
    <section
      id='home-advantages'
      aria-labelledby='home-advantages-title'
      className='editorial-folio-section editorial-ink-page border-b px-5 sm:px-8 lg:px-12'
    >
      <div className='mx-auto w-full max-w-[90rem]'>
        <div className='editorial-reveal grid gap-8 lg:grid-cols-[0.85fr_1.15fr] lg:items-end'>
          <div>
            <p className='editorial-label editorial-label-dark'>
              § 03 · {t('Platform advantages')}
            </p>
            <h2
              id='home-advantages-title'
              className='mt-5 max-w-xl text-[2rem] leading-tight font-medium break-keep sm:text-5xl lg:text-[2.75rem] xl:text-5xl'
            >
              {t('Reliable access for everyday AI development')}
            </h2>
          </div>
          <div
            role='img'
            aria-label={`${t('Affordable')}, ${t('Stable')}, ${t('Fast')}`}
            className='editorial-triad lg:justify-self-end'
          >
            <span className='editorial-triad-lobe editorial-triad-affordable'>
              <span>{t('Affordable')}</span>
            </span>
            <span className='editorial-triad-lobe editorial-triad-stable'>
              <span>{t('Stable')}</span>
            </span>
            <span className='editorial-triad-lobe editorial-triad-fast'>
              <span>{t('Fast')}</span>
            </span>
            <span className='editorial-triad-core' aria-hidden='true'>
              API
            </span>
          </div>
        </div>

        <div className='editorial-reveal editorial-rule mt-12 grid border-t border-l md:grid-cols-2'>
          {advantages.map((advantage, index) => {
            const Icon = advantage.icon
            return (
              <article
                key={advantage.title}
                className='editorial-rule group hover:bg-foreground/[0.04] min-h-48 border-r border-b p-6 transition-colors duration-300 sm:p-8'
              >
                <div className='flex items-center justify-between gap-4'>
                  <span className='text-muted-foreground font-mono text-xs transition-colors duration-300 group-hover:text-[var(--editorial-accent)]'>
                    {String(index + 1).padStart(2, '0')}
                  </span>
                  <Icon
                    aria-hidden='true'
                    className='text-muted-foreground size-5 transition-colors duration-300 group-hover:text-[var(--editorial-accent)]'
                  />
                </div>
                <h3 className='mt-10 text-xl font-medium'>{advantage.title}</h3>
                <p className='text-muted-foreground mt-3 max-w-xl text-sm leading-6'>
                  {advantage.description}
                </p>
              </article>
            )
          })}
        </div>
      </div>
    </section>
  )
}
