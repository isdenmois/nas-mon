import { fireEvent, render, screen } from '@testing-library/vue'
import { describe, expect, it } from 'vitest'
import type { CollectorStatus } from '../types'
import StatusChips from './StatusChips.vue'

function collectors(): Record<string, CollectorStatus> {
  return {
    z: { last_success: '2026-01-02T03:04:05Z', duration_ms: 3, runs: 5 },
    a: { last_error: 'timeout', duration_ms: 2, runs: 1 },
    b: { duration_ms: 0, runs: 0 },
  }
}

describe('StatusChips', () => {
  it('should render okay, failed, and idle collectors', () => {
    // Arrange
    const status = collectors()

    // Act
    render(StatusChips, { props: { collectors: status } })

    // Assert
    expect(screen.getByRole('button', { name: 'z: ok' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'a: error' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'b: idle' })).toBeInTheDocument()
  })

  it('should sort collectors by name', () => {
    // Arrange
    const status = collectors()

    // Act
    render(StatusChips, { props: { collectors: status } })

    // Assert
    const names = screen.getAllByRole('button').map((button) => button.textContent?.trim())
    expect(names).toEqual(['a: error', 'b: idle', 'z: ok'])
  })

  it('should reveal the error message when a failed collector is clicked', async () => {
    // Arrange
    render(StatusChips, { props: { collectors: collectors() } })
    const button = screen.getByRole('button', { name: 'a: error' })

    // Act
    await fireEvent.click(button)

    // Assert
    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('status')).toHaveTextContent('a: timeout')
  })

  it('should hide the error message when the same collector is clicked twice', async () => {
    // Arrange
    render(StatusChips, { props: { collectors: collectors() } })
    const button = screen.getByRole('button', { name: 'a: error' })

    // Act
    await fireEvent.click(button)
    await fireEvent.click(button)

    // Assert
    expect(button).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('should include the last success date in the chip tooltip', () => {
    // Arrange
    render(StatusChips, { props: { collectors: collectors() } })

    // Act
    const button = screen.getByRole('button', { name: 'z: ok' })

    // Assert
    expect(button).toHaveAttribute('title', expect.stringContaining('Last success'))
    expect(button).toHaveAttribute('title', expect.stringContaining('2026'))
  })

  it('should show an empty state when no collector status is available', () => {
    // Arrange
    const status: Record<string, CollectorStatus> = {}

    // Act
    render(StatusChips, { props: { collectors: status } })

    // Assert
    expect(screen.getByText('No collector status yet.')).toBeInTheDocument()
  })

  it('should not reveal an error for a healthy collector', async () => {
    // Arrange
    render(StatusChips, { props: { collectors: collectors() } })
    const button = screen.getByRole('button', { name: 'z: ok' })

    // Act
    await fireEvent.click(button)

    // Assert
    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })
})
