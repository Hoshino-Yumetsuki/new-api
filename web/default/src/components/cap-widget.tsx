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
import { useEffect, useRef, type RefObject } from 'react'

type CapSolveEvent = CustomEvent<{ token: string }>

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
    <cap-widget
      ref={ref as RefObject<HTMLDivElement>}
      className={className}
      data-cap-api-endpoint={apiEndpoint}
    />
  )
}
