import type { Ref } from './rowCache'

/** Where the local branches and HEAD were at one reading of the refs. */
export interface Tips {
  /** The checked-out branch, or "" when HEAD is detached. */
  head: string
  /** HEAD's commit, or "" before the first commit. */
  headCommit: string
  /** How far the checked-out branch is behind its upstream. */
  behind: number
  /** Each local branch's commit, by name. */
  branches: Record<string, string>
}

export function tipsOf(refs: readonly Pick<Ref, 'kind' | 'short' | 'commit' | 'head' | 'behind'>[], head: { branch: string; commit: string }): Tips {
  const branches: Record<string, string> = {}
  let behind = 0
  for (const r of refs) {
    if (r.kind !== 'local') continue
    branches[r.short] = r.commit
    if (r.head) behind = r.behind
  }
  return { head: head.branch, headCommit: head.commit, behind, branches }
}

/** A branch's tip somewhere new. */
export interface Move {
  /** The branch that moved, or "" for a detached HEAD. */
  branch: string
  /** Where it was; "" when it didn't move so much as get checked out, so nothing arrived. */
  from: string
  to: string
}

/**
 * What moved between two readings, and so where the view should go. The checked-out commit
 * comes first: a pull, merge, rebase, reset, commit or checkout all leave it somewhere new.
 * Otherwise one other local branch moving (a pull into a branch that isn't checked out) is
 * followed. Several moving at once has no one place to go, and remote branches moving (a fetch)
 * never count.
 */
export function branchMove(before: Tips | null, after: Tips): Move | null {
  if (!before) return null
  if (after.headCommit && (after.head !== before.head || after.headCommit !== before.headCommit)) {
    const sameBranch = after.head === before.head
    return { branch: after.head, from: sameBranch ? before.headCommit : '', to: after.headCommit }
  }
  const moved = Object.keys(after.branches).filter((name) => name in before.branches && before.branches[name] !== after.branches[name])
  if (moved.length !== 1) return null
  const [name] = moved
  return { branch: name, from: before.branches[name], to: after.branches[name] }
}

/** How far down the view a branch's new tip lands: a third, so the rows above it stay as
 * context and the commits it brought are below it. */
export const LANDING = 1 / 3

/** Which way the checked-out commit is from the view: null when any of its row is showing. */
export function headAway(rowY: number, rowH: number, top: number, viewH: number): 'up' | 'down' | null {
  if (rowY + rowH <= top) return 'up'
  if (rowY >= top + viewH) return 'down'
  return null
}

/** What the checked-out commit is called: its branch, or its short SHA when detached. */
export function headLabel(branch: string, commit: string): string {
  return branch || commit.slice(0, 7)
}

/** What a screen reader hears when the view follows a branch. */
export function moveAnnouncement(move: Move, arrived: number): string {
  const what = `${headLabel(move.branch, move.to)} ${move.from ? 'moved to' : 'is at'} ${move.to.slice(0, 7)}`
  if (!arrived) return what
  return `${what}, ${arrived.toLocaleString()} new ${arrived === 1 ? 'commit' : 'commits'}`
}

/** How long after the app ran something that moves a branch a move still counts as its own.
 * Reloading a large history after a pull takes a while. */
export const OWN_MOVE_MS = 15_000

/** Whether a move seen now was the app's own doing, given how many milliseconds ago it last ran
 * something that moves a branch (negative, or nothing, when it hasn't). */
export function isOwnMove(ageMs: number | null | undefined): boolean {
  return typeof ageMs === 'number' && ageMs >= 0 && ageMs < OWN_MOVE_MS
}
