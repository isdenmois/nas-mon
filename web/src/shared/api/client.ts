export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export async function request<T>(url: string, signal?: AbortSignal): Promise<T> {
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

export function isAbortError(err: unknown): boolean {
  return (
    (err instanceof DOMException && err.name === 'AbortError') ||
    (err instanceof Error && err.name === 'AbortError')
  )
}

export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}
