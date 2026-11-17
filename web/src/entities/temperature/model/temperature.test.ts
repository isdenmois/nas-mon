import { describe, expect, it } from 'vitest'
import type { TempReading } from '@/shared/api'
import { temperatureLabel } from './temperature'

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

describe('temperatureLabel', () => {
  it('should combine the chip and label', () => {
    // Arrange
    const reading = sensor()

    // Act
    const label = temperatureLabel(reading)

    // Assert
    expect(label).toBe('k10temp · Tctl')
  })

  it('should fall back to the sensor key when the label is empty', () => {
    // Arrange
    const reading = sensor({ label: '' })

    // Act
    const label = temperatureLabel(reading)

    // Assert
    expect(label).toBe('k10temp:Tctl')
  })
})
