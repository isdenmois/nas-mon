import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, errorMessage, fetchOverview, isAbortError } from './index'
import { errorResponse, mockFetch } from '@/test/fetch'

afterEach(() => {
  vi.unstubAllGlobals()
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
