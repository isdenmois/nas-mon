<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { errorMessage, fetchOverview, isAbortError } from './api'
import { formatDateTime } from './format'
import type { Overview } from './types'
import StatusChips from './components/StatusChips.vue'
import TempPanel from './components/TempPanel.vue'
import HostPanel from './components/HostPanel.vue'
import ContainerTable from './components/ContainerTable.vue'
import ContainerHistory from './components/ContainerHistory.vue'
import VolumePanel from './components/VolumePanel.vue'
import SmartPanel from './components/SmartPanel.vue'

const overview = ref<Overview | null>(null)
const banner = ref('')
const overviewBanner = 'Could not refresh overview'

let controller: AbortController | null = null
let timer: ReturnType<typeof setInterval> | undefined

const lastUpdate = computed(() =>
  overview.value ? formatDateTime(overview.value.ts) : 'Waiting for data…',
)
const smartError = computed(() => overview.value?.collectors?.smart?.last_error ?? '')

function notify(message: string): void {
  banner.value = message
}

async function loadOverview(): Promise<void> {
  controller?.abort()
  const ctrl = new AbortController()
  controller = ctrl
  try {
    const data = await fetchOverview(ctrl.signal)
    if (controller !== ctrl) return
    overview.value = data
    if (banner.value.startsWith(overviewBanner)) banner.value = ''
  } catch (err) {
    if (isAbortError(err) || controller !== ctrl) return
    const suffix = overview.value ? 'Showing last known data.' : 'No data loaded yet.'
    banner.value = `${overviewBanner}: ${errorMessage(err)}. ${suffix}`
  }
}

onMounted(() => {
  void loadOverview()
  timer = setInterval(() => {
    void loadOverview()
  }, 10_000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  controller?.abort()
})
</script>

<template>
  <div class="app">
    <header class="top">
      <div class="brand">
        <h1>mon</h1>
        <p class="updated">Last update: <span>{{ lastUpdate }}</span></p>
      </div>
      <StatusChips :collectors="overview?.collectors" />
    </header>

    <div v-if="banner" class="banner" role="alert">
      <span>{{ banner }}</span>
      <button type="button" class="banner-close" aria-label="Dismiss notification" @click="banner = ''">
        Dismiss
      </button>
    </div>

    <main>
      <TempPanel :temps="overview?.temps ?? []" @error="notify" />
      <HostPanel :host="overview?.host" />
      <ContainerTable :containers="overview?.containers ?? []" />
      <ContainerHistory :containers="overview?.containers ?? []" />
      <VolumePanel :volumes="overview?.volumes ?? []" />
      <SmartPanel :smart="overview?.smart ?? []" :error="smartError" @error="notify" />
    </main>

    <footer class="foot">mon · homelab monitor</footer>
  </div>
</template>
