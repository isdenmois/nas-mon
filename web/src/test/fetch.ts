import { vi } from 'vitest'

export function mockFetch() {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

export function jsonResponse(body: unknown) {
  return {
    ok: true,
    status: 200,
    statusText: 'OK',
    json: async () => body,
  }
}

export function errorResponse(status: number, statusText: string, body: () => Promise<unknown>) {
  return {
    ok: false,
    status,
    statusText,
    json: body,
  }
}
