import type { git } from '../../wailsjs/go/models'
import { dayText, sinceOfDay } from './filters/history'

// The clone dialog's Options (#1002): what each choice sends to git clone, and the line that
// says what's chosen while they're folded. Every default is what plain `git clone URL` does.

/** The dialog's choices, as its fields hold them. */
export interface ClonePicks {
  history: 'all' | 'depth' | 'since'
  /** How many commits, for 'depth', as typed. */
  depth: string
  /** A day ("2025-11-26"), for 'since'. */
  since: string
  contents: 'all' | 'partial'
  branches: 'all' | 'one'
  branch: string
  submodules: boolean
  /** Every folder, or only some, chosen in the dialog's second step (#1004). */
  folders: 'all' | 'some'
}

/** A plain `git clone URL`. */
export function defaultPicks(since = ''): ClonePicks {
  return { history: 'all', depth: '1000', since, contents: 'all', branches: 'all', branch: '', submodules: false, folders: 'all' }
}

/** The number of commits typed, or 0 when it isn't a whole number from 1 up. */
export function depthOf(typed: string): number {
  const t = typed.trim().replace(/[,_\s]/g, '')
  if (!/^\d+$/.test(t)) return 0
  const n = Number(t)
  return Number.isSafeInteger(n) && n > 0 ? n : 0
}

/** What's wrong with the choices, in a sentence, or '' when they can be sent. */
export function picksProblem(p: ClonePicks, now = new Date()): string {
  if (p.history === 'depth' && !depthOf(p.depth)) return 'The number of commits has to be a whole number, 1 or more.'
  if (p.history === 'since') {
    const s = sinceOfDay(p.since)
    if (!s) return 'Choose the day the history starts from.'
    if (s * 1000 > now.getTime()) return 'The day the history starts from is in the future.'
  }
  if (p.branches === 'one' && !p.branch.trim()) return 'Name the branch to clone.'
  return ''
}

/** The options git clone is given (git.CloneOptions). */
export function cloneOptions(p: ClonePicks): git.CloneOptions {
  return {
    depth: p.history === 'depth' ? depthOf(p.depth) : 0,
    since: p.history === 'since' ? sinceOfDay(p.since) : 0,
    partial: p.contents === 'partial',
    branch: p.branches === 'one' ? p.branch.trim() : '',
    submodules: p.submodules,
    folders: p.folders === 'some',
  } as git.CloneOptions
}

/** What's chosen, for the line beside Options while they're folded: "as git clone does", or
 * "the last 1,000 commits · contents when needed". */
export function cloneSummary(p: ClonePicks): string {
  const parts: string[] = []
  if (p.history === 'depth') {
    // With every branch, it's that many of each: git's --depth counts back from every tip.
    const each = p.branches === 'all' ? ' of each branch' : ''
    const n = depthOf(p.depth)
    parts.push((n === 1 ? 'the last commit' : n ? `the last ${n.toLocaleString()} commits` : 'some commits') + each)
  } else if (p.history === 'since') {
    const s = sinceOfDay(p.since)
    parts.push(s ? `since ${dayText(s)}` : 'since a day')
  }
  if (p.contents === 'partial') parts.push('contents when needed')
  if (p.branches === 'one') parts.push(p.branch.trim() ? `only ${p.branch.trim()}` : 'one branch')
  if (p.submodules) parts.push('with submodules')
  if (p.folders === 'some') parts.push('some folders')
  return parts.length ? parts.join(' · ') : 'as git clone does'
}

// The second step's folders (#1004), from the clone's tree: how many files each holds, and
// what a checkout of some of them writes, in cone mode.

/** A folder and its files (git.FolderCount): all of them, and those directly in it. "" is the
 * top level. */
export interface Folder {
  path: string
  files: number
  direct: number
}

/** The folder a path is in: "" for one at the top. */
export const parentOf = (p: string) => (p.includes('/') ? p.slice(0, p.lastIndexOf('/')) : '')

/** A path's last part. */
export const nameOf = (p: string) => p.slice(p.lastIndexOf('/') + 1)

/** The folders directly in parent ("" for the top level), in path order. */
export function childrenOf(counts: Folder[], parent: string): Folder[] {
  return counts.filter((c) => c.path !== '' && parentOf(c.path) === parent)
}

/** The chosen folders without any that's inside another chosen one, which brings it anyway. */
export function outermost(chosen: string[]): string[] {
  const set = [...new Set(chosen)]
  return set.filter((p) => !set.some((q) => q !== p && p.startsWith(q + '/'))).sort()
}

/** Whether path is a chosen folder or inside one. */
export const within = (path: string, chosen: string[]) => chosen.some((q) => path === q || path.startsWith(q + '/'))

/**
 * How many files a checkout of the chosen folders writes, by cone mode's rule: the top
 * level's own files, every file in a chosen folder, and the files directly in each folder
 * above one ("arch/x86" brings the 2 files directly in "arch").
 */
export function filesWritten(counts: Folder[], chosen: string[]): number {
  const by = new Map(counts.map((c) => [c.path, c]))
  const picked = outermost(chosen)
  let n = by.get('')?.direct ?? 0
  const above = new Set<string>()
  for (const p of picked) {
    n += by.get(p)?.files ?? 0
    for (let d = parentOf(p); d !== ''; d = parentOf(d)) above.add(d)
  }
  for (const d of above) n += by.get(d)?.direct ?? 0
  return n
}
