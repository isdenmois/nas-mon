import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { pollWhileVisible } from './polling'

function setVisibility(state: DocumentVisibilityState): void {
  Object.defineProperty(document, 'visibilityState', { value: state, configurable: true })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('pollWhileVisible', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setVisibility('visible')
  })

  afterEach(() => {
    vi.useRealTimers()
    setVisibility('visible')
  })

  it('runs the callback on every interval while visible', () => {
    // Arrange
    const fn = vi.fn()
    const stop = pollWhileVisible(fn, 10_000)

    // Act
    vi.advanceTimersByTime(30_000)

    // Assert
    expect(fn).toHaveBeenCalledTimes(3)
    stop()
  })

  it('skips polls while the tab is hidden', () => {
    // Arrange
    const fn = vi.fn()
    const stop = pollWhileVisible(fn, 10_000)

    // Act
    setVisibility('hidden')
    vi.advanceTimersByTime(60_000)

    // Assert
    expect(fn).not.toHaveBeenCalled()
    stop()
  })

  it('refreshes immediately when the tab becomes visible again', () => {
    // Arrange
    const fn = vi.fn()
    const stop = pollWhileVisible(fn, 10_000)
    setVisibility('hidden')
    vi.advanceTimersByTime(60_000)

    // Act
    setVisibility('visible')

    // Assert
    expect(fn).toHaveBeenCalledTimes(1)
    stop()
  })

  it('stops polling and listening after the returned function runs', () => {
    // Arrange
    const fn = vi.fn()
    const stop = pollWhileVisible(fn, 10_000)
    stop()

    // Act
    vi.advanceTimersByTime(60_000)
    setVisibility('hidden')
    setVisibility('visible')

    // Assert
    expect(fn).not.toHaveBeenCalled()
  })

  it('starts polling again after a later resume', () => {
    // Arrange
    const fn = vi.fn()
    const stop = pollWhileVisible(fn, 10_000)

    // Act
    setVisibility('hidden')
    vi.advanceTimersByTime(50_000)
    setVisibility('visible')
    vi.advanceTimersByTime(20_000)

    // Assert
    expect(fn).toHaveBeenCalledTimes(3)
    stop()
  })
})
