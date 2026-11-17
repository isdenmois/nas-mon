<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { errorMessage, fetchHistory, isAbortError } from '@/shared/api'
import type { RangeKey } from '@/shared/config'
import type { ChartSeries, HistorySeries, ValueFormat } from '@/shared/lib'
import SeriesChart from './SeriesChart.vue'

const props = withDefaults(
  defineProps<{
    series: HistorySeries[]
    range: RangeKey
    format: ValueFormat
    height?: string
    emptyText?: string
    showPoints?: boolean
    refreshMs?: number
  }>(),
  {
    height: '220px',
    emptyText: 'No history yet',
    showPoints: false,
    refreshMs: 60_000,
  },
)

const emit = defineEmits<{ error: [message: string] }>()

const datasets = ref<ChartSeries[]>([])
const loading = ref(false)
const error = ref('')

let controller: AbortController | null = null
let requestId = 0
let timer: ReturnType<typeof setInterval> | undefined

const seriesKey = computed(() => props.series.map((item) => `${item.kind}:${item.name}`).join('|'))

async function load(): Promise<void> {
  if (props.series.length === 0) {
    controller?.abort()
    requestId += 1
    datasets.value = []
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
      props.range,
      props.series.map((item) => `${item.kind}:${item.name}`),
      ctrl.signal,
    )
    if (id !== requestId) return
    const byKey = new Map(data.series.map((item) => [`${item.kind}:${item.name}`, item]))
    datasets.value = props.series
      .map((spec) => {
        const found = byKey.get(`${spec.kind}:${spec.name}`)
        return {
          label: spec.label,
          color: spec.color,
          points: (found?.points ?? []).map((point) => ({ ts: point.ts, value: point.avg })),
        }
      })
      .filter((item) => item.points.length > 0)
    error.value = ''
  } catch (err) {
    if (id !== requestId || isAbortError(err)) return
    error.value = errorMessage(err)
    emit('error', `History: ${error.value}`)
  } finally {
    if (id === requestId) loading.value = false
  }
}

watch([() => props.range, seriesKey], () => {
  void load()
})

onMounted(() => {
  void load()
  timer = setInterval(() => {
    void load()
  }, props.refreshMs)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  controller?.abort()
})
</script>

<template>
  <div>
    <SeriesChart
      :datasets="datasets"
      :format="format"
      :range="range"
      :height="height"
      :show-points="showPoints"
      :empty-text="loading ? 'Loading history…' : emptyText"
    />
    <p v-if="error" class="inline-error" role="status">History unavailable: {{ error }}</p>
  </div>
</template>
