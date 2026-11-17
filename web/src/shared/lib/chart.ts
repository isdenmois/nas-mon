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

export interface HistorySeries {
  kind: string
  name: string
  label: string
  color?: string
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
