import type {
  HistoryResponse,
  Overview,
  SeriesHistory,
  WearResponse,
} from '../web/src/shared/api/contracts'

// All fixtures are anchored to this instant. The browser clock is frozen to it
// in every spec so timestamps, chart axes and the wear-log range stay stable.
export const FIXED_NOW = new Date('2026-01-02T03:04:05.000Z')
export const FIXED_EPOCH = Math.floor(FIXED_NOW.getTime() / 1000)

const secondsAgo = (seconds: number): string =>
  new Date((FIXED_EPOCH - seconds) * 1000).toISOString()

export const overview: Overview = {
  ts: FIXED_NOW.toISOString(),
  temps: [
    { key: 'k10temp:Tctl', chip: 'k10temp', label: 'Tctl', celsius: 54.2, warn: false },
    { key: 'nvme:nvme0', chip: 'nvme', label: 'Composite', celsius: 42.8, warn: false },
    { key: 'nvme:nvme1', chip: 'nvme', label: 'Composite', celsius: 81.5, warn: true },
  ],
  host: {
    cpu_pct: 12.4,
    mem_used: 6.2 * 1024 ** 3,
    mem_total: 32 * 1024 ** 3,
    mem_pct: 19.4,
    load1: 0.42,
    load5: 0.55,
    load15: 0.61,
    net_rx_bps: 12.5 * 1024 ** 2,
    net_tx_bps: 3.1 * 1024 ** 2,
    updated_at: secondsAgo(0),
  },
  containers: [
    {
      name: 'mon',
      id: 'c0c0c0c0c0c0',
      image: 'mon:latest',
      cpu_pct: 1.2,
      mem_used: 48 * 1024 ** 2,
      mem_limit: 512 * 1024 ** 2,
      mem_pct: 9.4,
      net_rx_bps: 2_048,
      net_tx_bps: 1_024,
    },
    {
      name: 'jellyfin',
      id: 'a1a1a1a1a1a1',
      image: 'jellyfin/jellyfin:10.10.7',
      cpu_pct: 37.5,
      mem_used: 2.4 * 1024 ** 3,
      mem_limit: 8 * 1024 ** 3,
      mem_pct: 30,
      net_rx_bps: 8.5 * 1024 ** 2,
      net_tx_bps: 2.2 * 1024 ** 2,
    },
    {
      name: 'postgres',
      id: 'b2b2b2b2b2b2',
      image: 'postgres:17-alpine',
      cpu_pct: 4.1,
      mem_used: 320 * 1024 ** 2,
      mem_limit: 2 * 1024 ** 3,
      mem_pct: 15.6,
      net_rx_bps: 12_288,
      net_tx_bps: 8_192,
    },
  ],
  volumes: [
    {
      name: '/',
      container_path: '/volumes/root',
      used: 146 * 1024 ** 3,
      total: 512 * 1024 ** 3,
      used_pct: 28.5,
    },
    {
      name: '/mnt/media',
      container_path: '/volumes/media',
      used: 7.3 * 1024 ** 4,
      total: 8 * 1024 ** 4,
      used_pct: 91.3,
    },
  ],
  smart: [
    {
      disk: 'nvme0',
      model: 'Samsung SSD 990 PRO 2TB',
      serial: 'S6Z1NL0T123456',
      percentage_used: 7,
      tbw_bytes: 8 * 1024 ** 4,
      power_on_hours: 12034,
      media_errors: 0,
      reallocated: 0,
      temp_c: 42,
      collected_at: secondsAgo(60),
    },
    {
      disk: 'nvme1',
      model: 'WD_BLACK SN850X 4TB',
      serial: 'WD-BLACK-0001',
      percentage_used: 82,
      tbw_bytes: 60 * 1024 ** 4,
      power_on_hours: 20451,
      media_errors: 3,
      reallocated: 1,
      temp_c: 51,
      collected_at: secondsAgo(120),
    },
  ],
  collectors: {
    docker: { last_success: secondsAgo(15), duration_ms: 42, runs: 120 },
    host: { last_success: secondsAgo(15), duration_ms: 3, runs: 120 },
    smart: { last_success: secondsAgo(120), duration_ms: 1800, runs: 2 },
    temp: { last_success: secondsAgo(10), duration_ms: 1, runs: 180 },
    volume: { last_success: secondsAgo(60), duration_ms: 2, runs: 30 },
  },
}

export const emptyOverview: Overview = {
  ts: FIXED_NOW.toISOString(),
  temps: [],
  host: null,
  containers: [],
  volumes: [],
  smart: [],
  collectors: {},
}

export const wear: WearResponse = {
  latest: [
    {
      disk: 'nvme0',
      model: 'Samsung SSD 990 PRO 2TB',
      serial: 'S6Z1NL0T123456',
      percentage_used: 7,
      tbw_bytes: 12.8 * 1024 ** 4,
      power_on_hours: 12034,
      media_errors: 0,
      reallocated: 0,
      temp_c: 42,
      ts: FIXED_EPOCH - 10 * 86400,
      reason: 'TBW +100 GB',
    },
    {
      disk: 'nvme1',
      model: 'WD_BLACK SN850X 4TB',
      serial: 'WD-BLACK-0001',
      percentage_used: 82,
      tbw_bytes: 340 * 1024 ** 4,
      power_on_hours: 20451,
      media_errors: 3,
      reallocated: 1,
      temp_c: 51,
      ts: FIXED_EPOCH - 5 * 86400,
      reason: 'media errors 1 → 3',
    },
  ],
  log: [
    {
      disk: 'nvme0',
      percentage_used: 6,
      tbw_bytes: 12.0 * 1024 ** 4,
      power_on_hours: 11900,
      media_errors: 0,
      reallocated: 0,
      temp_c: 41,
      ts: FIXED_EPOCH - 29 * 86400,
      reason: 'first sample',
    },
    {
      disk: 'nvme0',
      percentage_used: 7,
      tbw_bytes: 12.4 * 1024 ** 4,
      power_on_hours: 11960,
      media_errors: 0,
      reallocated: 0,
      temp_c: 42,
      ts: FIXED_EPOCH - 20 * 86400,
      reason: 'wear increased',
    },
    {
      disk: 'nvme0',
      percentage_used: 7,
      tbw_bytes: 12.8 * 1024 ** 4,
      power_on_hours: 12034,
      media_errors: 0,
      reallocated: 0,
      temp_c: 42,
      ts: FIXED_EPOCH - 10 * 86400,
      reason: 'TBW +100 GB',
    },
    {
      disk: 'nvme1',
      percentage_used: 80,
      tbw_bytes: 330 * 1024 ** 4,
      power_on_hours: 20000,
      media_errors: 1,
      reallocated: 0,
      temp_c: 50,
      ts: FIXED_EPOCH - 25 * 86400,
      reason: 'media errors 0 → 1',
    },
    {
      disk: 'nvme1',
      percentage_used: 82,
      tbw_bytes: 340 * 1024 ** 4,
      power_on_hours: 20451,
      media_errors: 3,
      reallocated: 1,
      temp_c: 51,
      ts: FIXED_EPOCH - 5 * 86400,
      reason: 'media errors 1 → 3',
    },
  ],
}

const POINT_COUNT = 8
const STEP_SECONDS = 3600

function nameOffset(name: string): number {
  let sum = 0
  for (let index = 0; index < name.length; index += 1) sum += name.charCodeAt(index)
  return (sum % 7) - 3
}

function valueFor(kind: string, name: string, index: number): number {
  const offset = nameOffset(name)
  const ramp = 1 + index * 0.25
  switch (kind) {
    case 'temp':
      return 48 + index * 1.5 + offset
    case 'host_cpu':
      return 8 + index * 2.5
    case 'host_mem':
      return 58 + index * 0.5
    case 'host_load1':
      return 0.3 + index * 0.05
    case 'host_load5':
      return 0.45 + index * 0.03
    case 'host_load15':
      return 0.6 + index * 0.02
    case 'host_net_rx':
      return 12 * 1024 ** 2 * ramp
    case 'host_net_tx':
      return 3 * 1024 ** 2 * ramp
    case 'container_cpu':
      return 10 + index * 3 + offset * 2
    case 'container_mem':
      return 512 * 1024 ** 2 * ramp * (1 + offset * 0.05)
    case 'container_mem_pct':
      return 12 + index + offset
    case 'container_net_rx':
      return 1024 ** 2 * ramp * (1 + offset * 0.08)
    case 'container_net_tx':
      return 256 * 1024 * ramp * (1 + offset * 0.08)
    case 'volume_used_pct':
      return 40 + index
    default:
      return 1 + index
  }
}

// historyFor builds a deterministic answer for any series the UI asks for.
export function historyFor(requested: string[], range: string): HistoryResponse {
  const to = FIXED_EPOCH
  const from = to - POINT_COUNT * STEP_SECONDS

  const series: SeriesHistory[] = requested.map((key) => {
    const separator = key.indexOf(':')
    const kind = separator < 0 ? key : key.slice(0, separator)
    const name = separator < 0 ? '' : key.slice(separator + 1)
    return {
      kind,
      name,
      points: Array.from({ length: POINT_COUNT }, (_, index) => {
        const avg = valueFor(kind, name, index)
        return {
          ts: from + index * STEP_SECONDS,
          avg,
          min: avg * 0.9,
          max: avg * 1.1,
          n: 1,
        }
      }),
    }
  })

  return { step: range === '1h' ? '1m' : '1h', from, to, series }
}
