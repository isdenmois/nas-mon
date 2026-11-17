import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchHistory } from './index'
import { jsonResponse, mockFetch } from '@/test/fetch'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('fetchHistory', () => {
  it('should encode the range and every requested series', async () => {
    // Arrange
    const payload = { step: '1h', from: 1, to: 2, series: [] }
    const fetchMock = mockFetch()
    fetchMock.mockResolvedValue(jsonResponse(payload))

    // Act
    await fetchHistory('7d', ['temp:a', 'host_cpu'])

    // Assert
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/history?range=7d&series=temp%3Aa&series=host_cpu',
      expect.objectContaining({ headers: { Accept: 'application/json' } }),
    )
  })
})
