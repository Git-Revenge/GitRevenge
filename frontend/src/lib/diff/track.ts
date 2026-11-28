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
}

/** The track's steps: the versions, then the work tree when it has changes. */
export function trackSteps(track: Pick<app.BlameTrack, 'versions' | 'workTree'>): Step[] {
  const steps: Step[] = (track.versions ?? []).map((v: git.FileVersion) => ({ id: v.id, author: v.author, time: v.time, subject: v.subject, added: v.added, removed: v.removed, binary: !!v.binary }))
  if (track.workTree) steps.push({ id: '', author: '', time: 0, subject: 'Not committed yet', added: 0, removed: 0, binary: false })
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

/** A text's lines, without the empty one after its last newline. */
export function textLines(text: string): string[] {
  const lines = text.split('\n')
  if (lines.at(-1) === '') lines.pop()
  return lines.map((l) => l.replace(/\r$/, ''))
}
