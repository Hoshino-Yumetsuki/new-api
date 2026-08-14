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
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { PricingModel } from '../../types'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'customElements',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: () => ({ matches: false }),
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ModelCard } = await import('../model-card')
const { PricingTable } = await import('../pricing-table')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Cached: 'Cached',
        Input: 'Input',
        Output: 'Output',
      },
    },
  },
})

const model: PricingModel = {
  id: 1,
  model_name: 'dynamic-cache-model',
  quota_type: 0,
  model_ratio: 1,
  completion_ratio: 1,
  enable_groups: ['default'],
  billing_mode: 'tiered_expr',
  billing_expr:
    'len <= 1000 ? tier("short", p * 1 + c * 4 + cr * 0.1 + cc * 1.25 + cc1h * 2) : tier("long", p * 3 + c * 2 + cr * 0.3 + cc * 1.5 + cc1h * 0.75)',
}

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function render(element: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(<I18nextProvider i18n={i18n}>{element}</I18nextProvider>)
  })

  return { container, root }
}

async function unmount(rendered: Awaited<ReturnType<typeof render>>) {
  await act(async () => rendered.root.unmount())
  rendered.container.remove()
}

describe('dynamic cache prices in model square', () => {
  after(() => domWindow.close())

  test('card view aligns dynamic prices to input, output, and cached', async () => {
    const rendered = await render(
      <ModelCard model={model} onClick={() => {}} />
    )
    const text = rendered.container.textContent ?? ''

    assert.match(text, /Input\s*\$1/)
    assert.match(text, /Output\s*\$2/)
    assert.match(text, /Cached\s*\$0\.1/)
    assert.doesNotMatch(text, /Cache Read|Cache Write|1h/)

    await unmount(rendered)
  })

  test('table view aligns dynamic prices to input, output, and cached', async () => {
    const rendered = await render(<PricingTable models={[model]} />)
    const text = rendered.container.textContent ?? ''

    assert.match(text, /\$1\/\$2/)
    assert.match(text, /Cached/)
    assert.match(text, /\$0\.1/)
    assert.doesNotMatch(text, /Cache Read|Cache Write|1h/)

    await unmount(rendered)
  })
})
