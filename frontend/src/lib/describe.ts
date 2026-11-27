import type { git } from '../../wailsjs/go/models'

// What the commit panel's Details tab says about a commit's place among the tags (#940).

type Described = Pick<git.Described, 'name' | 'after' | 'since' | 'firstIn'>

/** The line under the describe name: which tag the commit comes after, and how far. */
export function afterSaid(d: Described): string {
  if (!d.after) return 'No tag comes before it.'
  if (d.since === 0) return `It’s the commit ${d.after} tags.`
  return `${d.since.toLocaleString()} ${d.since === 1 ? 'commit' : 'commits'} after ${d.after}`
}

/** The first tag that has the commit, or why there's none. */
export function firstInSaid(d: Described): string {
  return d.firstIn ? '' : 'No tag has it yet.'
}
