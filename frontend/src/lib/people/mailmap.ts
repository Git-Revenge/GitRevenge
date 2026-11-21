import type { app, git } from '../../../wailsjs/go/models'
import { splitLines } from '../ignorerules/rules'

/** The work tree's mailmap: the one file the app writes. */
export const WORK_TREE = '.mailmap'

/** What a mailmap source is called on screen. */
export function sourceLabel(source: string): string {
  return source === WORK_TREE ? '.mailmap' : source === 'mailmap.file' ? 'Your mailmap file' : source === 'mailmap.blob' ? 'A mailmap in the repository' : source
}

/** One line under a source's name, saying whose it is. */
export function sourceNote(s: Pick<git.MailmapSource, 'source' | 'exists'>): string {
  if (s.source === WORK_TREE) return s.exists ? 'shared · committed with the repository' : 'none in this repository'
  if (s.source === 'mailmap.file') return 'from mailmap.file in your git config · read only here'
  return 'from mailmap.blob in your git config · read only here'
}

const count = (n: number, one: string, many = `${one}s`) => `${n.toLocaleString()} ${n === 1 ? one : many}`

/** How much someone wrote: "71 commits", "2 commits, co-author on 6", "co-author on 4". */
export function wrote(commits: number, coAuthored: number): string {
  if (commits && coAuthored) return `${count(commits, 'commit')}, co-author on ${coAuthored.toLocaleString()}`
  if (coAuthored) return `co-author on ${coAuthored.toLocaleString()}`
  return count(commits, 'commit')
}

/** The identities joined to a person, leaving out the one they're shown as. */
export function joined(p: app.Person): app.PersonIdentity[] {
  return p.identities.filter((id) => id.name !== p.name || id.email !== p.email)
}

/** Where a name comes from: "line 2", "lines 2 and 5", "line 1 of your mailmap file". */
export function linesText(lines: git.MailmapRef[]): string {
  if (!lines.length) return ''
  const bySource = new Map<string, number[]>()
  for (const l of lines) bySource.set(l.source, [...(bySource.get(l.source) ?? []), l.line])
  const parts: string[] = []
  for (const [source, ns] of bySource) {
    const where = ns.length === 1 ? `line ${ns[0]}` : `lines ${ns.slice(0, -1).join(', ')} and ${ns[ns.length - 1]}`
    parts.push(source === WORK_TREE ? where : `${where} of ${sourceLabel(source).toLowerCase()}`)
  }
  return parts.join(', ')
}

/**
 * The lines that read a commit as someone else, with the file each is in: "line 6 of
 * .mailmap", "lines 2 and 5 of .mailmap", "line 6 of .mailmap and line 1 of your mailmap
 * file". For where the People view's own text isn't beside it to say which file.
 */
export function linesIn(lines: git.MailmapRef[]): string {
  const bySource = new Map<string, number[]>()
  for (const l of lines) bySource.set(l.source, [...(bySource.get(l.source) ?? []), l.line])
  const parts: string[] = []
  for (const [source, ns] of bySource) {
    const where = ns.length === 1 ? `line ${ns[0]}` : `lines ${ns.slice(0, -1).join(', ')} and ${ns[ns.length - 1]}`
    parts.push(`${where} of ${source === WORK_TREE ? WORK_TREE : sourceLabel(source).toLowerCase()}`)
  }
  return parts.join(' and ')
}

/** A colour for someone's initials, the same each time for the same name. */
export function faceColor(name: string): string {
  let h = 0
  for (const c of name) h = (h * 31 + c.codePointAt(0)!) >>> 0
  return `var(--lane-${h % 6})`
}

/** Whether an identity can be taken apart here: only lines of the .mailmap can be removed. */
export function canTakeApart(id: app.PersonIdentity): boolean {
  return id.lines.length > 0 && id.lines.every((l) => l.source === WORK_TREE)
}

export type NoteTone = '' | 'joins' | 'idle' | 'bad'

/** What a line does, in a few words, and how to show it. */
export function describeNote(n: git.MailmapNote): { text: string; tone: NoteTone } {
  if (n.kind === 'skipped') return { text: `git skips this: ${n.why ?? 'it has no address'}`, tone: 'bad' }
  if (n.kind !== 'map') return { text: '', tone: '' }
  if (n.replaced) return { text: 'a later line replaces this', tone: 'idle' }
  if (!n.identities) return { text: 'matches nobody now', tone: 'idle' }
  const who = n.identities > 1 ? `${count(n.identities, 'name')} · ` : ''
  if (!n.commits) return { text: `joins ${who}${count(n.coAuthored, 'co-author credit')}`, tone: 'joins' }
  return { text: `joins ${who}${count(n.commits, 'commit')}`, tone: 'joins' }
}

/** How many lines differ between two texts, for "2 lines not saved". */
export function changedLines(base: string, text: string): number {
  if (base === text) return 0
  const a = splitLines(base)
  const b = splitLines(text)
  let n = Math.abs(a.length - b.length)
  for (let i = 0; i < Math.min(a.length, b.length); i++) if (a[i] !== b[i]) n++
  return Math.max(n, 1)
}

/** The summary over the list: "7 names and 1 bot on 212 commits". */
export function summary(l: Pick<app.PeopleList, 'everyone' | 'bots' | 'commits'>): string {
  const people = l.everyone - l.bots
  const who = l.bots ? `${count(people, 'name')} and ${count(l.bots, 'bot')}` : count(people, 'name')
  return `${who} on ${count(l.commits, 'commit')}`
}
