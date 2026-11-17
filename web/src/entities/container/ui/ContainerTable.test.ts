import { render, screen, within } from '@testing-library/vue'
import { describe, expect, it } from 'vitest'
import type { ContainerSnapshot } from '@/shared/api'
import ContainerTable from './ContainerTable.vue'

function container(overrides: Partial<ContainerSnapshot> = {}): ContainerSnapshot {
  return {
    name: 'mon',
    id: 'c0c0c0c0c0c0',
    image: 'mon:latest',
    cpu_pct: 1.2,
    mem_used: 48 * 1024 ** 2,
    mem_limit: 512 * 1024 ** 2,
    mem_pct: 9.4,
    net_rx_bps: 2048,
    net_tx_bps: 1024,
    ...overrides,
  }
}

function containerNames(): string[] {
  const [, ...rows] = screen.getAllByRole('row')
  return rows.map((row) => within(row).getAllByRole('cell')[0].textContent?.trim() ?? '')
}

describe('ContainerTable', () => {
  it('should sort containers by CPU usage descending', () => {
    // Arrange
    const containers = [
      container({ name: 'low', cpu_pct: 1 }),
      container({ name: 'high', cpu_pct: 90 }),
      container({ name: 'mid', cpu_pct: 40 }),
    ]

    // Act
    render(ContainerTable, { props: { containers } })

    // Assert
    expect(containerNames()).toEqual(['high', 'mid', 'low'])
    expect(screen.getByText('3 running')).toBeInTheDocument()
  })

  it('should format CPU, memory, and network values', () => {
    // Arrange
    const containers = [
      container({
        cpu_pct: 12.5,
        mem_used: 512 * 1024 ** 2,
        mem_limit: 1024 ** 3,
        mem_pct: 50,
        net_rx_bps: 1024,
        net_tx_bps: 2048,
      }),
    ]

    // Act
    render(ContainerTable, { props: { containers } })

    // Assert
    const row = screen.getAllByRole('row')[1]
    expect(row).toHaveTextContent(/12[.,]5%/)
    expect(row).toHaveTextContent(/512 MB .* 1[.,]0 GB/)
    expect(row).toHaveTextContent(/1[.,]0 KB\/s/)
    expect(row).toHaveTextContent(/2[.,]0 KB\/s/)
  })

  it('should show an empty state when no containers are running', () => {
    // Arrange
    const containers: ContainerSnapshot[] = []

    // Act
    render(ContainerTable, { props: { containers } })

    // Assert
    expect(screen.getByText('No running containers')).toBeInTheDocument()
  })
})
