import { render, screen, within } from '@testing-library/vue'
import { describe, expect, it } from 'vitest'
import type { DiskSmart } from '../types'
import SmartPanel from './SmartPanel.vue'

function disk(overrides: Partial<DiskSmart> = {}): DiskSmart {
  return {
    disk: 'nvme0',
    model: 'Samsung SSD 990 PRO 2TB',
    serial: 'S6Z1NL0T123456',
    percentage_used: 7,
    tbw_bytes: 12.5 * 1024 ** 4,
    power_on_hours: 12034,
    media_errors: 0,
    reallocated: 0,
    temp_c: 42,
    collected_at: '2026-01-02T03:04:05Z',
    ...overrides,
  }
}

describe('SmartPanel', () => {
  it('should render a disk with its wear and hardware details', () => {
    // Arrange
    render(SmartPanel, { props: { smart: [disk()] } })

    // Act
    const row = screen.getAllByRole('row')[1]

    // Assert
    expect(screen.getByText('1 disk')).toBeInTheDocument()
    expect(row).toHaveTextContent('nvme0')
    expect(row).toHaveTextContent('Samsung SSD 990 PRO 2TB')
    expect(row).toHaveTextContent('7%')
    expect(row).toHaveTextContent(/12[.,]5 TB/)
    expect(row).toHaveTextContent('42 °C')
  })

  it('should render every disk and use the plural label', () => {
    // Arrange
    const disks = [disk(), disk({ disk: 'nvme1', model: 'WD_BLACK SN850X 4TB' })]

    // Act
    render(SmartPanel, { props: { smart: disks } })

    // Assert
    expect(screen.getByText('2 disks')).toBeInTheDocument()
    expect(screen.getByText('nvme0')).toBeInTheDocument()
    expect(screen.getByText('nvme1')).toBeInTheDocument()
  })

  it('should flag high wear', () => {
    // Arrange
    const disks = [disk({ percentage_used: 82 })]

    // Act
    render(SmartPanel, { props: { smart: disks } })

    // Assert
    expect(screen.getByText('82%')).toBeInTheDocument()
    expect(screen.getByText('high')).toBeInTheDocument()
  })

  it('should flag media errors and reallocated sectors', () => {
    // Arrange
    const disks = [disk({ media_errors: 3, reallocated: 1 })]

    // Act
    render(SmartPanel, { props: { smart: disks } })

    // Assert
    expect(screen.getByText('errors')).toBeInTheDocument()
    expect(screen.getByText('realloc')).toBeInTheDocument()
  })

  it('should render an em dash for missing optional values', () => {
    // Arrange
    const disks = [
      disk({
        percentage_used: null,
        tbw_bytes: null,
        power_on_hours: null,
        media_errors: null,
        reallocated: null,
        temp_c: null,
      }),
    ]

    // Act
    render(SmartPanel, { props: { smart: disks } })
    const row = screen.getAllByRole('row')[1]

    // Assert
    expect(within(row).getAllByText('—')).toHaveLength(6)
  })

  it('should show an empty state when no SMART data is available', () => {
    // Arrange
    const disks: DiskSmart[] = []

    // Act
    render(SmartPanel, { props: { smart: disks } })

    // Assert
    expect(screen.getByText('No SMART data')).toBeInTheDocument()
  })

  it('should surface a collector error when no disks are reported', () => {
    // Arrange
    const disks: DiskSmart[] = []

    // Act
    render(SmartPanel, { props: { smart: disks, error: 'smartctl failed' } })

    // Assert
    expect(screen.getByText('SMART collector error: smartctl failed')).toBeInTheDocument()
  })
})
