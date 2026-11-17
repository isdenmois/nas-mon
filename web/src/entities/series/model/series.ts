import type { SeriesDescriptor } from '@/shared/api'
import type { SeriesIndex } from '@/shared/lib'

// seriesIndex resolves the kind and name a panel knows into the id the API
// assigned. A series that has never been recorded is simply absent, so panels
// render "no history yet" instead of asking for a string the backend must parse.

export function seriesIndex(series: SeriesDescriptor[] | null | undefined): SeriesIndex {
  const ids = new Map<string, number>()
  for (const item of series ?? []) ids.set(`${item.kind}\u0000${item.name}`, item.id)

  const id = (kind: string, name: string): number | undefined => ids.get(`${kind}\u0000${name}`)

  return {
    id,
    resolve: (specs) =>
      specs.flatMap((spec) => {
        const seriesID = id(spec.kind, spec.name)
        return seriesID === undefined
          ? []
          : [{ id: seriesID, label: spec.label, color: spec.color }]
      }),
  }
}
