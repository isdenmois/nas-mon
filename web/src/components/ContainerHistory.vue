<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RANGES, type ContainerSnapshot, type HistorySeries, type RangeKey } from '../types'
import HistoryChart from './HistoryChart.vue'

const props = defineProps<{ containers: ContainerSnapshot[] }>()
const emit = defineEmits<{ error: [message: string] }>()

const range = ref<RangeKey>('24h')
const selected = ref<string[]>([])
let selectionInitialized = false

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
const names = computed(() => props.containers.map((container) => container.name))

watch(
  () => names.value.join('\u0000'),
  () => {
    if (!selectionInitialized) {
      if (names.value.length === 0) return
      selectionInitialized = true
      selected.value = [...names.value]
      return
    }
    const available = new Set(names.value)
    const kept = selected.value.filter((name) => available.has(name))
    if (kept.length !== selected.value.length) selected.value = kept
  },
  { immediate: true },
)

function colorFor(name: string): string {
  const index = names.value.indexOf(name)
  return palette[Math.max(index, 0) % palette.length]
}

function seriesFor(kind: string): HistorySeries[] {
  return selected.value.map((name) => ({ kind, name, label: name, color: colorFor(name) }))
}

const cpuSeries = computed(() => seriesFor('container_cpu'))
const rxSeries = computed(() => seriesFor('container_net_rx'))
const txSeries = computed(() => seriesFor('container_net_tx'))
const emptyText = computed(() =>
  selected.value.length === 0 ? 'Select at least one container.' : 'No container history yet',
)
</script>

<template>
  <section class="panel" aria-labelledby="container-history-title">
    <div class="panel-head">
      <h2 id="container-history-title">Container history</h2>
      <span class="muted">{{ selected.length }} of {{ containers.length }} selected</span>
    </div>

    <p v-if="containers.length === 0" class="empty">No running containers to chart.</p>

    <template v-else>
      <div class="panel-head chart-head">
        <h3>Range</h3>
        <div class="ranges" role="group" aria-label="Container history range">
          <button
            v-for="item in RANGES"
            :key="item"
            type="button"
            class="range-btn"
            :aria-pressed="range === item"
            :aria-label="`Show ${item} of container history`"
            @click="range = item"
          >
            {{ item }}
          </button>
        </div>
      </div>

      <div class="sensor-picker" role="group" aria-label="Containers shown in the charts">
        <button type="button" class="range-btn" @click="selected = [...names]">All</button>
        <button type="button" class="range-btn" @click="selected = []">None</button>
        <label v-for="name in names" :key="name" class="sensor-check" :class="{ on: selected.includes(name) }">
          <input v-model="selected" type="checkbox" :value="name" />
          <span :style="{ color: colorFor(name) }">{{ name }}</span>
        </label>
      </div>

      <div class="chart-grid">
        <article class="chart-card">
          <h3>CPU</h3>
          <HistoryChart
            :series="cpuSeries"
            :range="range"
            format="percent"
            height="180px"
            :empty-text="emptyText"
            @error="emit('error', $event)"
          />
        </article>
        <article class="chart-card">
          <h3>Network receive</h3>
          <HistoryChart
            :series="rxSeries"
            :range="range"
            format="rate"
            height="180px"
            :empty-text="emptyText"
            @error="emit('error', $event)"
          />
        </article>
        <article class="chart-card">
          <h3>Network transmit</h3>
          <HistoryChart
            :series="txSeries"
            :range="range"
            format="rate"
            height="180px"
            :empty-text="emptyText"
            @error="emit('error', $event)"
          />
        </article>
      </div>
    </template>
  </section>
</template>
