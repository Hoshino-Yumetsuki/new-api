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
import { useEffect, useRef } from 'react'

export function useHomeScrollSnap(enabled: boolean) {
  const ref = useRef<HTMLElement>(null)

  useEffect(() => {
    const home = ref.current
    if (!enabled || !home) return

    const media = window.matchMedia(
      '(min-width: 1024px) and (pointer: fine) and (prefers-reduced-motion: no-preference)'
    )
    let accumulatedDelta = 0
    let lastWheelAt = 0
    let snapStartedAt = 0
    let snapping = false
    let scrolling = false

    const reset = () => {
      accumulatedDelta = 0
      snapping = false
      scrolling = false
    }
    const onScrollEnd = () => {
      scrolling = false
    }
    const onMediaChange = () => {
      if (scrolling) {
        window.scrollTo({ top: window.scrollY, behavior: 'instant' })
      }
      reset()
    }
    const onWheel = (event: WheelEvent) => {
      if (
        !media.matches ||
        event.defaultPrevented ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey ||
        Math.abs(event.deltaX) >= Math.abs(event.deltaY)
      ) {
        return
      }

      // Embedded controls and scrollable previews retain their native input.
      for (
        let target = event.target instanceof Element ? event.target : null;
        target && target !== home;
        target = target.parentElement
      ) {
        if (
          target.matches(
            'input, textarea, select, [contenteditable]:not([contenteditable="false"])'
          ) ||
          (target.scrollHeight > target.clientHeight &&
            /auto|scroll/.test(window.getComputedStyle(target).overflowY))
        ) {
          return
        }
      }

      const now = performance.now()
      const gap = now - lastWheelAt
      lastWheelAt = now
      // Wait for both the animation and the gesture's inertial tail to finish.
      if (
        snapping &&
        ((scrolling && now - snapStartedAt < 1200) || gap < 180)
      ) {
        event.preventDefault()
        return
      }
      snapping = false

      const sections = [...home.children].filter(
        (element) => element.tagName === 'SECTION'
      )
      const bounds = sections.map((section) => section.getBoundingClientRect())
      const index = bounds.findIndex((rect) => rect.top <= 2 && rect.bottom > 2)
      if (index < 0) return
      const current = bounds[index]
      const down = event.deltaY > 0
      // Long sections must be readable before advancing to the next page.
      if (
        current.height > window.innerHeight + 2 &&
        (down ? current.bottom > window.innerHeight + 2 : current.top < -2)
      ) {
        accumulatedDelta = 0
        return
      }
      const destination = down
        ? bounds[index + 1]
        : bounds[current.top < -2 ? index : index - 1]
      if (!destination) return
      const top = Math.max(
        0,
        Math.min(
          window.scrollY + destination.top,
          document.documentElement.scrollHeight - window.innerHeight
        )
      )
      if (Math.abs(top - window.scrollY) < 2) return

      event.preventDefault()
      let delta = event.deltaY
      if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) delta *= 16
      if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
        delta *= window.innerHeight
      }
      if (gap > 180 || Math.sign(delta) !== Math.sign(accumulatedDelta)) {
        accumulatedDelta = 0
      }
      accumulatedDelta += delta
      if (Math.abs(accumulatedDelta) < 12) return

      accumulatedDelta = 0
      snapping = true
      scrolling = true
      snapStartedAt = now
      window.scrollTo({ top, behavior: 'smooth' })
    }

    home.addEventListener('wheel', onWheel, { passive: false })
    home.addEventListener('pointerdown', reset)
    home.addEventListener('keydown', reset)
    window.addEventListener('scrollend', onScrollEnd)
    media.addEventListener('change', onMediaChange)
    return () => {
      home.removeEventListener('wheel', onWheel)
      home.removeEventListener('pointerdown', reset)
      home.removeEventListener('keydown', reset)
      window.removeEventListener('scrollend', onScrollEnd)
      media.removeEventListener('change', onMediaChange)
    }
  }, [enabled])

  return ref
}
