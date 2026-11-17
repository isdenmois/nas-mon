<script setup lang="ts">
import { RANGES, type RangeKey } from '@/shared/config'

defineProps<{ range: RangeKey; label: string; groupLabel: string }>()
const emit = defineEmits<{ 'update:range': [range: RangeKey] }>()
</script>

<template>
  <div class="flex flex-wrap gap-1.5" role="group" :aria-label="groupLabel">
    <button
      v-for="item in RANGES"
      :key="item"
      type="button"
      class="range-btn"
      :aria-pressed="range === item"
      :aria-label="`Show ${item} of ${label}`"
      @click="emit('update:range', item)"
    >
      {{ item }}
    </button>
  </div>
</template>
