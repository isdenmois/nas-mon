import { describe, expect, it } from 'vitest'
import type { DiskSmart, Wear } from '@/shared/api'
import { groupWearByDisk, tbwSeries, wearDiskNames, wearLogFor, WEAR_LOG_ROWS } from './disk'

function entry(overrides: Partial<Wear> = {}): Wear {
  return { disk: 'nvme0', tbw_bytes: 1024, ts: 100, ...overrides }
}

function disk(name: string): DiskSmart {
  return {
    disk: name,
    model: 'model',
    serial: 'serial',
    collected_at: '2026-01-02T03:04:05Z',
  }
}

const HOUR = 3600

describe('groupWearByDisk', () => {
  it('should group entries per disk preserving log order', () => {
    // Arrange
    const log = [
      entry({ disk: 'nvme1', ts: 3 }),
      entry({ disk: 'nvme0', ts: 2 }),
      entry({ disk: 'nvme1', ts: 1 }),
    ]

    // Act
    const grouped = groupWearByDisk(log)

    // Assert
    expect(grouped.get('nvme1')?.map((item) => item.ts)).toEqual([3, 1])
    expect(grouped.get('nvme0')?.map((item) => item.ts)).toEqual([2])
  })
})

describe('wearDiskNames', () => {
  it('should merge reported disks with log disks, sorted and unique', () => {
    // Arrange
    const grouped = groupWearByDisk([entry({ disk: 'nvme2' }), entry({ disk: 'nvme0' })])
    const smart = [disk('nvme1'), disk('nvme0')]

    // Act
    const names = wearDiskNames(smart, grouped)

    // Assert
    expect(names).toEqual(['nvme0', 'nvme1', 'nvme2'])
  })
})

describe('wearLogFor', () => {
  it('should default to the visible row count', () => {
    // Arrange
    const log = Array.from({ length: WEAR_LOG_ROWS + 5 }, (_, index) =>
      entry({ ts: WEAR_LOG_ROWS + 5 - index }),
    )

    // Act
    const rows = wearLogFor(groupWearByDisk(log), 'nvme0')

    // Assert
    expect(rows).toHaveLength(WEAR_LOG_ROWS)
  })

  it('should return the newest entries of a disk capped by the limit', () => {
    // Arrange
    const grouped = groupWearByDisk([entry({ ts: 3 }), entry({ ts: 2 }), entry({ ts: 1 })])

    // Act
    const rows = wearLogFor(grouped, 'nvme0', 2)

    // Assert
    expect(rows.map((item) => item.ts)).toEqual([3, 2])
  })

  it('should return an empty list for an unknown disk', () => {
    // Arrange
    const grouped = groupWearByDisk([])

    // Act
    const rows = wearLogFor(grouped, 'nvme9')

    // Assert
    expect(rows).toEqual([])
  })
})

describe('tbwSeries', () => {
  it('should build one ascending series per disk, skipping missing TBW', () => {
    // Arrange
    const now = 100 * HOUR
    const log = [
      entry({ disk: 'nvme1', ts: now - 2 * HOUR, tbw_bytes: 20 }),
      entry({ disk: 'nvme0', ts: now - 1 * HOUR, tbw_bytes: 10 }),
      entry({ disk: 'nvme0', ts: now - 3 * HOUR, tbw_bytes: 5 }),
      entry({ disk: 'nvme0', ts: now, tbw_bytes: null }),
    ]

    // Act
    const series = tbwSeries(log)

    // Assert
    expect(series.map((item) => item.label)).toEqual(['nvme0', 'nvme1'])
    expect(series[0].points.map((point) => point.value)).toEqual([5, 10])
  })

  it('should keep the newest value when two samples share a timestamp', () => {
    // Arrange
    const now = 100 * HOUR
    const log = [entry({ ts: now - HOUR, tbw_bytes: 10 }), entry({ ts: now - HOUR, tbw_bytes: 30 })]

    // Act
    const series = tbwSeries(log)

    // Assert
    expect(series[0].points).toEqual([{ ts: now - HOUR, value: 30 }])
  })
})
