<script setup lang="ts">
import { computed, ref } from 'vue'
import type { TempReading } from '@/shared/api'
import type { RangeKey } from '@/shared/config'
import { useAutoSelection, type HistorySeries } from '@/shared/lib'
import { HistoryChart, RangePicker } from '@/shared/ui'
import { temperatureLabel } from '../model/temperature'

const props = defineProps<{ temps: TempReading[] }>()
const emit = defineEmits<{ error: [message: string] }>()

const range = ref<RangeKey>('24h')
const selected = useAutoSelection(() => props.temps.map((sensor) => sensor.key))

const chartSeries = computed<HistorySeries[]>(() => {
  const byKey = new Map(props.temps.map((sensor) => [sensor.key, sensor]))
  return selected.value.map((key) => {
    const sensor = byKey.get(key)
    return { kind: 'temp', name: key, label: sensor ? temperatureLabel(sensor) : key }
  })
})
</script>

<template>
  <section class="panel" aria-labelledby="temp-history-title">
    <div class="panel-head">
      <h2 id="temp-history-title">Temperature history</h2>
      <RangePicker
        v-model:range="range"
        label="temperature history"
        group-label="Temperature history range"
      />
    </div>

    <p v-if="temps.length === 0" class="empty">No temperature sensors reported.</p>

    <template v-else>
      <div class="sensor-picker" role="group" aria-label="Temperature sensors shown in the chart">
        <label
          v-for="sensor in temps"
          :key="sensor.key"
          class="sensor-check"
          :class="{ on: selected.includes(sensor.key) }"
        >
          <input v-model="selected" type="checkbox" :value="sensor.key" />
          <span :title="sensor.key">{{ temperatureLabel(sensor) }}</span>
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
    </template>
  </section>
</template>
