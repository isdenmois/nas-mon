import type { RangeKey } from '../config'

export type ValueFormat = 'percent' | 'bytes' | 'rate' | 'number' | 'temperature'

export function formatValue(value: number, format: ValueFormat): string {
  switch (format) {
    case 'percent':
      return formatPct(value)
    case 'bytes':
      return formatBytes(value)
    case 'rate':
      return formatRate(value)
    case 'temperature':
      return `${formatNum(value)} °C`
    default:
      return formatNum(value)
  }
}

const byteUnits = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
const decimal = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 })
const integer = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 })
const shortRanges: ReadonlySet<string> = new Set(['1h', '6h', '24h', '48h'])

export function formatBytes(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  let n = Math.max(0, value)
  let unit = 0
  while (n >= 1024 && unit < byteUnits.length - 1) {
    n /= 1024
    unit += 1
  }
  if (unit === 0) return `${integer.format(Math.round(n))} B`
  return `${n >= 100 ? integer.format(n) : n.toFixed(1)} ${byteUnits[unit]}`
}

export function formatRate(value: number | null | undefined): string {
  const bytes = formatBytes(value)
  return bytes === '—' ? bytes : `${bytes}/s`
}

export function formatPct(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return `${decimal.format(value)}%`
}

export function formatNum(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return decimal.format(value)
}

export function clampPct(value: number | null | undefined): number {
  if (value == null || !Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, value))
}

export function formatDateTime(value: string | number | null | undefined): string {
  if (value == null || value === '') return '—'
  const date = typeof value === 'number' ? new Date(value * 1000) : new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function formatAxis(tsSeconds: number, range: RangeKey): string {
  const date = new Date(tsSeconds * 1000)
  if (shortRanges.has(range)) {
    return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  }
  if (range === '1y' || range === 'all') {
    return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
  }
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}
