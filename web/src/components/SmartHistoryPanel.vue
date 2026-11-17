<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { errorMessage, fetchWear, isAbortError } from '../api'
import { formatBytes, formatDateTime, formatNum, rangeSeconds } from '../format'
import {
  RANGES,
  type ChartSeries,
  type DiskSmart,
  type RangeKey,
  type Wear,
  type WearResponse,
} from '../types'
import SeriesChart from './SeriesChart.vue'

const props = defineProps<{ smart: DiskSmart[]; error?: string }>()
const emit = defineEmits<{ error: [message: string] }>()

const palette = ['#60a5fa', '#f472b6', '#34d399', '#fbbf24', '#a78bfa', '#f87171', '#22d3ee', '#facc15']

const wear = ref<WearResponse | null>(null)
const loading = ref(false)
const loadError = ref('')
const range = ref<RangeKey>('30d')
const logDisk = ref('')

let controller: AbortController | null = null
let requestId = 0
let timer: ReturnType<typeof setInterval> | undefined

const logs = computed(() => {
  const byDisk = new Map<string, Wear[]>()
  for (const entry of wear.value?.log ?? []) {
    const list = byDisk.get(entry.disk)
    if (list) list.push(entry)
    else byDisk.set(entry.disk, [entry])
  }
  return byDisk
})

const logDisks = computed(() => {
  const names = new Set<string>(props.smart.map((disk) => disk.disk))
  for (const disk of logs.value.keys()) names.add(disk)
  return [...names].sort()
})

const selectedLog = computed(() => (logs.value.get(logDisk.value) ?? []).slice(0, 200))

const tbwDatasets = computed<ChartSeries[]>(() => {
  const window = rangeSeconds(range.value)
  const since = window == null ? null : Date.now() / 1000 - window
  const byDisk = new Map<string, Map<number, number>>()
  for (const entry of wear.value?.log ?? []) {
    if (entry.tbw_bytes == null) continue
    if (since != null && entry.ts < since) continue
    let points = byDisk.get(entry.disk)
    if (!points) {
      points = new Map()
      byDisk.set(entry.disk, points)
    }
    points.set(entry.ts, entry.tbw_bytes)
  }
  return [...byDisk.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([disk, points], index) => ({
      label: disk,
      color: palette[index % palette.length],
      points: [...points.entries()]
        .sort(([a], [b]) => a - b)
        .map(([ts, value]) => ({ ts, value })),
    }))
})

const tbwEmptyText = computed(() =>
  loading.value && !wear.value ? 'Loading write history…' : 'No TBW history yet',
)

watch(
  logDisks,
  (disks) => {
    if (!disks.includes(logDisk.value)) logDisk.value = disks[0] ?? ''
  },
  { immediate: true },
)

async function loadWear(): Promise<void> {
  controller?.abort()
  const ctrl = new AbortController()
  controller = ctrl
  const id = ++requestId
  loading.value = true
  loadError.value = ''
  try {
    const data = await fetchWear(20000, ctrl.signal)
    if (id !== requestId) return
    wear.value = data
  } catch (err) {
    if (id !== requestId || isAbortError(err)) return
    loadError.value = errorMessage(err)
    emit('error', `Wear history: ${loadError.value}`)
  } finally {
    if (id === requestId) loading.value = false
  }
}

onMounted(() => {
  void loadWear()
  // SMART rows change at most once per MON_SMART_INTERVAL (12h by default).
  timer = setInterval(() => {
    void loadWear()
  }, 12 * 60 * 60_000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  controller?.abort()
})
</script>

<template>
  <section class="panel" aria-labelledby="smart-history-title">
    <div class="panel-head">
      <h2 id="smart-history-title">SMART history</h2>
      <span class="muted">{{ logDisks.length }} {{ logDisks.length === 1 ? 'disk' : 'disks' }}</span>
    </div>

    <p v-if="logDisks.length === 0" class="empty" :class="{ 'inline-error': error }">
      {{
        loading && !wear
          ? 'Loading SMART history…'
          : error
            ? `SMART collector error: ${error}`
            : 'No SMART data'
      }}
    </p>

    <template v-else>
      <div class="panel-head chart-head">
        <h3>Lifetime written (TBW)</h3>
        <div class="ranges" role="group" aria-label="TBW history range">
          <button
            v-for="item in RANGES"
            :key="item"
            type="button"
            class="range-btn"
            :aria-pressed="range === item"
            :aria-label="`Show ${item} of TBW history`"
            @click="range = item"
          >
            {{ item }}
          </button>
        </div>
      </div>

      <SeriesChart
        :datasets="tbwDatasets"
        format="bytes"
        :range="range"
        height="220px"
        show-points
        :empty-text="tbwEmptyText"
      />

      <div class="panel-head chart-head log-head">
        <h3>Wear log</h3>
        <label class="select-label">
          <span>Disk</span>
          <select v-model="logDisk">
            <option v-for="disk in logDisks" :key="disk" :value="disk">{{ disk }}</option>
          </select>
        </label>
      </div>

      <p v-if="loading && !wear" class="muted">Loading wear log…</p>
      <p v-else-if="loadError" class="inline-error" role="status">
        Could not load wear history: {{ loadError }}
      </p>
      <div v-else-if="selectedLog.length > 0" class="table-wrap">
        <table class="inner-table">
          <thead>
            <tr>
              <th scope="col">Time</th>
              <th scope="col">Reason</th>
              <th scope="col" class="num">Wear</th>
              <th scope="col" class="num">TBW</th>
              <th scope="col" class="num">Power-on</th>
              <th scope="col" class="num">Media errors</th>
              <th scope="col" class="num">Temp</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(entry, index) in selectedLog" :key="`${entry.ts}-${index}`">
              <td>{{ formatDateTime(entry.ts) }}</td>
              <td>{{ entry.reason || '—' }}</td>
              <td class="num">{{ entry.percentage_used == null ? '—' : `${entry.percentage_used}%` }}</td>
              <td class="num">{{ formatBytes(entry.tbw_bytes) }}</td>
              <td class="num">{{ entry.power_on_hours == null ? '—' : `${formatNum(entry.power_on_hours)} h` }}</td>
              <td class="num">{{ entry.media_errors ?? '—' }}</td>
              <td class="num">{{ entry.temp_c == null ? '—' : `${entry.temp_c} °C` }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="muted">No wear log entries yet.</p>
      <p v-if="selectedLog.length >= 200" class="muted">Showing the latest 200 entries.</p>
      <p class="muted log-note">
        Logged only on significant changes: new errors, more wear, or at least 100 GB of new
        writes. Power-on hours and temperature alone do not create a row.
      </p>
    </template>
  </section>
</template>
