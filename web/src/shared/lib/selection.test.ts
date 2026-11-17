import { fireEvent, render, screen } from '@testing-library/vue'
import { defineComponent, h, type PropType } from 'vue'
import { describe, expect, it } from 'vitest'
import { useAutoSelection } from './selection'

// A minimal checkbox list so the composable runs inside a real component.
function mount(available: string[]) {
  const Host = defineComponent({
    props: { available: { type: Array as PropType<string[]>, required: true } },
    setup(props) {
      const selected = useAutoSelection(() => props.available)
      return () =>
        h('div', [
          ...props.available.map((key) =>
            h('label', [
              h('input', {
                type: 'checkbox',
                value: key,
                checked: selected.value.includes(key),
                onChange: (event: Event) => {
                  const target = event.target as HTMLInputElement
                  selected.value = target.checked
                    ? [...selected.value, key]
                    : selected.value.filter((item) => item !== key)
                },
              }),
              key,
            ]),
          ),
          h('output', selected.value.join(',')),
        ])
    },
  })

  return render(Host, { props: { available } })
}

const selection = (): string => screen.getByRole('status').textContent ?? ''

describe('useAutoSelection', () => {
  it('should select every key once data first arrives', async () => {
    // Arrange
    const view = mount([])

    // Act
    await view.rerender({ available: ['a', 'b'] })

    // Assert
    expect(selection()).toBe('a,b')
  })

  it('should keep the user choice when the list changes', async () => {
    // Arrange
    const view = mount(['a', 'b'])
    await fireEvent.click(screen.getByRole('checkbox', { name: 'b' }))

    // Act
    await view.rerender({ available: ['a', 'b', 'c'] })

    // Assert
    expect(selection()).toBe('a')
  })

  it('should drop keys that disappear from the list', async () => {
    // Arrange
    const view = mount(['a', 'b'])
    await fireEvent.click(screen.getByRole('checkbox', { name: 'b' }))

    // Act
    await view.rerender({ available: ['b'] })

    // Assert
    expect(selection()).toBe('')
  })
})
