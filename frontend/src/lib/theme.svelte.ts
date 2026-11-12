import { GraphDensity, GraphViews, SetGraphDensity, SetGraphView, SetTheme, Theme } from '../../wailsjs/go/app/App.js'
import type { Density } from './graph/draw'
import { currentView, savedFor, type GraphView } from './graph/views'
import { reducedMotion } from './ui/motion'

type Resolved = 'light' | 'dark'
/** A saved choice, or '' to follow the operating system. */
type Mode = '' | Resolved

const media = window.matchMedia('(prefers-color-scheme: dark)')

type Transitions = Document & { startViewTransition?: (update: () => Promise<void>) => unknown }

/** Sets data-theme on the document root. A change of theme cross-fades the whole window through
 * a view transition where the engine has one (WebView2 and WebKit do), over --dur-base from
 * style.css; the first paint, and anyone asking for less motion, switch at once. The update
 * waits two frames so the graph's canvas, which repaints on the next frame, is in the new
 * picture rather than flipping after the fade. */
function applyTheme(theme: Resolved) {
  const root = document.documentElement
  if (root.dataset.theme === theme) return
  const doc = document as Transitions
  if (!root.dataset.theme || !doc.startViewTransition || reducedMotion()) {
    root.dataset.theme = theme
    return
  }
  doc.startViewTransition(() => {
    root.dataset.theme = theme
    return new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done())))
  })
}

/** Appearance: the colour theme, applied as data-theme on the document root, the graph's row
 * density, and which view of the graph shows. */
class ThemeState {
  mode = $state<Mode>('')
  density = $state<Density>('comfortable')
  /** The experimental views switched on in settings, and the one saved as showing ('' for the
   * rows). */
  experimental = $state<string[]>([])
  savedView = $state('')
  /** The view of the graph that shows: its rows at the saved density, unless an experimental
   * view is both chosen and switched on. */
  view: GraphView = $derived(currentView(this.density, this.savedView, this.experimental))
  system = $state<Resolved>(media.matches ? 'dark' : 'light')
  resolved: Resolved = $derived(this.mode || this.system)

  constructor() {
    // Set before the first render so the page never paints in the wrong theme.
    document.documentElement.dataset.theme = this.resolved
    media.addEventListener('change', (e) => (this.system = e.matches ? 'dark' : 'light'))
    $effect.root(() => {
      $effect(() => {
        applyTheme(this.resolved)
      })
    })
  }

  /** Loads the saved choices. Until they arrive, and if they can't be read, the OS picks the
   * theme and the graph uses the timeline. */
  async load() {
    try {
      const [mode, density, views] = await Promise.all([Theme(), GraphDensity(), GraphViews()])
      this.mode = mode as Mode
      this.density = density === 'compact' ? 'compact' : 'comfortable'
      this.experimental = views?.experimental ?? []
      this.savedView = views?.view ?? ''
    } catch {
      // Keep the defaults.
    }
  }

  /** Switches the graph between the timeline and compact rows, and saves the choice. From an
   * experimental view it goes to the rows as they aren't now, as the command's title says. */
  async toggleDensity() {
    await this.setView(this.density === 'compact' ? 'timeline' : 'compact')
  }

  /** Shows the graph as the given view and saves the choice. */
  async setView(view: GraphView) {
    const next = savedFor(view)
    const viewChanged = next.view !== this.savedView
    const densityChanged = !!next.density && next.density !== this.density
    this.savedView = next.view
    if (next.density) this.density = next.density
    if (viewChanged) await SetGraphView(next.view)
    if (densityChanged) await SetGraphDensity(this.density === 'compact' ? 'compact' : '')
  }

  /** Switches to the other theme and saves that as the choice. */
  async toggle() {
    this.mode = this.resolved === 'dark' ? 'light' : 'dark'
    await SetTheme(this.mode)
  }
}

export const theme = new ThemeState()
