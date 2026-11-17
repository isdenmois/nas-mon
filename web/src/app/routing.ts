export type Page = 'overview' | 'charts'

const titles: Record<Page, string> = {
  overview: 'mon · homelab monitor',
  charts: 'mon · charts',
}

export function pageFromPath(pathname: string): Page {
  return pathname.replace(/\/+$/, '') === '/charts' ? 'charts' : 'overview'
}

export function pageTitle(page: Page): string {
  return titles[page]
}
