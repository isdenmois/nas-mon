import { expect, test, type Locator, type Page } from '@playwright/test'
import { freezeClock, mockApi } from './mock-api'

const tempPanel = (page: Page): Locator =>
  page.locator('section[aria-labelledby="temp-history-title"]')
const hostPanel = (page: Page): Locator =>
  page.locator('section[aria-labelledby="host-history-title"]')
const containerPanel = (page: Page): Locator =>
  page.locator('section[aria-labelledby="container-history-title"]')
const smartPanel = (page: Page): Locator =>
  page.locator('section[aria-labelledby="smart-history-title"]')

async function lastTempSeries(calls: Awaited<ReturnType<typeof mockApi>>): Promise<string[]> {
  return (
    calls.history.filter((call) => call.series.some((item) => item.startsWith('temp:'))).at(-1)
      ?.series ?? []
  )
}

test.beforeEach(async ({ page }) => {
  await freezeClock(page)
})

test('deep link renders every history panel from the API', async ({ page }) => {
  const calls = await mockApi(page)
  await page.goto('/charts')

  await expect(page).toHaveTitle('mon · charts')
  await expect(page.getByRole('heading', { name: 'Temperature history' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Host history' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Container history' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'SMART history' })).toBeVisible()

  const tempChart = tempPanel(page).locator('canvas')
  await expect(tempChart).toBeVisible()
  await expect(tempChart).toHaveAttribute(
    'aria-label',
    'History chart for k10temp · Tctl, nvme · Composite, nvme · Composite',
  )
  await expect(hostPanel(page).locator('canvas')).toHaveCount(4)
  await expect(containerPanel(page).locator('canvas')).toHaveCount(3)

  expect(calls.wear).toBeGreaterThan(0)
  expect(calls.history.some((call) => call.series.includes('container_cpu:jellyfin'))).toBe(true)
})

test('switching the range refetches the chart', async ({ page }) => {
  const calls = await mockApi(page)
  await page.goto('/charts')

  await expect
    .poll(() => calls.history.some((call) => call.range === '24h' && call.series.length > 0))
    .toBe(true)

  const button = page.getByRole('button', { name: 'Show 7d of temperature history' })
  await button.click()

  await expect(button).toHaveAttribute('aria-pressed', 'true')
  await expect
    .poll(() => calls.history.some((call) => call.range === '7d' && call.series.length > 0))
    .toBe(true)
  await expect(tempPanel(page).locator('canvas')).toBeVisible()
})

test('temperature checkboxes choose which series are charted', async ({ page }) => {
  const calls = await mockApi(page)
  await page.goto('/charts')

  const sensors = tempPanel(page).getByRole('checkbox')
  await expect(sensors).toHaveCount(3)
  await expect.poll(async () => (await lastTempSeries(calls)).length).toBe(3)

  await sensors.nth(0).uncheck()
  await sensors.nth(1).uncheck()
  await sensors.nth(2).uncheck()

  await expect(tempPanel(page).getByText('No temperature history yet')).toBeVisible()
  await expect(tempPanel(page).locator('canvas')).toBeHidden()

  await sensors.nth(0).check()
  await expect.poll(async () => lastTempSeries(calls)).toEqual(['temp:k10temp:Tctl'])
  await expect(tempPanel(page).locator('canvas')).toHaveAttribute(
    'aria-label',
    'History chart for k10temp · Tctl',
  )
})

test('container picker limits the charted series', async ({ page }) => {
  const calls = await mockApi(page)
  await page.goto('/charts')

  await containerPanel(page).getByRole('button', { name: 'None', exact: true }).click()
  await expect(containerPanel(page).getByText('Select at least one container.')).toHaveCount(3)
  await expect(containerPanel(page).locator('canvas')).toHaveCount(3)
  await expect(containerPanel(page).locator('canvas').first()).toBeHidden()

  await containerPanel(page).getByRole('checkbox', { name: 'jellyfin' }).check()
  await expect
    .poll(() =>
      calls.history.some(
        (call) => call.series.length === 1 && call.series[0] === 'container_cpu:jellyfin',
      ),
    )
    .toBe(true)
  await expect(containerPanel(page).locator('canvas')).toHaveCount(3)
  await expect(containerPanel(page).locator('canvas').first()).toBeVisible()
})

test('reports history failures in the banner', async ({ page }) => {
  await mockApi(page, { historyError: 'database is locked' })
  await page.goto('/charts')

  const banner = page.getByRole('alert')
  await expect(banner).toContainText('History: database is locked')
  await expect(tempPanel(page).getByText('History unavailable: database is locked')).toBeVisible()

  await banner.getByRole('button', { name: 'Dismiss notification' }).click()
  await expect(page.getByRole('alert')).toHaveCount(0)
})

test.describe('chart snapshots', () => {
  test('temperature and host panels match the baseline', async ({ page }) => {
    await mockApi(page)
    await page.goto('/charts')
    await expect(tempPanel(page).locator('canvas')).toBeVisible()
    await expect(hostPanel(page).locator('canvas')).toHaveCount(4)

    await expect(tempPanel(page)).toHaveScreenshot('temperature-history.png')
    await expect(hostPanel(page)).toHaveScreenshot('host-history.png')
  })

  test('container and smart panels match the baseline', async ({ page }) => {
    await mockApi(page)
    await page.goto('/charts')
    await expect(containerPanel(page).locator('canvas')).toHaveCount(3)
    await expect(smartPanel(page).locator('table')).toBeVisible()

    await expect(containerPanel(page)).toHaveScreenshot('container-history.png')
    await expect(smartPanel(page)).toHaveScreenshot('smart-history.png')
  })
})
