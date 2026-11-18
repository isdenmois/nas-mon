/**
 * Runs fn every intervalMs, but only while the tab is visible, and refreshes
 * once when the tab comes back. A dashboard left open in a background tab
 * otherwise keeps polling the API and redrawing charts nobody is looking at.
 *
 * Returns a function that stops the polling and the visibility listener.
 */
export function pollWhileVisible(fn: () => void, intervalMs: number): () => void {
  const timer = setInterval(() => {
    if (document.visibilityState === 'hidden') return
    fn()
  }, intervalMs)

  const onVisibilityChange = (): void => {
    if (document.visibilityState === 'visible') fn()
  }
  document.addEventListener('visibilitychange', onVisibilityChange)

  return () => {
    clearInterval(timer)
    document.removeEventListener('visibilitychange', onVisibilityChange)
  }
}
