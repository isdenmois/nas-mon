import { describe, expect, it } from 'vitest'
import {
  clampPct,
  formatAxis,
  formatBytes,
  formatDateTime,
  formatNum,
  formatPct,
  formatHours,
  formatRate,
  formatValue,
} from './format'

describe('formatBytes', () => {
  it('should format raw bytes', () => {
    // Arrange
    const value = 512

    // Act
    const result = formatBytes(value)

    // Assert
    expect(result).toBe('512 B')
  })

  it('should format kilobytes with one decimal', () => {
    // Arrange
    const value = 1536

    // Act
    const result = formatBytes(value)

    // Assert
    expect(result).toMatch(/^1[.,]5 KB$/)
  })

  it('should omit decimals for values of 100 units or more', () => {
    // Arrange
    const value = 100 * 1024

    // Act
    const result = formatBytes(value)

    // Assert
    expect(result).toBe('100 KB')
  })

  it('should clamp negative values to zero', () => {
    // Arrange
    const value = -10

    // Act
    const result = formatBytes(value)

    // Assert
    expect(result).toBe('0 B')
  })

  it('should return an em dash for missing or non-finite values', () => {
    // Arrange
    const values = [null, undefined, Number.NaN, Number.POSITIVE_INFINITY]

    // Act
    const results = values.map((value) => formatBytes(value))

    // Assert
    expect(results).toEqual(['—', '—', '—', '—'])
  })
})

describe('formatRate', () => {
  it('should append a per-second suffix', () => {
    // Arrange
    const value = 1024

    // Act
    const result = formatRate(value)

    // Assert
    expect(result).toMatch(/^1[.,]0 KB\/s$/)
  })

  it('should return an em dash for missing values', () => {
    // Arrange
    const value = undefined

    // Act
    const result = formatRate(value)

    // Assert
    expect(result).toBe('—')
  })
})

describe('formatPct', () => {
  it('should format a percentage', () => {
    // Arrange
    const value = 12.5

    // Act
    const result = formatPct(value)

    // Assert
    expect(result).toMatch(/^12[.,]5%$/)
  })

  it('should return an em dash for non-finite values', () => {
    // Arrange
    const value = Number.POSITIVE_INFINITY

    // Act
    const result = formatPct(value)

    // Assert
    expect(result).toBe('—')
  })
})

describe('formatNum', () => {
  it('should format a number with at most one decimal', () => {
    // Arrange
    const value = 42.25

    // Act
    const result = formatNum(value)

    // Assert
    expect(result).toMatch(/^42[.,][23]$/)
  })

  it('should return an em dash for non-finite values', () => {
    // Arrange
    const value = Number.NaN

    // Act
    const result = formatNum(value)

    // Assert
    expect(result).toBe('—')
  })
})

describe('formatValue', () => {
  it('should append a temperature unit', () => {
    // Arrange
    const value = 54.2

    // Act
    const result = formatValue(value, 'temperature')

    // Assert
    expect(result).toMatch(/^54[.,]2 °C$/)
  })

  it('should delegate percentages to formatPct', () => {
    // Arrange
    const value = 10

    // Act
    const result = formatValue(value, 'percent')

    // Assert
    expect(result).toMatch(/^10%$/)
  })

  it('should delegate rates to formatRate', () => {
    // Arrange
    const value = 2048

    // Act
    const result = formatValue(value, 'rate')

    // Assert
    expect(result).toMatch(/^2[.,]0 KB\/s$/)
  })

  it('should render a missing value as a dash', () => {
    // Arrange
    const value = null

    // Act
    const result = formatValue(value, 'temperature')

    // Assert
    expect(result).toBe('—')
  })
})

describe('formatHours', () => {
  it('should append the hour unit', () => {
    // Arrange
    const value = 12034

    // Act
    const result = formatHours(value)

    // Assert
    expect(result).toMatch(/^12[.,]034 h$/)
  })

  it('should keep the dash for a missing value', () => {
    // Arrange
    const value = undefined

    // Act
    const result = formatHours(value)

    // Assert
    expect(result).toBe('—')
  })
})

describe('clampPct', () => {
  it('should keep values inside the range unchanged', () => {
    // Arrange
    const value = 42

    // Act
    const result = clampPct(value)

    // Assert
    expect(result).toBe(42)
  })

  it('should clamp negative values to zero', () => {
    // Arrange
    const value = -5

    // Act
    const result = clampPct(value)

    // Assert
    expect(result).toBe(0)
  })

  it('should clamp values above one hundred', () => {
    // Arrange
    const value = 150

    // Act
    const result = clampPct(value)

    // Assert
    expect(result).toBe(100)
  })

  it('should return zero for non-finite values', () => {
    // Arrange
    const value = Number.NaN

    // Act
    const result = clampPct(value)

    // Assert
    expect(result).toBe(0)
  })
})

describe('formatDateTime', () => {
  it('should format an ISO timestamp', () => {
    // Arrange
    const value = '2026-06-15T12:00:00Z'

    // Act
    const result = formatDateTime(value)

    // Assert
    expect(result).toContain('2026')
    expect(result).not.toBe('—')
  })

  it('should format Unix seconds', () => {
    // Arrange
    const value = 0

    // Act
    const result = formatDateTime(value)

    // Assert
    expect(result).not.toBe('—')
  })

  it('should return an em dash for empty or invalid dates', () => {
    // Arrange
    const values = ['', 'not-a-date']

    // Act
    const results = values.map((value) => formatDateTime(value))

    // Assert
    expect(results).toEqual(['—', '—'])
  })
})

describe('formatAxis', () => {
  it('should format a short-range axis label as a time', () => {
    // Arrange
    const timestamp = Date.UTC(2026, 0, 2, 3, 4, 5) / 1000

    // Act
    const result = formatAxis(timestamp, '1h')

    // Assert
    expect(result).not.toBe('')
    expect(result).not.toBe('—')
  })

  it('should include the year for long-range axis labels', () => {
    // Arrange
    const timestamp = Date.UTC(2026, 0, 2, 3, 4, 5) / 1000

    // Act
    const result = formatAxis(timestamp, '1y')

    // Assert
    expect(result).toContain('2026')
  })
})
