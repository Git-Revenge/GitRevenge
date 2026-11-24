import { Preferences } from '../../wailsjs/go/app/App.js'

/** Settings that act outside the settings panel: the interface zoom, the clone folder,
 * whether forge marks wear their brands' colours, and how diffs are drawn. */
class Prefs {
  zoom = $state(100)
  cloneDir = $state('')
  forgeColors = $state(false)
  /** The split view joins its sides with the ribbon rather than padding them. */
  diffRibbon = $state(true)
  /** The line diff chosen, or '' for git's; the open diff compares again when it changes. */
  diffAlgorithm = $state('')
  /** Moved lines show as moves; the open diff compares again when it changes. */
  diffMoves = $state(true)

  /** Loads the saved settings and applies the zoom. Until then, and if they can't be read,
   * the defaults stand. */
  async load() {
    try {
      const p = await Preferences()
      this.zoom = p.zoom
      this.cloneDir = p.cloneDir
      this.forgeColors = p.forgeColors
      this.diffRibbon = p.diffRibbon
      this.diffAlgorithm = p.diffAlgorithm
      this.diffMoves = p.diffMoves
    } catch {
      // Keep the defaults.
    }
    this.apply()
  }

  /** Scales the whole interface. The UI is sized in pixels, so a root font size wouldn't. */
  apply() {
    document.documentElement.style.zoom = this.zoom === 100 ? '' : `${this.zoom}%`
  }
}

export const prefs = new Prefs()
