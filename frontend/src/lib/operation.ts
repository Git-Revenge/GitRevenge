import type { app } from '../../wailsjs/go/models'
import type { BisectMarks } from './bisect.svelte'

// The words of the strip for a merge, rebase, cherry-pick or revert in progress (issue #593),
// kept apart from the strip so they can be tested as plain functions.

export type OpState = Pick<app.RepoState, 'operation' | 'step' | 'total' | 'conflicts' | 'ours' | 'theirs' | 'commit' | 'subject' | 'origHead' | 'written'> &
  Partial<Pick<app.RepoState, 'author' | 'applied' | 'stuck' | 'stuckReason' | 'madeBranch'>>

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
      return { verb: 'Applying patches', a: '', join: 'onto', b: ours }
  }
  return { verb: s.operation, a: '', join: '', b: '' }
}

const capital = (s: string) => `${s[0].toUpperCase()}${s.slice(1)}`

/** Which patch git am has in hand: "patch 4 of 5". */
const patchAt = (s: OpState) => (s.total > 0 ? `patch ${s.step} of ${s.total}` : 'this patch')

/** The patch in hand said: ": Set the alert limit · Tyler Brooks". */
const patchSaid = (s: OpState) => `${s.subject ? `: ${s.subject}` : ''}${s.author ? ` · ${s.author}` : ''}`

/** Why a patch git am stopped on has nothing to resolve, and what can be done (#1023). seen
 * says the stop had conflicts, which were resolved to no change at all. "" for any other stop. */
export function stuckNote(s: OpState, seen: number): string {
  if (s.operation !== 'am' || !s.stuck) return ''
  if (seen > 0) return 'Resolved, it changes nothing, so there’s nothing to commit. It may be in already: skip it, or abort.'
  const why = s.stuckReason ? `Git says “${s.stuckReason.replace(/\.$/, '')}”, and there’s nothing in conflict to resolve.` : 'There’s nothing in conflict to resolve.'
  return `${why} Skip it, abort, or make the change by hand and stage it to bring Continue back.`
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
  if (s.operation === 'am') {
    const which = patchAt(s)
    if (s.stuck) return seen > 0 ? `Nothing of ${which} is left to commit${patchSaid(s) || '.'}` : `${capital(which)} didn’t apply${patchSaid(s) || '.'}`
    if (n > 0 || seen === 0) return `Stopped at ${which}${patchSaid(s) || '.'}`
    return `${capital(which)} is resolved and staged.`
  }
  const where = steps ? `commit ${s.step} of ${s.total}` : s.commit ? short(s.commit) : 'this commit'
  if (n > 0 || seen === 0) return `Stopped at ${where}${s.subject ? `: ${s.subject}` : '.'}`
  return `${where[0].toUpperCase()}${where.slice(1)} is resolved and staged.`
}

/** What Continue says: what pressing it will do. */
export function continueLabel(s: OpState): string {
  if (s.operation === 'merge') return 'Commit the merge'
  if ((s.operation === 'rebase' || s.operation === 'am') && s.total > 0) {
    const left = s.total - s.step
    return left > 0 ? `Continue with ${left} more` : s.operation === 'am' ? 'Finish applying' : 'Finish the rebase'
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
  const back =
    s.operation === 'am'
      ? s.madeBranch
        ? `${s.madeBranch} goes, as it was made for them, and you’re back where you were.`
        : `${ours} goes back to ${at}, where it was before the first patch.`
      : s.operation === 'rebase'
        ? `${ours} goes back to ${at}, where it was before.`
        : `${ours} stays at ${at}, as it was before the ${OP_NOUN[s.operation] ?? s.operation}.`
  const lost: string[] = []
  if (s.written.length) lost.push(`the ${count(s.written.length, 'commit', 'commits')} already written again`)
  if (s.operation === 'am' && s.step > 1) lost.push(`the ${count(s.step - 1, 'patch', 'patches')} already applied`)
  if (seen > 0) lost.push('your work on the conflicts')
  if (!lost.length) return back
  const what = lost.join(' and ')
  const one = lost.length === 1 && (seen > 0 || s.written.length === 1 || (s.operation === 'am' && s.step === 2))
  return `${back} ${what[0].toUpperCase()}${what.slice(1)} ${one ? 'is' : 'are'} thrown away.`
}

/** What to say after an abort. */
export function abortedMessage(s: OpState): string {
  if (s.operation === 'am') {
    if (s.madeBranch) return `Applying the patches stopped, and ${s.madeBranch}, made for them, is gone.`
    return `Applying the patches stopped.${s.origHead ? ` ${s.ours || 'HEAD'} is back at ${short(s.origHead)}.` : ''}`
  }
  const noun = OP_NOUN[s.operation] ?? s.operation
  const at = s.origHead ? ` ${s.ours || 'HEAD'} is back at ${short(s.origHead)}.` : ''
  return `${noun[0].toUpperCase()}${noun.slice(1)} aborted.${at}`
}

/** How many patches a series skipped, once the action taken at s finished it: those skipped
 * before the one in hand, and that one if it was skipped too. Those after it all went in, or
 * git am would have stopped again. */
export function skippedAtEnd(s: OpState, action: string): number {
  if (s.operation !== 'am') return 0
  return Math.max(0, s.step - 1 - (s.applied ?? 0)) + (action === 'skip' ? 1 : 0)
}

/** The Skip button's words. */
export const skipLabel = (s: OpState) => (s.operation === 'am' ? 'Skip this patch' : 'Skip this commit')

/** What to say once it's over. skipped is how many patches were skipped along the way. */
export function finishedMessage(s: OpState, skipped = 0): string {
  const t = opTitle(s)
  switch (s.operation) {
    case 'am': {
      const applied = Math.max(0, s.total - skipped)
      if (!applied) return `No patches were applied; ${skipped === 1 ? 'the one was' : `all ${skipped} were`} skipped.`
      return `Applied ${count(applied, 'patch', 'patches')} onto ${t.b}${skipped ? `; skipped ${skipped}` : ''}.`
    }
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
  if ((s.operation !== 'rebase' && s.operation !== 'am') || s.total < 1 || s.total > 12) return []
  return Array.from({ length: s.total }, (_, i) => (i + 1 < s.step ? 'done' : i + 1 === s.step ? (s.conflicts.length || s.stuck ? 'bad' : 'now') : ''))
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

/** What a stop holds beyond its files (#946): places the conflict tool will suggest combined,
 * the places left undecided beside them, and files a merge driver settled. "" when there's
 * nothing more to say. */
export function stopTally(t: { files: { places: number; suggested: number }[]; settled: { driver: string }[] } | null | undefined): string {
  if (!t) return ''
  const parts: string[] = []
  const suggested = t.files.reduce((n, f) => n + f.suggested, 0)
  const open = t.files.reduce((n, f) => n + f.places - f.suggested, 0)
  if (suggested) {
    parts.push(count(suggested, 'suggestion', 'suggestions'))
    if (open) parts.push(`${open} undecided`)
  }
  const by = new Map<string, number>()
  for (const s of t.settled) by.set(s.driver, (by.get(s.driver) ?? 0) + 1)
  for (const [driver, n] of by) parts.push(`${n} settled by ${driver}`)
  return parts.join(' · ')
}
