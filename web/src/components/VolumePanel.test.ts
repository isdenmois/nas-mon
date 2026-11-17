import { render, screen } from '@testing-library/vue'
import { describe, expect, it } from 'vitest'
import type { VolumeSnapshot } from '../types'
import VolumePanel from './VolumePanel.vue'

function volume(overrides: Partial<VolumeSnapshot> = {}): VolumeSnapshot {
  return {
    name: '/',
    container_path: '/volumes/root',
    used: 146 * 1024 ** 3,
    total: 512 * 1024 ** 3,
    used_pct: 28.5,
    ...overrides,
  }
}

describe('VolumePanel', () => {
  it('should render a volume with usage and path', () => {
    // Arrange
    render(VolumePanel, { props: { volumes: [volume()] } })

    // Act
    const usageBar = screen.getByRole('progressbar', { name: 'Usage of /' })

    // Assert
    expect(screen.getByText('1 mounted')).toBeInTheDocument()
    expect(screen.getByText('/volumes/root')).toBeInTheDocument()
    expect(screen.getByText(/146 GB .* 512 GB/)).toBeInTheDocument()
    expect(usageBar).toHaveAttribute('aria-valuenow', '29')
  })

  it('should render every volume', () => {
    // Arrange
    const volumes = [
      volume(),
      volume({
        name: '/mnt/media',
        container_path: '/volumes/media',
        used: 7.3 * 1024 ** 4,
        total: 8 * 1024 ** 4,
        used_pct: 91.3,
      }),
    ]

    // Act
    render(VolumePanel, { props: { volumes } })

    // Assert
    expect(screen.getByText('2 mounted')).toBeInTheDocument()
    expect(screen.getByText('/volumes/root')).toBeInTheDocument()
    expect(screen.getByText('/volumes/media')).toBeInTheDocument()
  })

  it('should clamp a usage percentage above one hundred', () => {
    // Arrange
    const volumes = [volume({ used_pct: 150 })]

    // Act
    render(VolumePanel, { props: { volumes } })

    // Assert
    expect(screen.getByRole('progressbar', { name: 'Usage of /' })).toHaveAttribute(
      'aria-valuenow',
      '100',
    )
  })

  it('should show an empty state when no filesystems are mounted', () => {
    // Arrange
    const volumes: VolumeSnapshot[] = []

    // Act
    render(VolumePanel, { props: { volumes } })

    // Assert
    expect(screen.getByText('No filesystems mounted under /volumes')).toBeInTheDocument()
  })
})
