import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, errorMessage, fetchHistory, fetchOverview, fetchWear, isAbortError } from './api'

function mockFetch() {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function jsonResponse(body: unknown) {
  return {
    ok: true,
    status: 200,
    statusText: 'OK',
    json: async () => body,
  }
}

function errorResponse(status: number, statusText: string, body: () => Promise<unknown>) {
  return {
    ok: false,
    status,
    statusText,
    json: body,
  }
}

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

describe('request failures', () => {
  it('should throw an ApiError with the JSON error message', async () => {
    // Arrange
    const fetchMock = mockFetch()
    fetchMock.mockResolvedValue(
      errorResponse(503, 'Service Unavailable', async () => ({ error: 'db down' })),
    )

    // Act
    const error = await fetchOverview().catch((reason: unknown) => reason)

    // Assert
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ status: 503, message: 'db down' })
  })

  it('should fall back to the HTTP status when the body is not JSON', async () => {
    // Arrange
    const fetchMock = mockFetch()
    fetchMock.mockResolvedValue(
      errorResponse(500, 'Internal Server Error', async () => {
        throw new SyntaxError('not json')
      }),
    )

    // Act
    const error = await fetchOverview().catch((reason: unknown) => reason)

    // Assert
    expect(error).toMatchObject({ status: 500, message: 'HTTP 500 Internal Server Error' })
  })

  it('should fall back to the HTTP status when the error field is absent', async () => {
    // Arrange
    const fetchMock = mockFetch()
    fetchMock.mockResolvedValue(errorResponse(500, 'Internal Server Error', async () => ({})))

    // Act
    const error = await fetchOverview().catch((reason: unknown) => reason)

    // Assert
    expect(error).toMatchObject({ status: 500, message: 'HTTP 500 Internal Server Error' })
  })
})

describe('isAbortError', () => {
  it('should recognize a DOMException abort', () => {
    // Arrange
    const error = new DOMException('aborted', 'AbortError')

    // Act
    const result = isAbortError(error)

    // Assert
    expect(result).toBe(true)
  })

  it('should recognize an Error named AbortError', () => {
    // Arrange
    const error = Object.assign(new Error('aborted'), { name: 'AbortError' })

    // Act
    const result = isAbortError(error)

    // Assert
    expect(result).toBe(true)
  })

  it('should reject unrelated errors and values', () => {
    // Arrange
    const values = [new Error('boom'), 'boom']

    // Act
    const results = values.map((value) => isAbortError(value))

    // Assert
    expect(results).toEqual([false, false])
  })
})

describe('errorMessage', () => {
  it('should return the message of an Error', () => {
    // Arrange
    const error = new Error('failed')

    // Act
    const result = errorMessage(error)

    // Assert
    expect(result).toBe('failed')
  })

  it('should stringify non-Error values', () => {
    // Arrange
    const value = 'failed'

    // Act
    const result = errorMessage(value)

    // Assert
    expect(result).toBe('failed')
  })
})
