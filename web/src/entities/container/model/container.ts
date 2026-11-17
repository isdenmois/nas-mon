import type { ContainerSnapshot } from '@/shared/api'
import { chartColor } from '@/shared/lib'

export function sortByCpu(containers: ContainerSnapshot[]): ContainerSnapshot[] {
  return [...containers].sort((a, b) => b.cpu_pct - a.cpu_pct)
}

export function containerColor(names: string[], name: string): string {
  return chartColor(names.indexOf(name))
}
