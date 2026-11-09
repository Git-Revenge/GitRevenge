import { GraphDensity, SetGraphDensity, SetTheme, Theme } from '../../wailsjs/go/app/App.js'
import type { Density } from './graph/draw'
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

/** Appearance: the colour theme, applied as data-theme on the document root, and the
 * graph's row density. */
class ThemeState {
  mode = $state<Mode>('')
  density = $state<Density>('comfortable')
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
      const [mode, density] = await Promise.all([Theme(), GraphDensity()])
      this.mode = mode as Mode
      this.density = density === 'compact' ? 'compact' : 'comfortable'
    } catch {
      // Keep the defaults.
    }
  }

  /** Switches the graph between the timeline and compact rows, and saves the choice. */
  async toggleDensity() {
    this.density = this.density === 'compact' ? 'comfortable' : 'compact'
    await SetGraphDensity(this.density === 'compact' ? 'compact' : '')
  }

  /** Switches to the other theme and saves that as the choice. */
  async toggle() {
    this.mode = this.resolved === 'dark' ? 'light' : 'dark'
    await SetTheme(this.mode)
  }
}

export const theme = new ThemeState()
