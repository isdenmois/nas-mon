import type { Overview } from './contracts'
import { request } from './client'

export function fetchOverview(signal?: AbortSignal): Promise<Overview> {
  return request<Overview>('/api/overview', signal)
}
