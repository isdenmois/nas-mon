import type { RangeKey } from '../config'
import type { HistoryResponse } from './contracts'
import { request } from './client'

export function fetchHistory(
  range: RangeKey,
  series: string[],
  signal?: AbortSignal,
): Promise<HistoryResponse> {
  const query = new URLSearchParams({ range })
  for (const item of series) query.append('series', item)
  return request<HistoryResponse>(`/api/history?${query.toString()}`, signal)
}
