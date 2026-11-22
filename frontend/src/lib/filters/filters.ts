import type { trailerindex } from '../../../wailsjs/go/models'

// Words for the sidebar's Smart filters panel. A filter is a reading of what's written on
// commits already: the name is "Smart filters", never "tags", which means git tags.

type Key = Pick<trailerindex.Key, 'id' | 'label' | 'kind'>
type Value = Pick<trailerindex.Value, 'label' | 'kind'>

/** What a key's values are, in a word, for beside its name. */
export function kindWord(kind: string): string {
  switch (kind) {
    case 'person':
      return 'people'
    case 'ticket':
      return 'tickets'
    case 'commit':
      return 'commits'
    case 'label':
      return 'labels'
    case 'identifier':
      return 'identifiers'
    default:
      return 'text'
  }
}

/** Why a key isn't offered as a filter, as the end of "Held back: …". */
export function heldWhy(held: string): string {
  switch (held) {
    case 'identifier':
      return 'a different value on nearly every commit'
    case 'everywhere':
      return 'on nearly every commit'
    case 'rare':
      return 'on only a commit or two'
    default:
      return ''
  }
}

/** A value's name in the list: a person without their address, anything else as it is. */
export function valueName(v: Value): string {
  if (v.kind !== 'person') return v.label
  const open = v.label.lastIndexOf('<')
  return open > 0 ? v.label.slice(0, open).trim() : v.label
}

/**
 * The search that finds the commits with a value: "Reviewed-by: Sam Ortiz". Picking a
 * value in the panel runs it, and the value is the picked one while it's what the graph is
 * searching for.
 */
export function queryFor(key: Key, value?: Value): string {
  return value ? `${key.label}: ${valueName(value)}` : `${key.label}:`
}

/** Whether a search is the one a value's pick runs, whatever the letter case. */
export function isPicked(query: string, key: Key, value: Value): boolean {
  return query.trim().toLowerCase() === queryFor(key, value).toLowerCase()
}

/** "24 more…" */
export function moreText(n: number): string {
  return `${n.toLocaleString()} more…`
}

/** "3 commits", "1 commit". */
export function commitsText(n: number): string {
  return `${n.toLocaleString()} ${n === 1 ? 'commit' : 'commits'}`
}

/**
 * What to say while the history is still being read: counts are of what's read so far.
 * Empty once it has all been read.
 */
export function soFar(s: Pick<trailerindex.Status, 'done' | 'reading' | 'read' | 'total'>): string {
  if (s.done) return ''
  if (s.reading && s.total > 0) return `Counts so far: ${s.read.toLocaleString()} of ${s.total.toLocaleString()} commits read.`
  return 'Counts so far: the history is still being read.'
}

/** "4 keys held back", "1 key held back". */
export function heldText(n: number): string {
  return `${n.toLocaleString()} ${n === 1 ? 'key' : 'keys'} held back`
}

/** The key a folder's open state is saved under, apart from the branch folders'. */
export const folderKey = (id: string) => `filter:${id}`

/** How many folders start open: the keys used most. A choice made since is kept. */
export const OPEN_AT_FIRST = 4

/** Whether a key's folder is open: as chosen, else the first few are. */
export function isOpen(open: Record<string, boolean>, id: string, index: number): boolean {
  return open[folderKey(id)] ?? index < OPEN_AT_FIRST
}
