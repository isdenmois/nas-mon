<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  errorMessage,
  fetchWear,
  isAbortError,
  type DiskSmart,
  type WearResponse,
} from '@/shared/api'
import type { RangeKey } from '@/shared/config'
import { formatBytes, formatDateTime, formatNum } from '@/shared/lib'
import { RangePicker, SeriesChart } from '@/shared/ui'
import {
  groupWearByDisk,
  tbwSeries,
  WEAR_LOG_FETCH_LIMIT,
  WEAR_LOG_ROWS,
  wearDiskNames,
  wearLogFor,
} from '../model/disk'

const props = defineProps<{ smart: DiskSmart[]; error?: string }>()
const emit = defineEmits<{ error: [message: string] }>()

const wear = ref<WearResponse | null>(null)
const loading = ref(false)
const loadError = ref('')
const range = ref<RangeKey>('30d')
const logDisk = ref('')

let controller: AbortController | null = null
let requestId = 0
let timer: ReturnType<typeof setInterval> | undefined

const grouped = computed(() => groupWearByDisk(wear.value?.log ?? []))
const logDisks = computed(() => wearDiskNames(props.smart, grouped.value))
const selectedLog = computed(() => wearLogFor(grouped.value, logDisk.value))
const tbwDatasets = computed(() => tbwSeries(wear.value?.log ?? []))
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
    const data = await fetchWear(range.value, WEAR_LOG_FETCH_LIMIT, ctrl.signal)
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

watch(range, () => {
  void loadWear()
})

onMounted(() => {
  void loadWear()
  // SMART rows change at most once per MON_SMART_INTERVAL (12h by default).
  timer = setInterval(
    () => {
      void loadWear()
    },
    12 * 60 * 60_000,
  )
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
        <RangePicker v-model:range="range" label="TBW history" group-label="TBW history range" />
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
      <p v-if="selectedLog.length >= WEAR_LOG_ROWS" class="muted">
        Showing the latest {{ WEAR_LOG_ROWS }} entries.
      </p>
      <p class="muted log-note">
        Logged only on significant changes: new errors, more wear, or at least 100 GB of new
        writes. Power-on hours and temperature alone do not create a row.
      </p>
    </template>
  </section>
</template>
