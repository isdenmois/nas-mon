<script setup lang="ts">
import { computed, ref } from 'vue'
import type { CollectorStatus } from '../types'
import { formatDateTime } from '../format'

const props = defineProps<{ collectors?: Record<string, CollectorStatus> | null }>()

type State = 'ok' | 'error' | 'idle'

const open = ref<string | null>(null)

const entries = computed(() =>
  Object.entries(props.collectors ?? {}).sort(([a], [b]) => a.localeCompare(b)),
)

function statusOf(status: CollectorStatus): State {
  if (status.last_error) return 'error'
  return status.runs > 0 ? 'ok' : 'idle'
}

function title(name: string, status: CollectorStatus): string {
  const head = status.last_error ? `${name}: ${status.last_error}` : `${name}: ${statusOf(status)}`
  return status.last_success ? `${head}\nLast success: ${formatDateTime(status.last_success)}` : head
}

function toggle(name: string): void {
  open.value = open.value === name ? null : name
}

function errorFor(name: string | null): string {
  return name ? (props.collectors?.[name]?.last_error ?? '') : ''
}
</script>

<template>
  <div class="collectors">
    <div class="chips" role="group" aria-label="Collector status">
      <button
        v-for="[name, status] in entries"
        :key="name"
        type="button"
        class="chip"
        :class="statusOf(status)"
        :title="title(name, status)"
        :aria-expanded="open === name"
        @click="toggle(name)"
      >
        <span class="dot" aria-hidden="true"></span>
        <span>{{ name }}: {{ statusOf(status) }}</span>
      </button>
      <span v-if="entries.length === 0" class="muted">No collector status yet.</span>
    </div>
    <p v-if="open && errorFor(open)" class="chip-error" role="status">
      <strong>{{ open }}:</strong> {{ errorFor(open) }}
    </p>
  </div>
</template>
