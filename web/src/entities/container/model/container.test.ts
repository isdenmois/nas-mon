import { describe, expect, it } from 'vitest'
import type { ContainerSnapshot } from '@/shared/api'
import { CHART_PALETTE } from '@/shared/lib'
import { containerColor, sortByCpu } from './container'

function container(name: string, cpu: number): ContainerSnapshot {
  return {
    name,
    id: name,
    image: `${name}:latest`,
    cpu_pct: cpu,
    mem_used: 0,
    mem_limit: 0,
    mem_pct: 0,
    net_rx_bps: 0,
    net_tx_bps: 0,
  }
}

describe('sortByCpu', () => {
  it('should order containers by CPU descending without mutating the input', () => {
    // Arrange
    const containers = [container('low', 1), container('high', 90), container('mid', 40)]

    // Act
    const result = sortByCpu(containers)

    // Assert
    expect(result.map((item) => item.name)).toEqual(['high', 'mid', 'low'])
    expect(containers.map((item) => item.name)).toEqual(['low', 'high', 'mid'])
  })
})

describe('containerColor', () => {
  it('should give neighbouring containers distinct palette colors', () => {
    // Arrange
    const names = ['mon', 'postgres', 'jellyfin']

    // Act
    const colors = names.map((name) => containerColor(names, name))

    // Assert
    expect(new Set(colors).size).toBe(3)
    expect(colors[0]).toBe(CHART_PALETTE[0])
  })

  it('should wrap around the palette for later containers', () => {
    // Arrange
    const names = Array.from({ length: CHART_PALETTE.length + 1 }, (_, index) => `c${index}`)

    // Act
    const color = containerColor(names, names[CHART_PALETTE.length])

    // Assert
    expect(color).toBe(CHART_PALETTE[0])
  })
})
