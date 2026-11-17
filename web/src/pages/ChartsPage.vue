<script setup lang="ts">
import { computed } from 'vue'
import type { Overview } from '../types'
import TempHistoryPanel from '../components/TempHistoryPanel.vue'
import HostHistoryPanel from '../components/HostHistoryPanel.vue'
import ContainerHistory from '../components/ContainerHistory.vue'
import SmartHistoryPanel from '../components/SmartHistoryPanel.vue'

const props = defineProps<{ overview: Overview | null }>()
const emit = defineEmits<{ error: [message: string] }>()

const smartError = computed(() => props.overview?.collectors?.smart?.last_error ?? '')
</script>

<template>
  <TempHistoryPanel :temps="overview?.temps ?? []" @error="emit('error', $event)" />
  <HostHistoryPanel @error="emit('error', $event)" />
  <ContainerHistory :containers="overview?.containers ?? []" @error="emit('error', $event)" />
  <SmartHistoryPanel
    :smart="overview?.smart ?? []"
    :error="smartError"
    @error="emit('error', $event)"
  />
</template>
