import type { RangeKey } from '../config'
import type { HistoryResponse } from './contracts'
import { request } from './client'

export function fetchHistory(
  range: RangeKey,
  ids: number[],
  signal?: AbortSignal,
): Promise<HistoryResponse> {
  const query = new URLSearchParams({ range })
  for (const id of ids) query.append('id', String(id))
  return request<HistoryResponse>(`/api/history?${query.toString()}`, signal)
}
