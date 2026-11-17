import { render, screen } from '@testing-library/vue'
import { describe, expect, it } from 'vitest'
import type { HostSnapshot } from '../types'
import HostPanel from './HostPanel.vue'

function host(overrides: Partial<HostSnapshot> = {}): HostSnapshot {
  return {
    cpu_pct: 12.4,
    mem_used: 8 * 1024 ** 3,
    mem_total: 16 * 1024 ** 3,
    mem_pct: 19.4,
    load1: 0.42,
    load5: 0.55,
    load15: 0.61,
    net_rx_bps: 12.5 * 1024 ** 2,
    net_tx_bps: 3.1 * 1024 ** 2,
    updated_at: '2026-01-02T03:04:05Z',
    ...overrides,
  }
}

describe('HostPanel', () => {
  it('should render every host metric section', () => {
    // Arrange
    const snapshot = host()

    // Act
    render(HostPanel, { props: { host: snapshot } })

    // Assert
    expect(screen.getByRole('heading', { name: 'Host' })).toBeInTheDocument()
    expect(screen.getByText('CPU')).toBeInTheDocument()
    expect(screen.getByText('RAM')).toBeInTheDocument()
    expect(screen.getByText('Load average')).toBeInTheDocument()
    expect(screen.getByText('Network')).toBeInTheDocument()
  })

  it('should expose rounded CPU and memory values to assistive technology', () => {
    // Arrange
    const snapshot = host()

    // Act
    render(HostPanel, { props: { host: snapshot } })

    // Assert
    expect(screen.getByRole('progressbar', { name: 'CPU usage' })).toHaveAttribute(
      'aria-valuenow',
      '12',
    )
    expect(screen.getByRole('progressbar', { name: 'Memory usage' })).toHaveAttribute(
      'aria-valuenow',
      '19',
    )
  })

  it('should show the formatted update timestamp', () => {
    // Arrange
    const snapshot = host()

    // Act
    render(HostPanel, { props: { host: snapshot } })

    // Assert
    expect(screen.getByText(/updated .*2026/)).toBeInTheDocument()
  })

  it('should show an empty state when host metrics are unavailable', () => {
    // Arrange
    const snapshot = null

    // Act
    render(HostPanel, { props: { host: snapshot } })

    // Assert
    expect(screen.getByText('Host metrics unavailable')).toBeInTheDocument()
  })

  it('should clamp a memory percentage above one hundred', () => {
    // Arrange
    const snapshot = host({ mem_pct: 150 })

    // Act
    render(HostPanel, { props: { host: snapshot } })

    // Assert
    expect(screen.getByRole('progressbar', { name: 'Memory usage' })).toHaveAttribute(
      'aria-valuenow',
      '100',
    )
  })

  it('should clamp a negative CPU percentage to zero', () => {
    // Arrange
    const snapshot = host({ cpu_pct: -5 })

    // Act
    render(HostPanel, { props: { host: snapshot } })

    // Assert
    expect(screen.getByRole('progressbar', { name: 'CPU usage' })).toHaveAttribute(
      'aria-valuenow',
      '0',
    )
  })
})
