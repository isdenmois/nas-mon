<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { errorMessage, fetchOverview, isAbortError, type Overview } from '@/shared/api'
import { formatDateTime, pollWhileVisible } from '@/shared/lib'
import { ChartsPage } from '@/pages/charts'
import { OverviewPage } from '@/pages/overview'
import { pageFromPath, pageTitle, type Page } from './routing'
import StatusChips from './ui/StatusChips.vue'

const overview = ref<Overview | null>(null)
const banner = ref('')
const overviewBanner = 'Could not refresh overview'

const page = ref<Page>(pageFromPath(window.location.pathname))

let controller: AbortController | null = null
let stopPolling: (() => void) | undefined

const lastUpdate = computed(() =>
  overview.value ? formatDateTime(overview.value.ts) : 'Waiting for data…',
)

function applyTitle(): void {
  document.title = pageTitle(page.value)
}

function navigate(event: MouseEvent, path: string): void {
  // Let modifier clicks and non-primary buttons open a new tab normally.
  if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  event.preventDefault()
  if (window.location.pathname !== path) {
    history.pushState(null, '', path)
    page.value = pageFromPath(path)
    window.scrollTo({ top: 0 })
  }
  applyTitle()
}

function onPopState(): void {
  page.value = pageFromPath(window.location.pathname)
  applyTitle()
}

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
  applyTitle()
  window.addEventListener('popstate', onPopState)
  void loadOverview()
  stopPolling = pollWhileVisible(() => {
    void loadOverview()
  }, 10_000)
})

onBeforeUnmount(() => {
  stopPolling?.()
  window.removeEventListener('popstate', onPopState)
  controller?.abort()
})
</script>

<template>
  <div class="app">
    <header class="top">
      <div class="brand">
        <div>
          <h1>mon</h1>
          <p class="updated">Last update: <span>{{ lastUpdate }}</span></p>
        </div>
        <nav class="nav" aria-label="Primary">
          <a
            href="/"
            :class="{ active: page === 'overview' }"
            :aria-current="page === 'overview' ? 'page' : undefined"
            @click="navigate($event, '/')"
          >
            Overview
          </a>
          <a
            href="/charts"
            :class="{ active: page === 'charts' }"
            :aria-current="page === 'charts' ? 'page' : undefined"
            @click="navigate($event, '/charts')"
          >
            Charts
          </a>
        </nav>
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
      <OverviewPage v-if="page === 'overview'" :overview="overview" />
      <ChartsPage v-else :overview="overview" @error="notify" />
    </main>

    <footer class="foot">mon · homelab monitor</footer>
  </div>
</template>
