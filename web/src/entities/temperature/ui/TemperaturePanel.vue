<script setup lang="ts">
import type { TempReading } from '@/shared/api'
import { formatNum } from '@/shared/lib'

defineProps<{ temps: TempReading[] }>()
</script>

<template>
  <section class="panel" aria-labelledby="temps-title">
    <div class="panel-head">
      <h2 id="temps-title">Temperatures</h2>
      <span class="muted">{{ temps.length }} {{ temps.length === 1 ? 'sensor' : 'sensors' }}</span>
    </div>

    <div v-if="temps.length > 0" class="temp-grid">
      <article v-for="sensor in temps" :key="sensor.key" class="temp-tile" :class="{ warn: sensor.warn }">
        <div class="temp-label" :title="sensor.key">{{ sensor.label || sensor.key }}</div>
        <div class="temp-value">{{ formatNum(sensor.celsius) }}<span class="unit">°C</span></div>
        <div class="temp-meta">
          <span :title="sensor.key">{{ sensor.chip }}</span>
          <span v-if="sensor.warn" class="warn-text">Warning</span>
        </div>
      </article>
    </div>
    <p v-else class="empty">No temperature sensors reported.</p>
  </section>
</template>
