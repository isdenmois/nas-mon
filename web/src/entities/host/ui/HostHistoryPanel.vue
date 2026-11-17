<script setup lang="ts">
import { ref } from 'vue'
import type { RangeKey } from '@/shared/config'
import { HistoryChart, RangePicker } from '@/shared/ui'
import { cpuSeries, loadSeries, netSeries, ramSeries } from '../model/host'

const emit = defineEmits<{ error: [message: string] }>()

const range = ref<RangeKey>('24h')
</script>

<template>
  <section class="panel" aria-labelledby="host-history-title">
    <div class="panel-head">
      <h2 id="host-history-title">Host history</h2>
      <RangePicker v-model:range="range" label="host history" group-label="Host history range" />
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
