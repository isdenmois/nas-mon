<script setup lang="ts">
import { computed } from 'vue'
import type { ContainerSnapshot } from '@/shared/api'
import { formatBytes, formatPct, formatRate } from '@/shared/lib'
import { sortByCpu } from '../model/container'

const props = defineProps<{ containers: ContainerSnapshot[] }>()

const rows = computed(() => sortByCpu(props.containers))
</script>

<template>
  <section class="panel" aria-labelledby="containers-title">
    <div class="panel-head">
      <h2 id="containers-title">Containers</h2>
      <span class="muted">{{ containers.length }} running</span>
    </div>

    <p v-if="containers.length === 0" class="empty">No running containers</p>

    <div v-else class="table-wrap">
      <table>
        <thead>
          <tr>
            <th scope="col">Name</th>
            <th scope="col">Image</th>
            <th scope="col" class="num">CPU</th>
            <th scope="col" class="num">Memory</th>
            <th scope="col" class="num">Rx</th>
            <th scope="col" class="num">Tx</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="container in rows" :key="container.id || container.name">
            <td class="name">{{ container.name }}</td>
            <td class="image-cell">
              <span class="truncate" :title="container.image">{{ container.image }}</span>
            </td>
            <td class="num">{{ formatPct(container.cpu_pct) }}</td>
            <td class="num">
              {{ formatBytes(container.mem_used) }} / {{ formatBytes(container.mem_limit) }}
              <span class="muted">({{ formatPct(container.mem_pct) }})</span>
            </td>
            <td class="num">{{ formatRate(container.net_rx_bps) }}</td>
            <td class="num">{{ formatRate(container.net_tx_bps) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
