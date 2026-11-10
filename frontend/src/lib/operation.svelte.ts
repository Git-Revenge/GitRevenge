import type { app } from '../../wailsjs/go/models'

/** Each open repository's operation in progress, as the strip last read it, and the strip's
 * request to open a conflicted file (issue #593). The strip sits above the repository view,
 * so this is how the two talk. */
class Operations {
  state = $state<Record<string, app.RepoState | null>>({})
  /** A conflicted file the strip asked to open, until the repository view takes it. */
  resolve = $state<{ path: string; file: string } | null>(null)

  of(path: string): app.RepoState | null {
    const s = this.state[path]
    return s?.operation ? s : null
  }
}

export const operations = new Operations()
