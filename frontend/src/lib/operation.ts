import type { app } from '../../wailsjs/go/models'
import type { BisectMarks } from './bisect.svelte'

// The words of the strip for a merge, rebase, cherry-pick or revert in progress (issue #593),
// kept apart from the strip so they can be tested as plain functions.

export type OpState = Pick<app.RepoState, 'operation' | 'step' | 'total' | 'conflicts' | 'ours' | 'theirs' | 'commit' | 'subject' | 'origHead' | 'written'>

const short = (id: string) => id.slice(0, 7)
const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`
const DETACHED = 'a detached HEAD'

/** The operation as a noun, for "Abort the rebase". */
export const OP_NOUN: Record<string, string> = { merge: 'merge', rebase: 'rebase', 'cherry-pick': 'cherry-pick', revert: 'revert', am: 'patches' }

/** The strip's heading in parts, so the names can be set in the code face: "Rebasing" topic
 * "onto" main. A part is "" when there's nothing to say for it. */
export function opTitle(s: OpState): { verb: string; a: string; join: string; b: string } {
  const ours = s.ours || DETACHED
  switch (s.operation) {
    case 'rebase':
      return { verb: 'Rebasing', a: ours, join: s.theirs ? 'onto' : '', b: s.theirs }
    case 'merge':
      return { verb: 'Merging', a: s.theirs || short(s.commit), join: 'into', b: ours }
    case 'cherry-pick':
      return { verb: 'Cherry-picking', a: short(s.commit), join: 'onto', b: ours }
    case 'revert':
      return { verb: 'Reverting', a: short(s.commit), join: 'on', b: ours }
    case 'am':
      return { verb: 'Applying patches', a: '', join: '', b: '' }
  }
  return { verb: s.operation, a: '', join: '', b: '' }
}

/** The line under the heading: where it stopped, or that it's ready to go on. seen is the most
 * files that have been in conflict at this stop, so a merge can count them down. */
export function opNote(s: OpState, seen: number): string {
  const n = s.conflicts.length
  const steps = s.total > 0
  if (s.operation === 'merge') {
    if (n > 0) return seen > n ? `${seen - n} of ${count(seen, 'file', 'files')} resolved.` : `${count(n, 'file needs', 'files need')} resolving before the merge can be committed.`
    return seen > 0 ? `${seen === 1 ? 'The conflict is' : `All ${seen} conflicts are`} resolved. Commit the merge to finish.` : 'Nothing is in conflict. Commit the merge to finish.'
  }
  const where = steps ? `commit ${s.step} of ${s.total}` : s.commit ? short(s.commit) : 'this commit'
  if (n > 0 || seen === 0) return `Stopped at ${where}${s.subject ? `: ${s.subject}` : '.'}`
  return `${where[0].toUpperCase()}${where.slice(1)} is resolved and staged.`
}

/** What Continue says: what pressing it will do. */
export function continueLabel(s: OpState): string {
  if (s.operation === 'merge') return 'Commit the merge'
  if (s.operation === 'rebase' && s.total > 0) {
    const left = s.total - s.step
    return left > 0 ? `Continue with ${left} more` : 'Finish the rebase'
  }
  if (s.operation === 'cherry-pick') return 'Finish the cherry-pick'
  if (s.operation === 'revert') return 'Finish the revert'
  return 'Continue'
}

/** Why Continue is off, for its tooltip: the files still in conflict. "" when it's on. */
export function blockedBy(s: OpState): string {
  const [first, ...rest] = s.conflicts
  if (!first) return ''
  return `Resolve ${first}${rest.length ? ` and ${count(rest.length, 'other', 'others')}` : ''} first`
}

/** What aborting undoes, said before it's done. */
export function abortEffect(s: OpState, seen: number): string {
  const ours = s.ours || 'HEAD'
  const at = s.origHead ? short(s.origHead) : 'where it was'
  const back = s.operation === 'rebase' ? `${ours} goes back to ${at}, where it was before.` : `${ours} stays at ${at}, as it was before the ${OP_NOUN[s.operation] ?? s.operation}.`
  const lost: string[] = []
  if (s.written.length) lost.push(`the ${count(s.written.length, 'commit', 'commits')} already written again`)
  if (seen > 0) lost.push('your work on the conflicts')
  if (!lost.length) return back
  const what = lost.join(' and ')
  const one = lost.length === 1 && (seen > 0 || s.written.length === 1)
  return `${back} ${what[0].toUpperCase()}${what.slice(1)} ${one ? 'is' : 'are'} thrown away.`
}

/** What to say after an abort. */
export function abortedMessage(s: OpState): string {
  const noun = OP_NOUN[s.operation] ?? s.operation
  const at = s.origHead ? ` ${s.ours || 'HEAD'} is back at ${short(s.origHead)}.` : ''
  return `${noun[0].toUpperCase()}${noun.slice(1)} aborted.${at}`
}

/** What to say once it's over. */
export function finishedMessage(s: OpState): string {
  const t = opTitle(s)
  switch (s.operation) {
    case 'rebase':
      return `Rebased ${t.a}${t.b ? ` onto ${t.b}` : ''}.`
    case 'merge':
      return `Merged ${t.a} into ${t.b}.`
    case 'cherry-pick':
      return `Cherry-picked ${t.a} onto ${t.b}.`
    case 'revert':
      return `Reverted ${t.a} on ${t.b}.`
  }
  return 'Done.'
}

/** A bead per step of a rebase, up to a dozen: done, the one in hand, and those to come. */
export function beads(s: OpState): ('done' | 'bad' | 'now' | '')[] {
  if (s.operation !== 'rebase' || s.total < 1 || s.total > 12) return []
  return Array.from({ length: s.total }, (_, i) => (i + 1 < s.step ? 'done' : i + 1 === s.step ? (s.conflicts.length ? 'bad' : 'now') : ''))
}

/** A rebase in progress as the graph marks it: the commits written so far, and the one in hand
 * with its number. */
export function rebaseMarks(s: OpState | null | undefined): BisectMarks | null {
  if (s?.operation !== 'rebase') return null
  const terms: BisectMarks['terms'] = {}
  for (const id of s.written) terms[id] = 'good'
  const note = s.total > 0 ? `${s.step} of ${s.total}, ${s.conflicts.length ? 'stopped here' : 'in hand'}` : 'in hand'
  // One digit fits the mark; past nine the note beside it still says which.
  const glyph = s.step > 0 && s.step < 10 ? String(s.step) : '…'
  return { terms, inQuestion: null, current: s.commit, found: '', currentNote: note, currentGlyph: glyph }
}
