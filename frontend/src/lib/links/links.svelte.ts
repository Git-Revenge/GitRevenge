import { LinkRules } from '../../../wailsjs/go/app/App.js'
import type { links } from '../../../wailsjs/go/models'
import { EventsOn } from '../../../wailsjs/runtime/runtime.js'
import { makeLinker, webOnly, type Linker } from './links'

/** Each repository's smart links, read from git config ('' for global only), and the
 * matcher built from them. A change to the global config reloads them all. */
class LinksStore {
  configs = $state<Record<string, links.Config>>({})
  #linkers = new Map<links.Config, Linker>()
  #listening = false

  /** The matcher for a repository; web addresses only until its rules have loaded. */
  linker(path: string): Linker {
    const cfg = this.configs[path]
    if (!cfg) return webOnly
    let l = this.#linkers.get(cfg)
    if (!l) {
      l = makeLinker(cfg)
      this.#linkers.set(cfg, l)
    }
    return l
  }

  async load(path: string) {
    this.#listen()
    try {
      const cfg = await LinkRules(path)
      this.configs[path] = cfg
    } catch {
      // Without them, only web addresses link.
    }
  }

  #listen() {
    if (this.#listening) return
    this.#listening = true
    try {
      EventsOn('gitconfig:changed', () => {
        for (const p of Object.keys(this.configs)) void this.load(p)
      })
    } catch {
      // Outside the app (tests), there are no events.
    }
  }
}

export const linkStore = new LinksStore()
