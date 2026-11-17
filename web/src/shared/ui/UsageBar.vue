<script setup lang="ts">
import { computed } from 'vue'
import { clampPct } from '@/shared/lib'

const props = withDefaults(
  defineProps<{
    value?: number | null
    label?: string
    hidden?: boolean
    dangerAt?: number
  }>(),
  {
    value: null,
    label: '',
    hidden: false,
    dangerAt: 90,
  },
)

const pct = computed(() => clampPct(props.value))
</script>

<template>
  <div
    class="bar"
    :role="hidden ? undefined : 'progressbar'"
    :aria-label="hidden ? undefined : label"
    :aria-hidden="hidden ? 'true' : undefined"
    :aria-valuemin="hidden ? undefined : 0"
    :aria-valuemax="hidden ? undefined : 100"
    :aria-valuenow="hidden ? undefined : Math.round(pct)"
  >
    <span :class="{ danger: pct >= dangerAt }" :style="{ width: `${pct}%` }"></span>
  </div>
</template>
