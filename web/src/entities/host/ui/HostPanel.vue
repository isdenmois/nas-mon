<script setup lang="ts">
import { computed } from 'vue'
import type { HostSnapshot } from '@/shared/api'
import { formatBytes, formatDateTime, formatNum, formatPct, formatRate } from '@/shared/lib'
import { UsageBar } from '@/shared/ui'

const props = defineProps<{ host?: HostSnapshot | null }>()

const updated = computed(() => (props.host ? formatDateTime(props.host.updated_at) : ''))
</script>

<template>
  <section class="panel" aria-labelledby="host-title">
    <div class="panel-head">
      <h2 id="host-title">Host</h2>
      <span v-if="updated" class="muted">updated {{ updated }}</span>
    </div>

    <p v-if="!host" class="empty">Host metrics unavailable</p>

    <div v-else class="host-grid">
      <article class="stat">
        <span class="stat-label">CPU</span>
        <strong class="stat-value">{{ formatPct(host.cpu_pct) }}</strong>
        <UsageBar :value="host.cpu_pct" label="CPU usage" />
      </article>

      <article class="stat">
        <span class="stat-label">RAM</span>
        <strong class="stat-value">{{ formatPct(host.mem_pct) }}</strong>
        <span class="stat-sub">{{ formatBytes(host.mem_used) }} / {{ formatBytes(host.mem_total) }}</span>
        <UsageBar :value="host.mem_pct" label="Memory usage" />
      </article>

      <article class="stat">
        <span class="stat-label">Load average</span>
        <div class="loads">
          <div><strong>{{ formatNum(host.load1) }}</strong><span>1m</span></div>
          <div><strong>{{ formatNum(host.load5) }}</strong><span>5m</span></div>
          <div><strong>{{ formatNum(host.load15) }}</strong><span>15m</span></div>
        </div>
      </article>

      <article class="stat">
        <span class="stat-label">Network</span>
        <div class="net"><span>Rx</span><strong>{{ formatRate(host.net_rx_bps) }}</strong></div>
        <div class="net"><span>Tx</span><strong>{{ formatRate(host.net_tx_bps) }}</strong></div>
      </article>
    </div>
  </section>
</template>
