import type { app, git } from '../../../wailsjs/go/models'

// The blame view's track through a file's versions (#1078): one step for each commit that
// changed the file, oldest first, and a last one for changes not committed yet.

/** One step on the track. id is "" for the work tree's. */
export interface Step {
  id: string
  author: string
  time: number
  subject: string
  added: number
  removed: number
  binary: boolean
  /** A merge: its new lines may come from its other parent, so its blame isn't worked out from the version before. */
  merge: boolean
}

/** The track's steps: the versions, then the work tree when it has changes. */
export function trackSteps(track: Pick<app.BlameTrack, 'versions' | 'workTree'>): Step[] {
  const steps: Step[] = (track.versions ?? []).map((v: git.FileVersion) => ({ id: v.id, author: v.author, time: v.time, subject: v.subject, added: v.added, removed: v.removed, binary: !!v.binary, merge: !!v.merge }))
  if (track.workTree) steps.push({ id: '', author: '', time: 0, subject: 'Not committed yet', added: 0, removed: 0, binary: false, merge: false })
  return steps
}

/** How tall a step's mark is, in px: taller the more lines it changed. A big change mustn't
 * dwarf the rest, so it grows slower past a few lines. */
export function markHeight(s: Pick<Step, 'added' | 'removed' | 'binary'>): number {
  const n = s.added + s.removed
  if (s.binary || n === 0) return 6
  return Math.round(6 + Math.min(20, Math.log2(1 + n) * 4))
}

/** Where along the track a step is, from 0 (the oldest) to 1 (the newest). */
export const along = (i: number, count: number) => (count > 1 ? i / (count - 1) : 0)

/** The years under the track: each at its first step. */
export function yearMarks(steps: Pick<Step, 'time'>[]): { label: string; at: number }[] {
  const out: { label: string; at: number }[] = []
  steps.forEach((s, i) => {
    if (!s.time) return
    const label = String(new Date(s.time * 1000).getFullYear())
    if (out.at(-1)?.label !== label) out.push({ label, at: along(i, steps.length) })
  })
  return out
}

/** Each author's place in the lane colours, in the order they first appear. */
export function authorInks(steps: Pick<Step, 'author'>[]): Map<string, number> {
  const inks = new Map<string, number>()
  for (const s of steps) if (s.author && !inks.has(s.author)) inks.set(s.author, inks.size)
  return inks
}

/** What the track says the step is: "Version 17 of 24", the newest, or the work tree. */
export function stepLabel(i: number, steps: Pick<Step, 'id'>[]): string {
  if (steps[i]?.id === '') return 'Not committed yet'
  const versions = steps.filter((s) => s.id !== '').length
  return `Version ${i + 1} of ${versions}${i === versions - 1 ? ', the newest' : ''}`
}

/** What a step changed, "+5 −2", or for the oldest, everything it brought. */
export function churnText(s: Pick<Step, 'added' | 'removed' | 'binary'>, first: boolean): string {
  if (s.binary) return 'binary'
  return first ? `+${s.added}, the first` : `+${s.added} −${s.removed}`
}

/** The lines (from 0) a diff's blocks changed on its new side. */
export function changedLines(blocks: Pick<{ newStart: number; newEnd: number }, 'newStart' | 'newEnd'>[]): Set<number> {
  const out = new Set<number>()
  for (const b of blocks) for (let i = b.newStart; i < b.newEnd; i++) out.add(i)
  return out
}

// Who wrote each line at an older version (#1079), worked out from a version next to it when
// that one's blame is known: git blame of every line took 1.4 s on the Linux kernel's
// mm/slub.c, against 168 to 385 ms for the few lines a step changed.

/** A blamed line, as git.BlameLine has it. */
export type Blamed = Pick<git.BlameLine, 'commit' | 'author' | 'time' | 'summary' | 'line' | 'origLine'> & { edge?: boolean }
/** One place a version changed, old lines [oldStart, oldEnd) for new [newStart, newEnd). */
type Block = { oldStart: number; oldEnd: number; newStart: number; newEnd: number }

/** A version's blame from the one before it: a line it didn't change keeps its blame there,
 * and a line it changed is its own. lines is the version's text. A merge's new lines may
 * come from its other parent, so it never gets here. null if the two don't line up. */
export function blameForward(before: Blamed[], blocks: Block[], lines: string[], step: Pick<Step, 'id' | 'author' | 'time' | 'subject'>): Blamed[] | null {
  const out: Blamed[] = []
  let o = 0
  let n = 0
  const same = (until: number) => {
    for (; n < until; n++, o++) {
      if (o >= before.length) return false
      out.push({ ...before[o], line: lines[n] })
    }
    return true
  }
  for (const b of blocks) {
    if (b.newStart - n !== b.oldStart - o || !same(b.newStart)) return null
    for (; n < b.newEnd; n++) out.push({ commit: step.id, author: step.author, time: step.time, summary: step.subject, line: lines[n], origLine: n + 1 })
    o = b.oldEnd
  }
  if (lines.length - n !== before.length - o || !same(lines.length)) return null
  return out
}

/** A version's blame from the one after it: a line the next version didn't change keeps its
 * blame there; the ones it changed are left null, with the ranges (from 1) to blame them by.
 * count is the version's number of lines, blocks the next version's change. */
export function blameBackward(after: Blamed[], blocks: Block[], count: number): { known: (Blamed | null)[]; ranges: { from: number; to: number }[] } | null {
  const known: (Blamed | null)[] = []
  const ranges: { from: number; to: number }[] = []
  let o = 0
  let n = 0
  const same = (until: number) => {
    for (; o < until; o++, n++) {
      if (n >= after.length) return false
      known.push(after[n])
    }
    return true
  }
  for (const b of blocks) {
    if (b.oldStart - o !== b.newStart - n || !same(b.oldStart)) return null
    if (b.oldEnd > b.oldStart) ranges.push({ from: b.oldStart + 1, to: b.oldEnd })
    for (; o < b.oldEnd; o++) known.push(null)
    n = b.newEnd
  }
  if (count - o !== after.length - n || !same(count)) return null
  return { known, ranges }
}

/** Fills blameBackward's gaps with the lines blamed for its ranges, in order. null if the
 * counts don't match. */
export function fillBlame(known: (Blamed | null)[], blamed: Blamed[]): Blamed[] | null {
  let k = 0
  const out = known.map((l) => l ?? blamed[k++] ?? null)
  return k === blamed.length && out.every((l) => l !== null) ? (out as Blamed[]) : null
}

/** A text's lines, without the empty one after its last newline. */
export function textLines(text: string): string[] {
  const lines = text.split('\n')
  if (lines.at(-1) === '') lines.pop()
  return lines.map((l) => l.replace(/\r$/, ''))
}
