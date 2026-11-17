import { describe, expect, it } from 'vitest'
import { pageFromPath, pageTitle } from './routing'

describe('pageFromPath', () => {
  it('should resolve the overview route', () => {
    // Arrange
    const paths = ['/', '']

    // Act
    const pages = paths.map(pageFromPath)

    // Assert
    expect(pages).toEqual(['overview', 'overview'])
  })

  it('should resolve the charts route with or without a trailing slash', () => {
    // Arrange
    const paths = ['/charts', '/charts/']

    // Act
    const pages = paths.map(pageFromPath)

    // Assert
    expect(pages).toEqual(['charts', 'charts'])
  })

  it('should treat unknown paths as the overview', () => {
    // Arrange
    const path = '/nope'

    // Act
    const page = pageFromPath(path)

    // Assert
    expect(page).toBe('overview')
  })
})

describe('pageTitle', () => {
  it('should title each page', () => {
    // Arrange
    const pages = ['overview', 'charts'] as const

    // Act
    const titles = pages.map(pageTitle)

    // Assert
    expect(titles).toEqual(['mon · homelab monitor', 'mon · charts'])
  })
})
