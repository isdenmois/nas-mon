<script setup lang="ts">
import { computed } from 'vue'
import type { Overview } from '@/shared/api'
import { ContainerHistoryPanel } from '@/entities/container'
import { SmartHistoryPanel } from '@/entities/disk'
import { HostHistoryPanel } from '@/entities/host'
import { seriesIndex } from '@/entities/series'
import { TemperatureHistoryPanel } from '@/entities/temperature'

const props = defineProps<{ overview: Overview | null }>()
const emit = defineEmits<{ error: [message: string] }>()

const index = computed(() => seriesIndex(props.overview?.series))
const smartError = computed(() => props.overview?.collectors?.smart?.last_error ?? '')
</script>

<template>
  <TemperatureHistoryPanel
    :temps="overview?.temps ?? []"
    :series-index="index"
    @error="emit('error', $event)"
  />
  <HostHistoryPanel :series-index="index" @error="emit('error', $event)" />
  <ContainerHistoryPanel
    :containers="overview?.containers ?? []"
    :series-index="index"
    @error="emit('error', $event)"
  />
  <SmartHistoryPanel
    :smart="overview?.smart ?? []"
    :error="smartError"
    @error="emit('error', $event)"
  />
</template>
