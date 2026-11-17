import type { DiskSmart, Wear } from '@/shared/api'
import { chartColor, type ChartSeries } from '@/shared/lib'

// Rows shown in the wear-log table. The fetch asks for more so the TBW chart
// still has history behind the visible window; the server bounds both by range.
export const WEAR_LOG_ROWS = 200
export const WEAR_LOG_FETCH_LIMIT = 20_000

export function groupWearByDisk(log: Wear[]): Map<string, Wear[]> {
  const byDisk = new Map<string, Wear[]>()
  for (const entry of log) {
    const list = byDisk.get(entry.disk)
    if (list) list.push(entry)
    else byDisk.set(entry.disk, [entry])
  }
  return byDisk
}

export function wearDiskNames(smart: DiskSmart[], grouped: Map<string, Wear[]>): string[] {
  const names = new Set<string>(smart.map((disk) => disk.disk))
  for (const disk of grouped.keys()) names.add(disk)
  return [...names].sort()
}

export function wearLogFor(
  grouped: Map<string, Wear[]>,
  disk: string,
  limit = WEAR_LOG_ROWS,
): Wear[] {
  return (grouped.get(disk) ?? []).slice(0, limit)
}

export function tbwSeries(log: Wear[]): ChartSeries[] {
  const byDisk = new Map<string, Map<number, number>>()
  for (const entry of log) {
    if (entry.tbw_bytes == null) continue
    let points = byDisk.get(entry.disk)
    if (!points) {
      points = new Map()
      byDisk.set(entry.disk, points)
    }
    points.set(entry.ts, entry.tbw_bytes)
  }
  return [...byDisk.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([disk, points], index) => ({
      label: disk,
      color: chartColor(index),
      points: [...points.entries()].sort(([a], [b]) => a - b).map(([ts, value]) => ({ ts, value })),
    }))
}
