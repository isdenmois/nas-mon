// Wire contract of the mon HTTP API. Kept free of runtime code so e2e fixtures
// and the browser bundle can both import it without pulling in DOM types.

export interface TempReading {
  key: string
  chip: string
  label: string
  celsius: number
  warn: boolean
}

export interface HostSnapshot {
  cpu_pct: number
  mem_used: number
  mem_total: number
  mem_pct: number
  load1: number
  load5: number
  load15: number
  net_rx_bps: number
  net_tx_bps: number
  updated_at: string
}

export interface ContainerSnapshot {
  name: string
  id: string
  image: string
  cpu_pct: number
  mem_used: number
  mem_limit: number
  mem_pct: number
  net_rx_bps: number
  net_tx_bps: number
}

export interface VolumeSnapshot {
  name: string
  container_path: string
  used: number
  total: number
  used_pct: number
}

export interface DiskSmart {
  disk: string
  model: string
  serial: string
  percentage_used?: number | null
  tbw_bytes?: number | null
  power_on_hours?: number | null
  media_errors?: number | null
  reallocated?: number | null
  temp_c?: number | null
  collected_at: string
}

export interface CollectorStatus {
  last_success?: string
  last_error?: string
  duration_ms: number
  runs: number
}

export interface SeriesDescriptor {
  id: number
  kind: string
  name: string
}

export interface Overview {
  ts: string
  temps: TempReading[]
  host?: HostSnapshot | null
  containers: ContainerSnapshot[]
  volumes: VolumeSnapshot[]
  smart: DiskSmart[]
  collectors: Record<string, CollectorStatus>
  series: SeriesDescriptor[]
}

export interface SeriesPoint {
  ts: number
  avg: number
  min: number
  max: number
  n: number
}

export interface SeriesHistory {
  id: number
  kind: string
  name: string
  points: SeriesPoint[]
}

export interface HistoryResponse {
  step: string
  from: number
  to: number
  series: SeriesHistory[]
}

export interface Wear {
  disk: string
  model?: string
  serial?: string
  percentage_used?: number | null
  tbw_bytes?: number | null
  power_on_hours?: number | null
  media_errors?: number | null
  reallocated?: number | null
  temp_c?: number | null
  ts: number
  reason?: string
}

export interface WearResponse {
  latest: Wear[]
  log: Wear[]
}
