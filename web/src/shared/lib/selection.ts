import { ref, watch } from 'vue'

// useAutoSelection tracks a checkbox selection over a changing list of keys.
// The first non-empty list selects everything; afterwards removed keys drop out
// while the user's choices survive list updates (e.g. a container restarting).
export function useAutoSelection(available: () => string[]) {
  const selected = ref<string[]>([])
  let initialized = false

  watch(
    () => available().join('\u0000'),
    () => {
      const keys = available()
      if (!initialized) {
        if (keys.length === 0) return
        initialized = true
        selected.value = [...keys]
        return
      }
      const known = new Set(keys)
      const kept = selected.value.filter((key) => known.has(key))
      if (kept.length !== selected.value.length) selected.value = kept
    },
    { immediate: true },
  )

  return selected
}
