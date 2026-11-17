import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchOverview } from './index'
import { jsonResponse, mockFetch } from '@/test/fetch'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('fetchOverview', () => {
  it('should request the overview and return parsed JSON', async () => {
    // Arrange
    const payload = { ts: '2026-01-02T03:04:05Z', temps: [] }
    const fetchMock = mockFetch()
    fetchMock.mockResolvedValue(jsonResponse(payload))
    const controller = new AbortController()

    // Act
    const result = await fetchOverview(controller.signal)

    // Assert
    expect(result).toEqual(payload)
    expect(fetchMock).toHaveBeenCalledWith('/api/overview', {
      signal: controller.signal,
      headers: { Accept: 'application/json' },
    })
  })
})
