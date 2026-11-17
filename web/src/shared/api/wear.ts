import type { RangeKey } from '@/shared/config'
import type { WearResponse } from './contracts'
import { request } from './client'

export function fetchWear(
  range: RangeKey,
  limit = 200,
  signal?: AbortSignal,
): Promise<WearResponse> {
  const query = new URLSearchParams({ limit: String(limit), range })
  return request<WearResponse>(`/api/wear?${query.toString()}`, signal)
}
