<script setup lang="ts">
import type { VolumeSnapshot } from '../types'
import { clampPct, formatBytes, formatPct } from '../format'

defineProps<{ volumes: VolumeSnapshot[] }>()
</script>

<template>
  <section class="panel" aria-labelledby="volumes-title">
    <div class="panel-head">
      <h2 id="volumes-title">Filesystems</h2>
      <span class="muted">{{ volumes.length }} mounted</span>
    </div>

    <p v-if="volumes.length === 0" class="empty">No filesystems mounted under /volumes</p>

    <ul v-else class="vol-list">
      <li v-for="volume in volumes" :key="volume.name || volume.container_path">
        <div class="vol-head">
          <span class="vol-name">{{ volume.name }}</span>
          <span class="muted">{{ formatBytes(volume.used) }} / {{ formatBytes(volume.total) }} · {{ formatPct(volume.used_pct) }}</span>
        </div>
        <div
          class="bar"
          role="progressbar"
          :aria-label="`Usage of ${volume.name}`"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="Math.round(clampPct(volume.used_pct))"
        >
          <span
            :class="{ danger: clampPct(volume.used_pct) >= 90 }"
            :style="{ width: `${clampPct(volume.used_pct)}%` }"
          ></span>
        </div>
        <div class="vol-path">{{ volume.container_path }}</div>
      </li>
    </ul>
  </section>
</template>
