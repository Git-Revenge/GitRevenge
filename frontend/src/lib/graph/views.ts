import type { IconName } from '../ui/icons'
import type { Density } from './draw'

/** A way of looking at the graph. Timeline and compact are its rows at two densities; the rest
 * are experimental, and show only once switched on in settings. */
export type GraphView = 'timeline' | 'compact' | (string & {})

export interface ViewInfo {
  id: GraphView
  label: string
  icon: IconName
  /** What the view is, in a line, for the menu item's tooltip. */
  title: string
  experimental: boolean
}

/** Every view this version has, in menu order: the rows first, then the experiments. */
export const VIEWS: ViewInfo[] = [
  { id: 'timeline', label: 'Timeline', icon: 'timeline', title: 'Two lines for a commit that starts a run, with authors', experimental: false },
  { id: 'compact', label: 'Compact', icon: 'rows', title: 'One line per commit', experimental: false },
  { id: 'horizontal', label: 'Horizontal', icon: 'across', title: 'Time runs across: a column per commit, newest on the left', experimental: true },
]

/** The views the menu lists: the rows, and the experimental views switched on. */
export function menuViews(switchedOn: readonly string[], all: readonly ViewInfo[] = VIEWS): ViewInfo[] {
  return all.filter((v) => !v.experimental || switchedOn.includes(v.id))
}

/** The view showing, from what's saved. An experimental view that's switched off, or one this
 * version doesn't have, reads as the rows at the saved density. */
export function currentView(density: Density, saved: string, switchedOn: readonly string[], all: readonly ViewInfo[] = VIEWS): GraphView {
  if (saved && switchedOn.includes(saved) && all.some((v) => v.experimental && v.id === saved)) return saved
  return density === 'compact' ? 'compact' : 'timeline'
}

/** What choosing a view saves: the experimental view, or '' for the rows, and the rows' density
 * when the choice is one of them (an experimental view leaves the density as it was). */
export function savedFor(view: GraphView): { view: string; density?: Density } {
  if (view === 'timeline') return { view: '', density: 'comfortable' }
  if (view === 'compact') return { view: '', density: 'compact' }
  return { view }
}

/** The experimental views, with what the settings panel says of each. */
export const EXPERIMENTS: { id: string; label: string; about: string }[] = [{ id: 'horizontal', label: 'Horizontal', about: 'Time runs across: a column for each commit and a row for each lane, newest on the left.' }]

/** Whether the view is the graph's rows, at either density. */
export function isRows(view: GraphView): boolean {
  return view === 'timeline' || view === 'compact'
}
