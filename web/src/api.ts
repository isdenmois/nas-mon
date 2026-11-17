import type { HistoryResponse, Overview, RangeKey, WearResponse } from './types'

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(url: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(url, {
    signal,
    headers: { Accept: 'application/json' },
  })
  if (!res.ok) {
    let detail = ''
    try {
      const body = (await res.json()) as { error?: string }
      detail = body.error ?? ''
    } catch {
      // The response was not JSON; fall back to the HTTP status.
    }
    throw new ApiError(res.status, detail || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as T
}

export function fetchOverview(signal?: AbortSignal): Promise<Overview> {
  return request<Overview>('/api/overview', signal)
}

export function fetchHistory(
  range: RangeKey,
  series: string[],
  signal?: AbortSignal,
): Promise<HistoryResponse> {
  const query = new URLSearchParams({ range })
  for (const item of series) query.append('series', item)
  return request<HistoryResponse>(`/api/history?${query.toString()}`, signal)
}

export function fetchWear(limit = 200, signal?: AbortSignal): Promise<WearResponse> {
  return request<WearResponse>(`/api/wear?limit=${limit}`, signal)
}

export function isAbortError(err: unknown): boolean {
  return (
    (err instanceof DOMException && err.name === 'AbortError') ||
    (err instanceof Error && err.name === 'AbortError')
  )
}

export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}
