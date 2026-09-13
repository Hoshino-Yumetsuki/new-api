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
import { cleanup, render } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { useHomeScrollSnap } from '../hooks/use-home-scroll-snap'

let scrollY: number
let now: number
let media: MediaQueryList

function HomeFixture(props: { enabled?: boolean }) {
  const ref = useHomeScrollSnap(props.enabled ?? true)
  return (
    <main ref={ref}>
      <section>
        <div data-testid='preview'>
          <textarea />
        </div>
      </section>
      <section />
      <section />
    </main>
  )
}

function mountHome(heights = [800, 800, 800]) {
  const view = render(<HomeFixture />)
  const sections = [...view.container.querySelectorAll('section')]
  let top = 0
  for (const [index, section] of sections.entries()) {
    const offset = top
    const height = heights[index]
    vi.spyOn(section, 'getBoundingClientRect').mockImplementation(() =>
      DOMRect.fromRect({ y: offset - scrollY, height })
    )
    top += height
  }
  vi.spyOn(document.documentElement, 'scrollHeight', 'get').mockReturnValue(top)
  return { ...view, sections }
}

function wheel(target: Element, init: WheelEventInit = {}) {
  const event = new WheelEvent('wheel', {
    bubbles: true,
    cancelable: true,
    deltaY: 100,
    ...init,
  })
  target.dispatchEvent(event)
  return event
}

beforeEach(() => {
  scrollY = 0
  now = 1000
  media = Object.assign(new EventTarget(), { matches: true }) as MediaQueryList
  vi.stubGlobal('matchMedia', () => media)
  vi.stubGlobal('innerHeight', 800)
  vi.spyOn(window, 'scrollY', 'get').mockImplementation(() => scrollY)
  vi.spyOn(performance, 'now').mockImplementation(() => now)
  vi.spyOn(window, 'scrollTo').mockImplementation(
    (options?: ScrollToOptions | number) => {
      if (typeof options === 'object') scrollY = options.top ?? scrollY
    }
  )
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

test('a large gesture advances one section and its inertia cannot skip the next', () => {
  const { sections } = mountHome()
  expect(wheel(sections[0], { deltaY: 2400 }).defaultPrevented).toBe(true)
  expect(scrollY).toBe(800)
  window.dispatchEvent(new Event('scrollend'))
  now += 100
  wheel(sections[1], { deltaY: 300 })
  now += 100
  wheel(sections[1], { deltaY: 30 })
  expect(scrollY).toBe(800)
  now += 200
  wheel(sections[1])
  expect(scrollY).toBe(1600)
})

test('small trackpad deltas accumulate rather than getting ignored indefinitely', () => {
  const { sections } = mountHome()
  wheel(sections[0], { deltaY: 5 })
  now += 20
  wheel(sections[0], { deltaY: 5 })
  expect(scrollY).toBe(0)
  now += 20
  wheel(sections[0], { deltaY: 5 })
  expect(scrollY).toBe(800)
})

test.each([WheelEvent.DOM_DELTA_LINE, WheelEvent.DOM_DELTA_PAGE])(
  'wheel delta mode %s advances without requiring pixel-sized input',
  (deltaMode) => {
    const { sections } = mountHome()
    wheel(sections[0], { deltaY: 1, deltaMode })
    expect(scrollY).toBe(800)
  }
)

test('oversized sections stay natively scrollable until their trailing edge is visible', () => {
  const { sections } = mountHome([1400, 800, 800])
  expect(wheel(sections[0]).defaultPrevented).toBe(false)
  scrollY = 300
  expect(wheel(sections[0], { deltaY: -100 }).defaultPrevented).toBe(false)
  expect(wheel(sections[0]).defaultPrevented).toBe(false)
  scrollY = 600
  wheel(sections[0])
  expect(scrollY).toBe(1400)
})

test('scrolling up from a partial page returns to its start before the preceding page', () => {
  const { sections } = mountHome()
  scrollY = 1100
  wheel(sections[1], { deltaY: -100 })
  expect(scrollY).toBe(800)
  window.dispatchEvent(new Event('scrollend'))
  now += 200
  wheel(sections[1], { deltaY: -100 })
  expect(scrollY).toBe(0)
  now += 200
  window.dispatchEvent(new Event('scrollend'))
  expect(wheel(sections[0], { deltaY: -100 }).defaultPrevented).toBe(false)
})

test('nested scrolling, editing, zooming and horizontal gestures retain native behavior', () => {
  const { sections, getByTestId, getByRole } = mountHome()
  const preview = getByTestId('preview')
  preview.style.overflowY = 'auto'
  vi.spyOn(preview, 'scrollHeight', 'get').mockReturnValue(400)
  vi.spyOn(preview, 'clientHeight', 'get').mockReturnValue(200)
  expect(wheel(preview).defaultPrevented).toBe(false)
  expect(wheel(getByRole('textbox')).defaultPrevented).toBe(false)
  expect(wheel(sections[0], { ctrlKey: true }).defaultPrevented).toBe(false)
  expect(wheel(sections[0], { deltaX: 200 }).defaultPrevented).toBe(false)
  expect(scrollY).toBe(0)
})

test('disabling motion or leaving the default homepage releases wheel input', () => {
  const view = mountHome()
  Object.assign(media, { matches: false })
  media.dispatchEvent(new Event('change'))
  expect(wheel(view.sections[0]).defaultPrevented).toBe(false)
  Object.assign(media, { matches: true })
  media.dispatchEvent(new Event('change'))
  view.rerender(<HomeFixture enabled={false} />)
  expect(wheel(view.sections[0]).defaultPrevented).toBe(false)
  expect(scrollY).toBe(0)
})
