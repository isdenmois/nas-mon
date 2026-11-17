import type { WearResponse } from './contracts'
import { request } from './client'

export function fetchWear(limit = 200, signal?: AbortSignal): Promise<WearResponse> {
  return request<WearResponse>(`/api/wear?limit=${limit}`, signal)
}
