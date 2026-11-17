import { render, screen } from '@testing-library/vue'
import { describe, expect, it } from 'vitest'
import type { TempReading } from '../types'
import TempPanel from './TempPanel.vue'

function sensor(overrides: Partial<TempReading> = {}): TempReading {
  return {
    key: 'k10temp:Tctl',
    chip: 'k10temp',
    label: 'Tctl',
    celsius: 54.2,
    warn: false,
    ...overrides,
  }
}

describe('TempPanel', () => {
  it('should render a single sensor with its value and chip', () => {
    // Arrange
    render(TempPanel, { props: { temps: [sensor()] } })

    // Act
    const heading = screen.getByRole('heading', { name: 'Temperatures' })
    const value = screen.getByText(/^54[.,]2$/)

    // Assert
    expect(heading).toBeInTheDocument()
    expect(value).toBeInTheDocument()
    expect(screen.getByText('1 sensor')).toBeInTheDocument()
    expect(screen.getByText('k10temp')).toBeInTheDocument()
  })

  it('should render every sensor and use the plural label', () => {
    // Arrange
    const temps = [
      sensor(),
      sensor({ key: 'nvme:nvme0', chip: 'nvme', label: 'Composite', celsius: 42.8 }),
    ]

    // Act
    render(TempPanel, { props: { temps } })

    // Assert
    expect(screen.getByText('2 sensors')).toBeInTheDocument()
    expect(screen.getByText('Tctl')).toBeInTheDocument()
    expect(screen.getByText('Composite')).toBeInTheDocument()
  })

  it('should show a warning for sensors above their threshold', () => {
    // Arrange
    const temp = sensor({ key: 'nvme:nvme1', label: 'Composite', celsius: 81.5, warn: true })

    // Act
    render(TempPanel, { props: { temps: [temp] } })

    // Assert
    expect(screen.getByText('Warning')).toBeInTheDocument()
  })

  it('should fall back to the sensor key when the label is empty', () => {
    // Arrange
    const temp = sensor({ label: '' })

    // Act
    render(TempPanel, { props: { temps: [temp] } })

    // Assert
    expect(screen.getByText('k10temp:Tctl')).toBeInTheDocument()
  })

  it('should show an empty state when no sensors are reported', () => {
    // Arrange
    const temps: TempReading[] = []

    // Act
    render(TempPanel, { props: { temps } })

    // Assert
    expect(screen.getByText('No temperature sensors reported.')).toBeInTheDocument()
  })
})
