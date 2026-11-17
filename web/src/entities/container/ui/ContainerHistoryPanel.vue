<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ContainerSnapshot } from '@/shared/api'
import type { RangeKey } from '@/shared/config'
import { useAutoSelection, type HistorySeries } from '@/shared/lib'
import { HistoryChart, RangePicker } from '@/shared/ui'
import { containerColor } from '../model/container'

const props = defineProps<{ containers: ContainerSnapshot[] }>()
const emit = defineEmits<{ error: [message: string] }>()

const range = ref<RangeKey>('24h')
const names = computed(() => props.containers.map((container) => container.name))
const selected = useAutoSelection(() => names.value)

function seriesFor(kind: string): HistorySeries[] {
  return selected.value.map((name) => ({
    kind,
    name,
    label: name,
    color: containerColor(names.value, name),
  }))
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
        <RangePicker
          v-model:range="range"
          label="container history"
          group-label="Container history range"
        />
      </div>

      <div class="sensor-picker" role="group" aria-label="Containers shown in the charts">
        <button type="button" class="range-btn" @click="selected = [...names]">All</button>
        <button type="button" class="range-btn" @click="selected = []">None</button>
        <label v-for="name in names" :key="name" class="sensor-check" :class="{ on: selected.includes(name) }">
          <input v-model="selected" type="checkbox" :value="name" />
          <span :style="{ color: containerColor(names, name) }">{{ name }}</span>
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
