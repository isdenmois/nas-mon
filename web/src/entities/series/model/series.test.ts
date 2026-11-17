import { describe, expect, it } from 'vitest'
import type { SeriesDescriptor } from '@/shared/api'
import { seriesIndex } from './series'

function descriptor(overrides: Partial<SeriesDescriptor> = {}): SeriesDescriptor {
  return { id: 1, kind: 'temp', name: 'k10temp:Tctl', ...overrides }
}

describe('seriesIndex', () => {
  it('should resolve a kind and name to the server id', () => {
    // Arrange
    const index = seriesIndex([descriptor()])

    // Act
    const id = index.id('temp', 'k10temp:Tctl')

    // Assert
    expect(id).toBe(1)
  })

  it('should keep names containing colons unambiguous', () => {
    // Arrange
    const index = seriesIndex([
      descriptor({ id: 4, name: 'nvme:nvme0:Composite' }),
      descriptor({ id: 5, name: 'nvme0' }),
    ])

    // Act
    const composite = index.id('temp', 'nvme:nvme0:Composite')
    const plain = index.id('temp', 'nvme0')

    // Assert
    expect(composite).toBe(4)
    expect(plain).toBe(5)
  })

  it('should return undefined for a series that has not been recorded', () => {
    // Arrange
    const index = seriesIndex([descriptor()])

    // Act
    const id = index.id('host_cpu', 'host')

    // Assert
    expect(id).toBeUndefined()
  })

  it('should drop unresolved specs instead of charting a guessed id', () => {
    // Arrange
    const index = seriesIndex([descriptor({ id: 9, kind: 'host_cpu', name: 'host' })])

    // Act
    const resolved = index.resolve([
      { kind: 'host_cpu', name: 'host', label: 'CPU' },
      { kind: 'host_mem', name: 'host', label: 'RAM' },
    ])

    // Assert
    expect(resolved).toEqual([{ id: 9, label: 'CPU', color: undefined }])
  })

  it('should tolerate a missing registry', () => {
    // Arrange
    const index = seriesIndex(undefined)

    // Act
    const resolved = index.resolve([{ kind: 'temp', name: 'x', label: 'x' }])

    // Assert
    expect(resolved).toEqual([])
  })
})
