import { expect, test } from '@playwright/test'
import { emptyOverview, overview } from './fixtures'
import { freezeClock, mockApi } from './mock-api'

test.beforeEach(async ({ page }) => {
  await freezeClock(page)
})

test('renders the current snapshot', async ({ page }) => {
  const calls = await mockApi(page)
  await page.goto('/')

  await expect(page).toHaveTitle('mon · homelab monitor')
  await expect(page.locator('.updated span')).toHaveText(/Jan 02, 2026/)
  expect(calls.overview).toBe(1)

  const temps = page.locator('.temp-tile')
  await expect(temps).toHaveCount(3)
  await expect(temps.filter({ hasText: 'k10temp' })).toContainText('54.2')
  await expect(temps.filter({ hasText: 'k10temp' })).toContainText('Tctl')
  await expect(page.locator('.temp-tile.warn')).toContainText('81.5')
  await expect(page.locator('.temp-tile.warn')).toContainText('Warning')

  await expect(page.getByRole('progressbar', { name: 'CPU usage' })).toHaveAttribute(
    'aria-valuenow',
    '12',
  )
  await expect(page.getByRole('progressbar', { name: 'Memory usage' })).toHaveAttribute(
    'aria-valuenow',
    '19',
  )
  await expect(page.locator('.loads')).toContainText('1m')
  await expect(page.locator('.net')).toHaveCount(2)

  const containers = page.locator('section[aria-labelledby="containers-title"]')
  await expect(containers).toContainText('3 running')
  const rows = containers.locator('tbody tr')
  await expect(rows).toHaveCount(3)
  // The table sorts by CPU descending.
  await expect(rows.nth(0)).toContainText('jellyfin')
  await expect(rows.nth(1)).toContainText('postgres')
  await expect(rows.nth(2)).toContainText('mon')
  await expect(rows.nth(0)).toContainText('37.5%')

  const volumes = page.locator('section[aria-labelledby="volumes-title"]')
  await expect(volumes).toContainText('2 mounted')
  await expect(volumes).toContainText('/mnt/media')
  await expect(
    page.getByRole('progressbar', { name: 'Usage of /mnt/media', exact: true }),
  ).toHaveAttribute('aria-valuenow', '91')

  const smart = page.locator('section[aria-labelledby="smart-title"]')
  await expect(smart).toContainText('2 disks')
  const smartRows = smart.locator('tbody tr')
  await expect(smartRows.nth(0)).toContainText('Samsung SSD 990 PRO 2TB')
  await expect(smartRows.nth(0)).toContainText('8.0 TB')
  await expect(smartRows.nth(1)).toContainText('82%')
  await expect(smartRows.nth(1)).toContainText('high')
  await expect(smartRows.nth(1)).toContainText('errors')
  await expect(smartRows.nth(1)).toContainText('realloc')

  await expect(page.getByRole('button', { name: 'temp: ok', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'smart: ok', exact: true })).toBeVisible()
})

test('shows empty states when nothing has been collected', async ({ page }) => {
  await mockApi(page, { overview: emptyOverview })
  await page.goto('/')

  await expect(page.getByText('No temperature sensors reported.')).toBeVisible()
  await expect(page.getByText('Host metrics unavailable')).toBeVisible()
  await expect(page.getByText('No running containers', { exact: true })).toBeVisible()
  await expect(page.getByText('No filesystems mounted under /volumes')).toBeVisible()
  await expect(page.getByText('No SMART data', { exact: true })).toBeVisible()
  await expect(page.getByText('No collector status yet.')).toBeVisible()
  await expect(page.locator('.updated span')).toHaveText(/Jan 02, 2026/)
})

test('reports a failed overview and lets the user dismiss it', async ({ page }) => {
  await mockApi(page, { overviewError: 'collector exploded' })
  await page.goto('/')

  const banner = page.getByRole('alert')
  await expect(banner).toContainText(
    'Could not refresh overview: collector exploded. No data loaded yet.',
  )
  await expect(page.getByText('No temperature sensors reported.')).toBeVisible()

  await banner.getByRole('button', { name: 'Dismiss notification' }).click()
  await expect(page.getByRole('alert')).toHaveCount(0)
})

test('reveals the collector error behind a failing chip', async ({ page }) => {
  await mockApi(page, {
    overview: {
      ...overview,
      collectors: {
        ...overview.collectors,
        volume: {
          last_error: 'statfs /volumes/media: permission denied',
          duration_ms: 1,
          runs: 3,
        },
      },
    },
  })
  await page.goto('/')

  await page.getByRole('button', { name: 'volume: error', exact: true }).click()
  await expect(page.getByRole('status')).toContainText(
    'volume: statfs /volumes/media: permission denied',
  )
})

test('navigates between overview and charts', async ({ page }) => {
  await mockApi(page)
  await page.goto('/')

  await page.getByRole('link', { name: 'Charts' }).click()
  await expect(page).toHaveURL(/\/charts$/)
  await expect(page).toHaveTitle('mon · charts')
  await expect(page.getByRole('heading', { name: 'Temperature history' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Charts' })).toHaveAttribute(
    'aria-current',
    'page',
  )

  await page.getByRole('link', { name: 'Overview' }).click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', { name: 'Temperatures' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Overview' })).toHaveAttribute(
    'aria-current',
    'page',
  )

  await page.goBack()
  await expect(page).toHaveURL(/\/charts$/)
  await expect(page.getByRole('heading', { name: 'Host history' })).toBeVisible()
})
