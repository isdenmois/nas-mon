<script setup lang="ts">
import { ref } from 'vue'
import { RANGES, type HistorySeries, type RangeKey } from '../types'
import HistoryChart from './HistoryChart.vue'

const emit = defineEmits<{ error: [message: string] }>()

const range = ref<RangeKey>('24h')
const cpuSeries: HistorySeries[] = [{ kind: 'host_cpu', name: 'host', label: 'CPU' }]
const ramSeries: HistorySeries[] = [{ kind: 'host_mem', name: 'host', label: 'RAM' }]
const loadSeries: HistorySeries[] = [
  { kind: 'host_load1', name: 'host', label: 'load 1m' },
  { kind: 'host_load5', name: 'host', label: 'load 5m' },
  { kind: 'host_load15', name: 'host', label: 'load 15m' },
]
const netSeries: HistorySeries[] = [
  { kind: 'host_net_rx', name: 'host', label: 'Rx' },
  { kind: 'host_net_tx', name: 'host', label: 'Tx' },
]
</script>

<template>
  <section class="panel" aria-labelledby="host-history-title">
    <div class="panel-head">
      <h2 id="host-history-title">Host history</h2>
      <div class="ranges" role="group" aria-label="Host history range">
        <button
          v-for="item in RANGES"
          :key="item"
          type="button"
          class="range-btn"
          :aria-pressed="range === item"
          :aria-label="`Show ${item} of host history`"
          @click="range = item"
        >
          {{ item }}
        </button>
      </div>
    </div>

    <div class="chart-grid">
      <article class="chart-card">
        <h3>CPU</h3>
        <HistoryChart
          :series="cpuSeries"
          :range="range"
          format="percent"
          height="180px"
          empty-text="No CPU history yet"
          @error="emit('error', $event)"
        />
      </article>
      <article class="chart-card">
        <h3>RAM</h3>
        <HistoryChart
          :series="ramSeries"
          :range="range"
          format="percent"
          height="180px"
          empty-text="No RAM history yet"
          @error="emit('error', $event)"
        />
      </article>
      <article class="chart-card">
        <h3>Load average</h3>
        <HistoryChart
          :series="loadSeries"
          :range="range"
          format="number"
          height="180px"
          empty-text="No load history yet"
          @error="emit('error', $event)"
        />
      </article>
      <article class="chart-card">
        <h3>Network</h3>
        <HistoryChart
          :series="netSeries"
          :range="range"
          format="rate"
          height="180px"
          empty-text="No network history yet"
          @error="emit('error', $event)"
        />
      </article>
    </div>
  </section>
</template>
