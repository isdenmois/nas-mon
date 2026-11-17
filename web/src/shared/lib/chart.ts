export interface ChartPoint {
  ts: number
  value: number
}

export interface ChartSeries {
  label: string
  points: ChartPoint[]
  color?: string
  dashed?: boolean
}

// SeriesSpec names a series the way the API does; SeriesIndex turns it into the
// server-assigned id that history queries use.
export interface SeriesSpec {
  kind: string
  name: string
  label: string
  color?: string
}

export interface HistorySeries {
  id: number
  label: string
  color?: string
}

export interface SeriesIndex {
  id(kind: string, name: string): number | undefined
  resolve(specs: SeriesSpec[]): HistorySeries[]
}

export const CHART_PALETTE = [
  '#60a5fa',
  '#f472b6',
  '#34d399',
  '#fbbf24',
  '#a78bfa',
  '#f87171',
  '#22d3ee',
  '#facc15',
] as const

export function chartColor(index: number): string {
  return CHART_PALETTE[Math.max(index, 0) % CHART_PALETTE.length]
}
