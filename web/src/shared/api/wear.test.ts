import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchWear } from './index'
import { jsonResponse, mockFetch } from '@/test/fetch'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('fetchWear', () => {
  it('should use the default limit', async () => {
    // Arrange
    const fetchMock = mockFetch()
    fetchMock.mockResolvedValue(jsonResponse({ latest: [], log: [] }))

    // Act
    await fetchWear()

    // Assert
    expect(fetchMock.mock.calls[0][0]).toBe('/api/wear?limit=200')
  })

  it('should use a custom limit', async () => {
    // Arrange
    const fetchMock = mockFetch()
    fetchMock.mockResolvedValue(jsonResponse({ latest: [], log: [] }))

    // Act
    await fetchWear(5)

    // Assert
    expect(fetchMock.mock.calls[0][0]).toBe('/api/wear?limit=5')
  })
})
