import type { app, git } from '../../wailsjs/go/models'

// The words of the plain diff's preview (#1024), kept apart from the dialog so they can be
// tested as plain functions.

export type DiffMode = 'files' | 'index' | 'cached'

/** Where the diff's changes go, as the choices say them. */
export const MODES: { mode: DiffMode; label: string; note: string }[] = [
  { mode: 'files', label: 'The files', note: 'as git apply does' },
  { mode: 'index', label: 'The files, and stage them', note: '--index' },
  { mode: 'cached', label: 'Staged only', note: 'the files stay as they are' },
]

export type Tone = 'ok' | 'warn' | 'mute'

type File = Pick<git.DiffFile, 'path' | 'oldPath' | 'created' | 'removed'>
type Try = Pick<app.DiffTry, 'status'>
type Applied = Pick<app.AppliedDiff, 'files' | 'conflicts'>

/** A file's row: what its try said, in words, and whether it can be ticked. A file that
 * doesn't fit, or whose change is in already, can't: git apply would refuse the whole diff. */
export function rowStatus(f: File, r: Try | undefined, reverse: boolean, unclean: boolean): { text: string; tone: Tone; can: boolean } {
  if (unclean) return { text: 'has your changes', tone: 'warn', can: true }
  if (!r) return { text: 'trying…', tone: 'mute', can: true }
  switch (r.status) {
    case 'applies':
      return { text: what(f, reverse), tone: 'ok', can: true }
    case 'merges':
      return { text: 'applies by merging', tone: 'ok', can: true }
    case 'conflicts':
      return { text: 'merges with a conflict', tone: 'warn', can: true }
    case 'applied':
      return { text: reverse ? 'not here to take out' : 'in already', tone: 'mute', can: false }
  }
  return { text: 'doesn’t fit', tone: 'warn', can: false }
}

/** What applying does to a file that fits: made, deleted, renamed or changed. In reverse,
 * each the other way round. */
function what(f: File, reverse: boolean): string {
  if (f.created) return reverse ? 'deleted' : 'a new file'
  if (f.removed) return reverse ? 'comes back' : 'deleted'
  if (f.oldPath) return reverse ? `renamed back to ${f.oldPath}` : `renamed from ${f.oldPath}`
  return 'applies'
}

/** What a picked file says about the diff as a whole that stops Apply, or "". */
export function blockedBy(picked: string[], unclean: string[], tries: Record<string, Try>, mode: DiffMode): string {
  const mine = picked.filter((p) => unclean.includes(p))
  if (mine.length) return `${listed(mine)} ${mine.length === 1 ? 'has' : 'have'} changes of yours. Commit or stash them first, so the diff can be undone cleanly, or leave ${mine.length === 1 ? 'it' : 'them'} out.`
  const clash = picked.filter((p) => tries[p]?.status === 'conflicts')
  if (mode === 'cached' && clash.length) return `${listed(clash)} would merge with a conflict, which staging only can’t hold. Apply to the files, or leave ${clash.length === 1 ? 'it' : 'them'} out.`
  return ''
}

/** Files named: "a.js", "a.js and b.js", "a.js and 3 others". */
export function listed(files: string[]): string {
  if (files.length <= 2) return files.join(' and ')
  return `${files[0]} and ${files.length - 1} others`
}

const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

/** The Apply button's words. */
export function applyLabel(n: number, reverse: boolean): string {
  return reverse ? `Take out of ${count(n, 'file', 'files')}` : `Apply to ${count(n, 'file', 'files')}`
}

/** What to say once it's applied. */
export function appliedMessage(done: Applied, reverse: boolean): string {
  const files = count(done.files, 'file', 'files')
  const said = reverse ? `Took the diff out of ${files}` : `Applied the diff to ${files}`
  const n = done.conflicts.length
  if (!n) return `${said}.`
  return `${said}; ${n === 1 ? `${done.conflicts[0]} has a conflict` : `${n} have conflicts`} to resolve in Changes.`
}

/** The toast's way back: Undo, or after a conflict, Put these files back. */
export function undoLabel(done: Applied): string {
  return done.conflicts.length ? 'Put these files back' : 'Undo'
}

/** The lead line: the file, and what's in it. */
export function diffSummary(info: { files: Pick<git.DiffFile, 'added' | 'deleted'>[] }): string {
  const plus = info.files.reduce((a, f) => a + f.added, 0)
  const minus = info.files.reduce((a, f) => a + f.deleted, 0)
  return `a plain diff · ${count(info.files.length, 'file', 'files')} · +${plus} −${minus}`
}
