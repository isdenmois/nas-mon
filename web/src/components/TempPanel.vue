<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  CategoryScale,
  Chart,
  Legend,
  LineController,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
  type ChartData,
} from 'chart.js'
import { errorMessage, fetchHistory, isAbortError } from '../api'
import { formatAxis, formatNum } from '../format'
import { RANGES, type RangeKey, type SeriesHistory, type TempReading } from '../types'

const props = defineProps<{ temps: TempReading[] }>()
const emit = defineEmits<{ error: [message: string] }>()

Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Tooltip, Legend)

const range = ref<RangeKey>('24h')
const selected = ref<string[]>([])
const history = ref<SeriesHistory[]>([])
const loading = ref(false)
const error = ref('')
const canvas = ref<HTMLCanvasElement | null>(null)

let chart: Chart | null = null
let controller: AbortController | null = null
let requestId = 0
let selectionInitialized = false
let timer: ReturnType<typeof setInterval> | undefined

const hasPoints = computed(() => history.value.some((series) => series.points.length > 0))
const chartLabel = computed(() => {
  const names = history.value.filter((series) => series.points.length > 0).map((series) => series.name)
  return names.length > 0
    ? `Line chart of temperature history for ${names.join(', ')}`
    : 'No temperature history available'
})

function sensorLabel(sensor: TempReading): string {
  return sensor.label ? `${sensor.chip} · ${sensor.label}` : sensor.key
}

watch(
  () => props.temps.map((sensor) => sensor.key).join('\u0000'),
  () => {
    const keys = props.temps.map((sensor) => sensor.key)
    if (!selectionInitialized) {
      if (keys.length === 0) return
      selectionInitialized = true
      selected.value = [...keys]
      return
    }
    const available = new Set(keys)
    const kept = selected.value.filter((key) => available.has(key))
    if (kept.length !== selected.value.length) selected.value = kept
  },
  { immediate: true },
)

async function load(): Promise<void> {
  if (selected.value.length === 0) {
    controller?.abort()
    requestId += 1
    history.value = []
    loading.value = false
    error.value = ''
    return
  }

  controller?.abort()
  const ctrl = new AbortController()
  controller = ctrl
  const id = ++requestId
  loading.value = true
  try {
    const data = await fetchHistory(
      range.value,
      selected.value.map((key) => `temp:${key}`),
      ctrl.signal,
    )
    if (id !== requestId) return
    history.value = data.series
    error.value = ''
  } catch (err) {
    if (id !== requestId || isAbortError(err)) return
    error.value = errorMessage(err)
    emit('error', `Temperature history: ${error.value}`)
  } finally {
    if (id === requestId) loading.value = false
  }
}

watch([range, () => selected.value.join('\u0000')], () => {
  void load()
})

function draw(): void {
  void nextTick(() => {
    if (!canvas.value || !hasPoints.value) {
      chart?.destroy()
      chart = null
      return
    }

    const stamps = new Set<number>()
    for (const series of history.value) {
      for (const point of series.points) stamps.add(point.ts)
    }
    const xs = [...stamps].sort((a, b) => a - b)
    const palette = ['#60a5fa', '#f472b6', '#34d399', '#fbbf24', '#a78bfa', '#f87171', '#22d3ee', '#facc15']
    const data: ChartData<'line'> = {
      labels: xs.map((ts) => formatAxis(ts, range.value)),
      datasets: history.value
        .filter((series) => series.points.length > 0)
        .map((series, index) => {
          const byTs = new Map(series.points.map((point) => [point.ts, point.avg]))
          const color = palette[index % palette.length]
          return {
            label: series.name,
            data: xs.map((ts) => byTs.get(ts) ?? null),
            borderColor: color,
            backgroundColor: `${color}33`,
            borderWidth: 2,
            pointRadius: 0,
            tension: 0.25,
            spanGaps: true,
            fill: false,
          }
        }),
    }

    if (chart) {
      chart.data = data
      chart.update()
      return
    }

    chart = new Chart(canvas.value, {
      type: 'line',
      data,
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        interaction: { mode: 'index', intersect: false },
        plugins: {
          legend: {
            labels: { color: '#cbd5e1', boxWidth: 10, boxHeight: 10, usePointStyle: true },
          },
          tooltip: {
            callbacks: {
              label: (ctx) => `${ctx.dataset.label ?? ''}: ${Number(ctx.parsed.y).toFixed(1)} °C`,
            },
          },
        },
        scales: {
          x: {
            ticks: { color: '#94a3b8', maxTicksLimit: 8, maxRotation: 0, autoSkip: true },
            grid: { color: 'rgba(148, 163, 184, 0.10)' },
          },
          y: {
            ticks: { color: '#94a3b8', callback: (value) => `${value} °C` },
            grid: { color: 'rgba(148, 163, 184, 0.10)' },
          },
        },
      },
    })
  })
}

watch([history, hasPoints, range], draw, { deep: true })

onMounted(() => {
  void load()
  timer = setInterval(() => {
    void load()
  }, 60_000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  controller?.abort()
  chart?.destroy()
  chart = null
})
</script>

<template>
  <section class="panel" aria-labelledby="temps-title">
    <div class="panel-head">
      <h2 id="temps-title">Temperatures</h2>
      <span class="muted">{{ temps.length }} {{ temps.length === 1 ? 'sensor' : 'sensors' }}</span>
    </div>

    <div v-if="temps.length > 0" class="temp-grid">
      <article v-for="sensor in temps" :key="sensor.key" class="temp-tile" :class="{ warn: sensor.warn }">
        <div class="temp-label" :title="sensor.key">{{ sensor.label || sensor.key }}</div>
        <div class="temp-value">{{ formatNum(sensor.celsius) }}<span class="unit">°C</span></div>
        <div class="temp-meta">
          <span :title="sensor.key">{{ sensor.chip }}</span>
          <span v-if="sensor.warn" class="warn-text">Warning</span>
        </div>
      </article>
    </div>
    <p v-else class="empty">No temperature sensors reported.</p>

    <div class="panel-head chart-head">
      <h3>Temperature history</h3>
      <div class="ranges" role="group" aria-label="History range">
        <button
          v-for="item in RANGES"
          :key="item"
          type="button"
          class="range-btn"
          :aria-pressed="range === item"
          :aria-label="`Show ${item} of temperature history`"
          @click="range = item"
        >
          {{ item }}
        </button>
      </div>
    </div>

    <div
      v-if="temps.length > 0"
      class="sensor-picker"
      role="group"
      aria-label="Temperature sensors shown in the chart"
    >
      <label
        v-for="sensor in temps"
        :key="sensor.key"
        class="sensor-check"
        :class="{ on: selected.includes(sensor.key) }"
      >
        <input v-model="selected" type="checkbox" :value="sensor.key" />
        <span :title="sensor.key">{{ sensorLabel(sensor) }}</span>
      </label>
    </div>

    <div class="chart-box">
      <canvas v-show="hasPoints" ref="canvas" role="img" :aria-label="chartLabel"></canvas>
      <p v-if="!hasPoints" class="empty chart-empty">
        {{
          selected.length === 0
            ? temps.length === 0
              ? 'No history yet'
              : 'Select at least one sensor.'
            : loading
              ? 'Loading history…'
              : 'No history yet'
        }}
      </p>
    </div>
    <p v-if="error" class="inline-error" role="status">History unavailable: {{ error }}</p>
  </section>
</template>
