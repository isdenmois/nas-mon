<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { formatNum } from '../format'
import { RANGES, type HistorySeries, type RangeKey, type TempReading } from '../types'
import HistoryChart from './HistoryChart.vue'

const props = defineProps<{ temps: TempReading[] }>()
const emit = defineEmits<{ error: [message: string] }>()

const range = ref<RangeKey>('24h')
const selected = ref<string[]>([])
let selectionInitialized = false

function sensorLabel(sensor: TempReading): string {
  return sensor.label ? `${sensor.chip} · ${sensor.label}` : sensor.key
}

const chartSeries = computed<HistorySeries[]>(() => {
  const byKey = new Map(props.temps.map((sensor) => [sensor.key, sensor]))
  return selected.value.map((key) => {
    const sensor = byKey.get(key)
    return { kind: 'temp', name: key, label: sensor ? sensorLabel(sensor) : key }
  })
})

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

    <HistoryChart
      :series="chartSeries"
      :range="range"
      format="temperature"
      height="300px"
      empty-text="No temperature history yet"
      @error="emit('error', $event)"
    />
  </section>
</template>
