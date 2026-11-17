<script setup lang="ts">
import type { VolumeSnapshot } from '@/shared/api'
import { formatBytes, formatPct } from '@/shared/lib'
import { UsageBar } from '@/shared/ui'

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
        <UsageBar :value="volume.used_pct" :label="`Usage of ${volume.name}`" />
        <div class="vol-path">{{ volume.container_path }}</div>
      </li>
    </ul>
  </section>
</template>
