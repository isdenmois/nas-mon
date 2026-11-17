<script setup lang="ts">
import { computed } from 'vue'
import type { Overview } from '../types'
import TempPanel from '../components/TempPanel.vue'
import HostPanel from '../components/HostPanel.vue'
import ContainerTable from '../components/ContainerTable.vue'
import VolumePanel from '../components/VolumePanel.vue'
import SmartPanel from '../components/SmartPanel.vue'

const props = defineProps<{ overview: Overview | null }>()

const smartError = computed(() => props.overview?.collectors?.smart?.last_error ?? '')
</script>

<template>
  <TempPanel :temps="overview?.temps ?? []" />
  <HostPanel :host="overview?.host" />
  <ContainerTable :containers="overview?.containers ?? []" />
  <VolumePanel :volumes="overview?.volumes ?? []" />
  <SmartPanel :smart="overview?.smart ?? []" :error="smartError" />
</template>
