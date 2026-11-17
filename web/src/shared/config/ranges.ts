export const RANGES = ['1h', '6h', '24h', '7d', '30d', '90d', '1y', 'all'] as const

export type RangeKey = (typeof RANGES)[number]
