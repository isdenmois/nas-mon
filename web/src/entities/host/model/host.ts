import type { HistorySeries } from '@/shared/lib'

export const cpuSeries: HistorySeries[] = [{ kind: 'host_cpu', name: 'host', label: 'CPU' }]
export const ramSeries: HistorySeries[] = [{ kind: 'host_mem', name: 'host', label: 'RAM' }]
export const loadSeries: HistorySeries[] = [
  { kind: 'host_load1', name: 'host', label: 'load 1m' },
  { kind: 'host_load5', name: 'host', label: 'load 5m' },
  { kind: 'host_load15', name: 'host', label: 'load 15m' },
]
export const netSeries: HistorySeries[] = [
  { kind: 'host_net_rx', name: 'host', label: 'Rx' },
  { kind: 'host_net_tx', name: 'host', label: 'Tx' },
]
