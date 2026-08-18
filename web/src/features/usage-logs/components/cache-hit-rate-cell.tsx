import { getCacheHitRate } from '../lib/format'

export function CacheHitRateCell({
  promptTokens,
  cacheReadTokens,
  cacheWriteTokens,
  billingPath,
}: {
  promptTokens: number
  cacheReadTokens: number
  cacheWriteTokens?: number
  billingPath?: string
}) {
  const cacheHitRate = getCacheHitRate(
    promptTokens,
    cacheReadTokens,
    cacheWriteTokens,
    billingPath
  )
  if (cacheHitRate === 0) {
    return <span className='text-muted-foreground/50 text-[11px]'>—</span>
  }

  return (
    <span
      className='font-mono text-xs font-medium tabular-nums'
      style={{ color: 'var(--color-emerald-600)' }}
    >
      {(cacheHitRate * 100).toFixed(1)}%
    </span>
  )
}
