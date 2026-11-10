import type { app } from '../../wailsjs/go/models'

// The words of the merge and rebase previews (issue #595), kept apart from the dialog so they
// can be tested as plain functions.

/** How a merge is recorded: the modes Integrate's merge takes. */
export type MergeHow = 'no-ff' | 'ff-only' | 'squash'

export type MergeChoice = { how: MergeHow; label: string; off: boolean; why: string }

/** "1 file", "3 files". */
export const plural = (n: number, one: string, many: string) => `${n.toLocaleString()} ${n === 1 ? one : many}`

type Merge = Pick<app.MergePreview, 'fastForward' | 'conflicts' | 'unknown' | 'intoOnly' | 'commitCount'>

/** The ways to merge, in the order they're offered, with those that can't apply turned off and
 * why. The first that's on is the default: a fast-forward when one is possible. */
export function mergeChoices(p: Merge, into: string, from: string): MergeChoice[] {
  const conflicts = !p.fastForward && p.conflicts.length > 0
  return [
    {
      how: 'ff-only',
      label: 'Fast-forward only',
      off: !p.fastForward,
      why: p.fastForward ? '' : `It can't fast-forward: ${into} has ${plural(p.intoOnly, 'commit', 'commits')} ${from} doesn't.`,
    },
    { how: 'no-ff', label: 'Merge commit', off: false, why: '' },
    {
      how: 'squash',
      label: 'Squash into one',
      off: conflicts,
      why: conflicts ? "Squash is off while files conflict: a squash that stops can't be aborted the way a merge can." : '',
    },
  ]
}

/** What the merge button says: what pressing it will do. */
export function mergeVerb(p: Merge, how: MergeHow, into: string): string {
  if (how === 'ff-only') return `Fast-forward ${into}`
  if (how === 'squash') return `Squash into ${into}`
  if (!p.unknown && !p.fastForward && p.conflicts.length > 0) return `Merge, and resolve ${plural(p.conflicts.length, 'conflict', 'conflicts')}`
  return 'Merge'
}

/** One line on what the merge will make, and how to take it back. */
export function mergeSummary(p: Merge, how: MergeHow, into: string, from: string): string {
  if (how === 'ff-only') return `${into} simply moves forward to ${from}: no merge commit is made. Undo puts ${into} back where it was.`
  if (how === 'squash')
    return `Everything ${from} changed is staged as one change for you to commit. Nothing is committed yet, and ${from} isn't recorded as merged, so keep or delete it yourself.`
  if (p.fastForward) return `${into} could simply move forward, but this makes a merge commit anyway, so the branch stays visible in the history. Undo puts ${into} back where it was.`
  return `This makes a merge commit on ${into} that joins the two. Undo puts ${into} back where it was.`
}

/** What the rebase button says. */
export function rebaseVerb(p: Pick<app.RebasePreview, 'commitCount'>, edit: boolean): string {
  return edit ? 'Choose what to do with each commit' : `Rebase ${plural(p.commitCount, 'commit', 'commits')}`
}
