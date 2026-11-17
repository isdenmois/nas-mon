<script setup lang="ts">
import { computed, ref } from 'vue'
import type { HostSnapshot, HistorySeries, RangeKey } from '../types'
import { RANGES } from '../types'
import { clampPct, formatBytes, formatDateTime, formatNum, formatPct, formatRate } from '../format'
import HistoryChart from './HistoryChart.vue'

const props = defineProps<{ host?: HostSnapshot | null }>()
const emit = defineEmits<{ error: [message: string] }>()

const range = ref<RangeKey>('24h')
const updated = computed(() => (props.host ? formatDateTime(props.host.updated_at) : ''))

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
  <section class="panel" aria-labelledby="host-title">
    <div class="panel-head">
      <h2 id="host-title">Host</h2>
      <span v-if="updated" class="muted">updated {{ updated }}</span>
    </div>

    <p v-if="!host" class="empty">Host metrics unavailable</p>

    <div v-else class="host-grid">
      <article class="stat">
        <span class="stat-label">CPU</span>
        <strong class="stat-value">{{ formatPct(host.cpu_pct) }}</strong>
        <div
          class="bar"
          role="progressbar"
          aria-label="CPU usage"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="Math.round(clampPct(host.cpu_pct))"
        >
          <span :class="{ danger: clampPct(host.cpu_pct) >= 90 }" :style="{ width: `${clampPct(host.cpu_pct)}%` }"></span>
        </div>
      </article>

      <article class="stat">
        <span class="stat-label">RAM</span>
        <strong class="stat-value">{{ formatPct(host.mem_pct) }}</strong>
        <span class="stat-sub">{{ formatBytes(host.mem_used) }} / {{ formatBytes(host.mem_total) }}</span>
        <div
          class="bar"
          role="progressbar"
          aria-label="Memory usage"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="Math.round(clampPct(host.mem_pct))"
        >
          <span :class="{ danger: clampPct(host.mem_pct) >= 90 }" :style="{ width: `${clampPct(host.mem_pct)}%` }"></span>
        </div>
      </article>

      <article class="stat">
        <span class="stat-label">Load average</span>
        <div class="loads">
          <div><strong>{{ formatNum(host.load1) }}</strong><span>1m</span></div>
          <div><strong>{{ formatNum(host.load5) }}</strong><span>5m</span></div>
          <div><strong>{{ formatNum(host.load15) }}</strong><span>15m</span></div>
        </div>
      </article>

      <article class="stat">
        <span class="stat-label">Network</span>
        <div class="net"><span>Rx</span><strong>{{ formatRate(host.net_rx_bps) }}</strong></div>
        <div class="net"><span>Tx</span><strong>{{ formatRate(host.net_tx_bps) }}</strong></div>
      </article>
    </div>

    <div class="panel-head chart-head">
      <h3>Host history</h3>
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
