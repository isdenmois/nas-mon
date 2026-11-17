<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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
import { formatAxis, formatValue, type ValueFormat } from '../format'
import type { ChartSeries, RangeKey } from '../types'

Chart.register(
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  Tooltip,
  Legend,
)

const props = withDefaults(
  defineProps<{
    datasets: ChartSeries[]
    format: ValueFormat
    range?: RangeKey
    height?: string
    emptyText?: string
    showPoints?: boolean
  }>(),
  {
    range: '24h',
    height: '220px',
    emptyText: 'No history yet',
    showPoints: false,
  },
)

const palette = [
  '#60a5fa',
  '#f472b6',
  '#34d399',
  '#fbbf24',
  '#a78bfa',
  '#f87171',
  '#22d3ee',
  '#facc15',
]

const canvas = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

const hasData = computed(() => props.datasets.some((series) => series.points.length > 0))
const ariaLabel = computed(() =>
  hasData.value
    ? `History chart for ${props.datasets
        .filter((series) => series.points.length > 0)
        .map((series) => series.label)
        .join(', ')}`
    : props.emptyText,
)

function draw(): void {
  if (!canvas.value || !hasData.value) {
    chart?.destroy()
    chart = null
    return
  }

  const stamps = new Set<number>()
  for (const series of props.datasets) {
    for (const point of series.points) stamps.add(point.ts)
  }
  const xs = [...stamps].sort((a, b) => a - b)
  const data: ChartData<'line'> = {
    labels: xs.map((ts) => formatAxis(ts, props.range)),
    datasets: props.datasets
      .filter((series) => series.points.length > 0)
      .map((series, index) => {
        const byTs = new Map(series.points.map((point) => [point.ts, point.value]))
        const color = series.color ?? palette[index % palette.length]
        return {
          label: series.label,
          data: xs.map((ts) => byTs.get(ts) ?? null),
          borderColor: color,
          backgroundColor: `${color}33`,
          borderWidth: 2,
          borderDash: series.dashed ? [5, 4] : undefined,
          pointRadius: props.showPoints ? 3 : 0,
          pointBackgroundColor: color,
          tension: 0.2,
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
            label: (ctx) =>
              `${ctx.dataset.label ?? ''}: ${formatValue(Number(ctx.parsed.y), props.format)}`,
          },
        },
      },
      scales: {
        x: {
          ticks: { color: '#94a3b8', maxTicksLimit: 8, maxRotation: 0, autoSkip: true },
          grid: { color: 'rgba(148, 163, 184, 0.10)' },
        },
        y: {
          beginAtZero: props.format === 'percent' || props.format === 'rate',
          ticks: {
            color: '#94a3b8',
            callback: (value) => formatValue(Number(value), props.format),
          },
          grid: { color: 'rgba(148, 163, 184, 0.10)' },
        },
      },
    },
  })
}

watch(() => [props.datasets, props.range, props.format, props.showPoints], draw, { deep: true })
onMounted(draw)
onBeforeUnmount(() => {
  chart?.destroy()
  chart = null
})
</script>

<template>
  <div class="chart-box" :style="{ height }">
    <canvas v-show="hasData" ref="canvas" role="img" :aria-label="ariaLabel"></canvas>
    <p v-if="!hasData" class="empty chart-empty">{{ emptyText }}</p>
  </div>
</template>
