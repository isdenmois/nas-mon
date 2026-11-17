<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { errorMessage, fetchWear, isAbortError } from '../api'
import { clampPct, formatBytes, formatDateTime, formatNum } from '../format'
import type { DiskSmart, Wear, WearResponse } from '../types'

defineProps<{ smart: DiskSmart[]; error?: string }>()
const emit = defineEmits<{ error: [message: string] }>()

const expanded = ref<string | null>(null)
const wear = ref<WearResponse | null>(null)
const loading = ref(false)
const loadError = ref('')

let controller: AbortController | null = null
let requestId = 0

const logs = computed(() => {
  const byDisk = new Map<string, Wear[]>()
  for (const entry of wear.value?.log ?? []) {
    const list = byDisk.get(entry.disk)
    if (list) list.push(entry)
    else byDisk.set(entry.disk, [entry])
  }
  return byDisk
})

function wearLog(disk: string): Wear[] {
  return logs.value.get(disk) ?? []
}

async function loadWear(): Promise<void> {
  if (wear.value) return
  controller?.abort()
  const ctrl = new AbortController()
  controller = ctrl
  const id = ++requestId
  loading.value = true
  loadError.value = ''
  try {
    const data = await fetchWear(200, ctrl.signal)
    if (id !== requestId) return
    wear.value = data
  } catch (err) {
    if (id !== requestId || isAbortError(err)) return
    loadError.value = errorMessage(err)
    emit('error', `Wear history: ${loadError.value}`)
  } finally {
    if (id === requestId) loading.value = false
  }
}

function toggle(disk: string): void {
  expanded.value = expanded.value === disk ? null : disk
  if (expanded.value) void loadWear()
}

onBeforeUnmount(() => controller?.abort())
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
          <template v-for="disk in smart" :key="disk.disk">
            <tr
              class="smart-row"
              :class="{ expanded: expanded === disk.disk }"
              @click="toggle(disk.disk)"
            >
              <td>
                <button
                  type="button"
                  class="row-toggle"
                  :aria-expanded="expanded === disk.disk"
                  :aria-controls="`wear-${disk.disk}`"
                  :aria-label="`Wear history for ${disk.disk}`"
                  @click.stop="toggle(disk.disk)"
                >
                  <span class="chev" aria-hidden="true">{{ expanded === disk.disk ? '▾' : '▸' }}</span>
                  {{ disk.disk }}
                </button>
              </td>
              <td class="image-cell">
                <span class="truncate" :title="`${disk.model} · S/N ${disk.serial}`">{{ disk.model || '—' }}</span>
              </td>
              <td class="num">
                <div class="wear-cell">
                  <div class="bar wear-bar" aria-hidden="true">
                    <span
                      :class="{ danger: (disk.percentage_used ?? 0) >= 80 }"
                      :style="{ width: `${clampPct(disk.percentage_used)}%` }"
                    ></span>
                  </div>
                  <span :class="{ 'warn-text': (disk.percentage_used ?? 0) >= 80 }">
                    {{ disk.percentage_used == null ? '—' : `${disk.percentage_used}%` }}
                  </span>
                  <span v-if="(disk.percentage_used ?? 0) >= 80" class="badge danger">high</span>
                </div>
              </td>
              <td class="num">{{ formatBytes(disk.tbw_bytes) }}</td>
              <td class="num">{{ disk.power_on_hours == null ? '—' : `${formatNum(disk.power_on_hours)} h` }}</td>
              <td class="num" :class="{ 'warn-text': (disk.media_errors ?? 0) > 0 }">
                {{ disk.media_errors ?? '—' }}
                <span v-if="(disk.media_errors ?? 0) > 0" class="badge danger">errors</span>
              </td>
              <td class="num" :class="{ 'warn-text': (disk.reallocated ?? 0) > 0 }">
                {{ disk.reallocated ?? '—' }}
                <span v-if="(disk.reallocated ?? 0) > 0" class="badge danger">realloc</span>
              </td>
              <td class="num">{{ disk.temp_c == null ? '—' : `${disk.temp_c} °C` }}</td>
              <td class="updated-cell">{{ formatDateTime(disk.collected_at) }}</td>
            </tr>

            <tr v-if="expanded === disk.disk" :id="`wear-${disk.disk}`" class="detail-row">
              <td colspan="9">
                <div class="wear-log">
                  <p v-if="loading" class="muted">Loading wear history…</p>
                  <p v-else-if="loadError" class="inline-error" role="status">
                    Could not load wear history: {{ loadError }}
                  </p>
                  <table v-else-if="wearLog(disk.disk).length > 0" class="inner-table">
                    <thead>
                      <tr>
                        <th scope="col">Time</th>
                        <th scope="col">Reason</th>
                        <th scope="col" class="num">Wear</th>
                        <th scope="col" class="num">TBW</th>
                        <th scope="col" class="num">Power-on</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(entry, index) in wearLog(disk.disk)" :key="`${entry.ts}-${index}`">
                        <td>{{ formatDateTime(entry.ts) }}</td>
                        <td>{{ entry.reason || '—' }}</td>
                        <td class="num">{{ entry.percentage_used == null ? '—' : `${entry.percentage_used}%` }}</td>
                        <td class="num">{{ formatBytes(entry.tbw_bytes) }}</td>
                        <td class="num">
                          {{ entry.power_on_hours == null ? '—' : `${formatNum(entry.power_on_hours)} h` }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                  <p v-else class="muted">No wear log entries for this disk yet.</p>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </section>
</template>
