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
import { useTranslation } from 'react-i18next'

import { getLobeIcon } from '@/lib/lobe-icon'

const PROVIDERS = [
  { name: 'Claude', icon: 'Claude.Color' },
  { name: 'Gemini', icon: 'Gemini.Color' },
  { name: 'DeepSeek', icon: 'DeepSeek.Color' },
  { name: 'Qwen', icon: 'Qwen.Color' },
  { name: 'GLM', icon: 'Zhipu.Color' },
  { name: 'OpenAI', icon: 'OpenAI' },
  { name: 'Moonshot', icon: 'Moonshot.Color' },
  { name: 'Mistral', icon: 'Mistral.Color' },
  { name: 'OpenRouter', icon: 'OpenRouter' },
  { name: 'Groq', icon: 'Groq' },
] as const

const TOOLS = [
  'Cursor',
  'Windsurf',
  'Cline',
  'Cherry Studio',
  'Roo Code',
  'Claude Code',
]

function ToolMark(props: { name: string }) {
  const initials = props.name
    .split(' ')
    .map((part) => part[0])
    .join('')
    .slice(0, 2)

  return (
    <span
      aria-hidden='true'
      className='border-border/60 bg-muted/60 text-muted-foreground flex size-7 shrink-0 items-center justify-center rounded-md border font-mono text-[9px] font-semibold'
    >
      {initials}
    </span>
  )
}

const logoItemClassName =
  'editorial-rule group flex min-h-16 items-center gap-3 border-b py-3.5 pr-3 text-left text-xs font-medium transition-colors duration-300 hover:text-[var(--editorial-accent)]'

export function Ecosystem() {
  const { t } = useTranslation()

  return (
    <section
      id='home-ecosystem'
      aria-labelledby='home-ecosystem-title'
      className='editorial-folio-section border-b px-5 sm:px-8 lg:px-12'
    >
      <div className='mx-auto grid w-full max-w-[90rem] gap-12 lg:grid-cols-[minmax(24rem,0.78fr)_minmax(0,1.55fr)] lg:items-center lg:gap-16'>
        <div className='editorial-reveal max-w-xl'>
          <p className='editorial-label'>§ 04 · {t('Model ecosystem')}</p>
          <h2
            id='home-ecosystem-title'
            className='mt-5 text-4xl leading-tight font-medium text-balance sm:text-5xl lg:text-[2.5rem] lg:break-keep'
          >
            {t('Leading models and developer tools, connected through one API')}
          </h2>
          <p className='text-muted-foreground mt-6 max-w-lg text-base leading-7'>
            {t(
              'Switch between major model providers and coding clients without rebuilding each integration.'
            )}
          </p>
          <p className='editorial-colophon editorial-rule mt-9 hidden border-t pt-3 lg:flex'>
            <span>MODELS / 10</span>
            <span>TOOLS / 06</span>
          </p>
        </div>

        <div className='editorial-reveal min-w-0'>
          <div>
            <div className='flex items-center justify-between gap-4'>
              <h3 className='text-muted-foreground font-mono text-[10px] tracking-[0.16em] uppercase'>
                {t('Models and providers')}
              </h3>
              <span className='text-muted-foreground font-mono text-[9px] tracking-[0.14em] uppercase'>
                10 / INDEX
              </span>
            </div>
            <div className='editorial-rule mt-3 grid grid-cols-2 border-t sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5'>
              {PROVIDERS.map((provider) => (
                <div key={provider.name} className={logoItemClassName}>
                  <div aria-hidden='true' className='shrink-0'>
                    {getLobeIcon(provider.icon, 24)}
                  </div>
                  <span>{provider.name}</span>
                </div>
              ))}
              <div className={logoItemClassName}>
                <span className='text-base font-medium text-[var(--editorial-accent)] italic'>
                  30+
                </span>
                <span>{t('More providers')}</span>
              </div>
            </div>
          </div>

          <div className='mt-8'>
            <div className='flex items-center justify-between gap-4'>
              <h3 className='text-muted-foreground font-mono text-[10px] tracking-[0.16em] uppercase'>
                {t('Developer tools')}
              </h3>
              <span className='text-muted-foreground font-mono text-[9px] tracking-[0.14em] uppercase'>
                06 / INDEX
              </span>
            </div>
            <div className='editorial-rule mt-3 grid grid-cols-2 border-t sm:grid-cols-3 xl:grid-cols-6'>
              {TOOLS.map((tool) => (
                <div key={tool} className={logoItemClassName}>
                  <ToolMark name={tool} />
                  <span>{tool}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
