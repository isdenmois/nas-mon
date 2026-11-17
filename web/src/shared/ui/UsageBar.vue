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
    class="mt-2 h-1.5 overflow-hidden rounded-[999px] bg-[#0b1324]"
    :role="hidden ? undefined : 'progressbar'"
    :aria-label="hidden ? undefined : label"
    :aria-hidden="hidden ? 'true' : undefined"
    :aria-valuemin="hidden ? undefined : 0"
    :aria-valuemax="hidden ? undefined : 100"
    :aria-valuenow="hidden ? undefined : Math.round(pct)"
  >
    <span
      class="block h-full rounded-[inherit] [transition:width_0.25s_ease]"
      :class="pct >= dangerAt ? 'bg-danger' : 'bg-accent'"
      :style="{ width: `${pct}%` }"
    ></span>
  </div>
</template>
