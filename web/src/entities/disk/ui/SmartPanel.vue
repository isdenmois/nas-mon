<script setup lang="ts">
import type { DiskSmart } from '@/shared/api'
import { formatBytes, formatDateTime, formatHours, formatValue } from '@/shared/lib'
import { UsageBar } from '@/shared/ui'

defineProps<{ smart: DiskSmart[]; error?: string }>()
</script>

<template>
  <section class="panel" aria-labelledby="smart-title">
    <div class="panel-head">
      <h2 id="smart-title">SMART / SSD</h2>
      <span class="muted">{{ smart.length }} {{ smart.length === 1 ? 'disk' : 'disks' }}</span>
    </div>

    <p v-if="smart.length === 0" class="empty" :class="{ 'inline-error': error }">
      {{ error ? `SMART collector error: ${error}` : 'No SMART data' }}
    </p>

    <div v-else class="table-wrap">
      <table class="smart-table">
        <thead>
          <tr>
            <th scope="col">Disk</th>
            <th scope="col">Model</th>
            <th scope="col" class="num">Wear</th>
            <th scope="col" class="num">TBW</th>
            <th scope="col" class="num">Power-on</th>
            <th scope="col" class="num">Media errors</th>
            <th scope="col" class="num">Reallocated</th>
            <th scope="col" class="num">Temp</th>
            <th scope="col">Last update</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="disk in smart" :key="disk.disk">
            <td>{{ disk.disk }}</td>
            <td class="image-cell">
              <span class="truncate" :title="`${disk.model} · S/N ${disk.serial}`">{{ disk.model || '—' }}</span>
            </td>
            <td class="num">
              <div class="wear-cell">
                <UsageBar
                  class="wear-bar"
                  hidden
                  :value="disk.percentage_used"
                  :danger-at="80"
                />
                <span :class="{ 'warn-text': (disk.percentage_used ?? 0) >= 80 }">
                  {{ formatValue(disk.percentage_used, 'percent') }}
                </span>
                <span v-if="(disk.percentage_used ?? 0) >= 80" class="badge danger">high</span>
              </div>
            </td>
            <td class="num">{{ formatBytes(disk.tbw_bytes) }}</td>
            <td class="num">{{ formatHours(disk.power_on_hours) }}</td>
            <td class="num" :class="{ 'warn-text': (disk.media_errors ?? 0) > 0 }">
              {{ disk.media_errors ?? '—' }}
              <span v-if="(disk.media_errors ?? 0) > 0" class="badge danger">errors</span>
            </td>
            <td class="num" :class="{ 'warn-text': (disk.reallocated ?? 0) > 0 }">
              {{ disk.reallocated ?? '—' }}
              <span v-if="(disk.reallocated ?? 0) > 0" class="badge danger">realloc</span>
            </td>
            <td class="num">{{ formatValue(disk.temp_c, 'temperature') }}</td>
            <td class="updated-cell">{{ formatDateTime(disk.collected_at) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
