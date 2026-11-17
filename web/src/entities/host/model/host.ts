import type { SeriesSpec } from '@/shared/lib'

export const cpuSeries: SeriesSpec[] = [{ kind: 'host_cpu', name: 'host', label: 'CPU' }]
export const ramSeries: SeriesSpec[] = [{ kind: 'host_mem', name: 'host', label: 'RAM' }]
export const loadSeries: SeriesSpec[] = [
  { kind: 'host_load1', name: 'host', label: 'load 1m' },
  { kind: 'host_load5', name: 'host', label: 'load 5m' },
  { kind: 'host_load15', name: 'host', label: 'load 15m' },
]
export const netSeries: SeriesSpec[] = [
  { kind: 'host_net_rx', name: 'host', label: 'Rx' },
  { kind: 'host_net_tx', name: 'host', label: 'Tx' },
]
