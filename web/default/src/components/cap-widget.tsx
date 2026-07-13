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
import 'cap-widget'
import { useEffect, useRef, type CSSProperties, type RefObject } from 'react'

import { cn } from '@/lib/utils'

type CapSolveEvent = CustomEvent<{ token: string }>

/** Maps shadcn theme tokens to Cap widget CSS variables (see capjs.js.org/guide/widget). */
const capWidgetThemeStyle = {
  '--cap-background': 'var(--background)',
  '--cap-border-color': 'var(--border)',
  '--cap-border-radius': '14px',
  '--cap-color': 'var(--foreground)',
  '--cap-checkbox-border': '1px solid var(--ring)',
  '--cap-checkbox-background': 'var(--secondary)',
  '--cap-spinner-color': 'var(--primary)',
  '--cap-spinner-background-color': 'var(--muted)',
} as CSSProperties

interface CapWidgetProps {
  apiEndpoint: string
  onVerify: (token: string) => void
  onExpire?: () => void
  className?: string
}

export function CapWidget({
  apiEndpoint,
  onVerify,
  onExpire,
  className,
}: CapWidgetProps) {
  const ref = useRef<HTMLElement | null>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return

    const handleSolve = (event: Event) => {
      const token = (event as CapSolveEvent).detail?.token
      if (token) onVerify(token)
    }
    const handleExpire = () => onExpire?.()

    el.addEventListener('solve', handleSolve)
    el.addEventListener('error', handleExpire)
    el.addEventListener('reset', handleExpire)

    return () => {
      el.removeEventListener('solve', handleSolve)
      el.removeEventListener('error', handleExpire)
      el.removeEventListener('reset', handleExpire)
    }
  }, [apiEndpoint, onVerify, onExpire])

  return (
    <div className={cn(className)} style={capWidgetThemeStyle}>
      <cap-widget
        ref={ref as RefObject<HTMLDivElement>}
        data-cap-api-endpoint={apiEndpoint}
      />
    </div>
  )
}
