import type { Page, Route } from '@playwright/test'
import type { HistoryResponse, Overview, WearResponse } from '../web/src/shared/api/contracts'
import { FIXED_NOW, historyFor, overview, wear } from './fixtures'

export interface ApiOptions {
  overview?: Overview
  history?: HistoryResponse | ((series: string[], range: string) => HistoryResponse)
  wear?: WearResponse
  overviewError?: number | string
  historyError?: number | string
  wearError?: number | string
}

export interface ApiCalls {
  overview: number
  history: { range: string; series: string[] }[]
  wear: number
}

// mockApi intercepts every /api request before Vite's proxy sees it, so the
// suite never needs the Go backend.
export async function mockApi(page: Page, options: ApiOptions = {}): Promise<ApiCalls> {
  const calls: ApiCalls = { overview: 0, history: [], wear: 0 }

  await page.route(
    (url) => url.pathname.startsWith('/api/'),
    async (route) => {
      const url = new URL(route.request().url())

      if (url.pathname === '/api/overview') {
        calls.overview += 1
        await fulfill(route, options.overviewError, options.overview ?? overview)
        return
      }

      if (url.pathname === '/api/history') {
        const range = url.searchParams.get('range') ?? '24h'
        const series = url.searchParams.getAll('series')
        calls.history.push({ range, series })
        let data: HistoryResponse
        if (typeof options.history === 'function') {
          data = options.history(series, range)
        } else {
          data = options.history ?? historyFor(series, range)
        }
        await fulfill(route, options.historyError, data)
        return
      }

      if (url.pathname === '/api/wear') {
        calls.wear += 1
        await fulfill(route, options.wearError, options.wear ?? wear)
        return
      }

      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not found' }),
      })
    },
  )

  return calls
}

export async function freezeClock(page: Page): Promise<void> {
  await page.clock.setFixedTime(FIXED_NOW)
}

async function fulfill(
  route: Route,
  error: number | string | undefined,
  body: unknown,
): Promise<void> {
  if (error !== undefined) {
    const status = typeof error === 'number' ? error : 500
    const message = typeof error === 'string' ? error : 'internal error'
    await route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify({ error: message }),
    })
    return
  }
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
}
