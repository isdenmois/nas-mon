<script setup lang="ts">
import { computed } from 'vue'
import type { Overview } from '@/shared/api'
import { ContainerTable } from '@/entities/container'
import { SmartPanel } from '@/entities/disk'
import { HostPanel } from '@/entities/host'
import { TemperaturePanel } from '@/entities/temperature'
import { VolumePanel } from '@/entities/volume'

const props = defineProps<{ overview: Overview | null }>()

const smartError = computed(() => props.overview?.collectors?.smart?.last_error ?? '')
</script>

<template>
  <TemperaturePanel :temps="overview?.temps ?? []" />
  <HostPanel :host="overview?.host" />
  <ContainerTable :containers="overview?.containers ?? []" />
  <VolumePanel :volumes="overview?.volumes ?? []" />
  <SmartPanel :smart="overview?.smart ?? []" :error="smartError" />
</template>
