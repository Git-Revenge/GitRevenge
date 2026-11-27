// The words of the Repository panel's This clone section (#1005): how a clone differs from a
// whole one, and what each way to the rest fetches.

/** The parts of a clone's shape the section reads (git.CloneShape). */
export interface Shape {
  shallow?: { since: number; commits: number }
  partial?: { remote: string; filter: string }
  sparse?: { cone: boolean; folders: string[]; files: number; total: number }
  oneBranch?: { remote: string; branch: string }
}

/** A whole clone has nothing for the section to show. */
export const isWhole = (s: Shape | null | undefined): boolean => !s || (!s.shallow && !s.partial && !s.sparse && !s.oneBranch)

const DAY = 24 * 60 * 60

/** A day, as "Nov 26, 2025", in UTC so it reads the same everywhere. */
export function day(seconds: number): string {
  return new Date(seconds * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' })
}

const count = (n: number) => n.toLocaleString('en-US')

/** A size in bytes, as "2.4 GB", "310 MB" or "12 KB". */
export function sizeWords(bytes: number): string {
  if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1)} GB`
  if (bytes >= 1 << 20) return `${Math.round(bytes / (1 << 20))} MB`
  return `${Math.max(1, Math.round(bytes / 1024))} KB`
}

/** What a partial clone has only when it's needed, from its filter. */
export function partialWords(filter: string): string {
  const limit = /^blob:limit=(\d+)([kmg]?)$/i.exec(filter)
  if (limit) {
    const unit = { '': ' bytes', k: ' KB', m: ' MB', g: ' GB' }[limit[2].toLowerCase() as '' | 'k' | 'm' | 'g']
    return `Files over ${limit[1]}${unit} when they're needed`
  }
  if (filter.startsWith('tree:')) return "Folders and files when they're needed"
  return "File contents when they're needed"
}

/** A sparse checkout's folders, as "arch/x86 · include · kernel", at most max and then how many more. */
export function folderNames(folders: string[], max = 6): string {
  const shown = folders.slice(0, max).join(' · ')
  return folders.length > max ? `${shown} and ${folders.length - max} more` : shown
}

/** How much more history to fetch, as the menu offers it. */
export interface MoreChoice {
  label: string
  more: { commits: number; since: number; all: boolean }
}

/** The ways to more history: a year before the clone's oldest commit, 1,000 commits more, or
 * all of it. */
export function moreHistory(since: number): MoreChoice[] {
  const yearBefore = since - 365 * DAY
  return [
    { label: `A year more, since ${day(yearBefore)}`, more: { commits: 0, since: yearBefore, all: false } },
    { label: '1,000 commits more', more: { commits: 1000, since: 0, all: false } },
    { label: 'All of it', more: { commits: 0, since: 0, all: true } },
  ]
}

/** The shallow row's lines. */
export function shallowLines(s: { since: number; commits: number }): { what: string; tag: string } {
  return { what: `History since ${day(s.since)}`, tag: `${count(s.commits)} ${s.commits === 1 ? 'commit' : 'commits'} · shallow` }
}

/** The sparse row's lines. */
export function sparseLines(s: { cone: boolean; folders: string[]; files: number; total: number }): { what: string; tag: string } {
  const n = s.folders.length
  const what = s.cone ? `${n === 1 ? '1 folder' : `${count(n)} folders`} checked out` : `${n === 1 ? '1 pattern' : `${count(n)} patterns`} checked out`
  return { what, tag: `${count(s.files)} of ${count(s.total)} files · sparse` }
}

type StorageFigures = { bytes: number; loose: number; packs: number; garbage: number; gcAuto: number; autoPackLimit: number; due: boolean }

/** The Storage row's lines (#937): what the objects take on disk, and how they're kept. */
export function storageLines(s: StorageFigures): { what: string; tag: string } {
  const loose = `${count(s.loose)} loose ${s.loose === 1 ? 'object' : 'objects'}`
  const packs = `${count(s.packs)} ${s.packs === 1 ? 'pack' : 'packs'}`
  return { what: `${s.bytes ? sizeWords(s.bytes) : 'Nothing'} on disk`, tag: `${loose} · ${packs}` }
}

/** Why maintenance is suggested, in git's own terms; "" when git wouldn't tidy up yet. */
export function dueSaid(s: StorageFigures): string {
  if (!s.due) return ''
  if (s.gcAuto > 0 && s.loose > s.gcAuto) return `More than ${count(s.gcAuto)} objects are loose (gc.auto), so git would pack them.`
  return `More than ${count(s.autoPackLimit)} packs (gc.autoPackLimit), so git would join them.`
}

/** Files in the object folder that are neither objects nor packs, said when there are any. */
export function garbageSaid(s: Pick<StorageFigures, 'garbage'>): string {
  return s.garbage ? `${count(s.garbage)} stray ${s.garbage === 1 ? 'file' : 'files'} in the object folder, such as a pack left half written.` : ''
}
