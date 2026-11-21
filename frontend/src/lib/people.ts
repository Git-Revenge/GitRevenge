/** A name and an address: an author or a committer, as a commit has them or as the
 * mailmap reads them (git.Identity in Go). */
export interface Who {
  name: string
  email: string
}

/** Who to show for a commit: the mailmap's reading of them when there is one, and what
 * the commit says otherwise. */
export function shownAs(mapped: Who | undefined, written: Who): Who {
  return mapped?.name || mapped?.email ? mapped : written
}

/**
 * What the commit says, as "Name <address>", when the mailmap reads it as someone else,
 * and null when the two are the same. A mailmap never changes a commit, so the panel says
 * both: who they're shown as, and what was written.
 */
export function writtenAs(mapped: Who | undefined, written: Who): string | null {
  const shown = shownAs(mapped, written)
  if (shown.name === written.name && shown.email === written.email) return null
  return written.email ? `${written.name} <${written.email}>` : written.name
}
