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
 * A key and value as a search would write them: "Reviewed-by: Sam Ortiz", or "Changelog:"
 * for a whole key. A key held back from the list is still searched for this way.
 */
export function queryFor(key: Key, value?: Value): string {
  return value ? `${key.label}: ${valueName(value)}` : `${key.label}:`
}

/** A key's word endings that are one key (endings in Go's trailerindex). */
const ENDINGS: Record<string, string> = { close: 'closes', closed: 'closes', fix: 'fixes', fixed: 'fixes', resolve: 'resolves', resolved: 'resolves', ref: 'refs', reference: 'refs', references: 'refs' }

/** The ID of the key a spelling belongs to, as the trailer index joins keys: whatever the
 * letter case, and with "Closed" and "Close" as "Closes" (KeyID in Go). */
export function keyId(spelling: string): string {
  const lower = spelling.trim().toLowerCase()
  return ENDINGS[lower] ?? lower
}

/** What the pin beside a key says it does. */
export function pinLabel(key: string, pinned: boolean): string {
  return pinned ? `Unpin ${key} from the graph’s rows` : `Pin ${key} to the graph’s rows`
}

type OfferStore = Pick<Storage, 'getItem' | 'setItem'>
const offerKey = (path: string) => `gitrevenge.pinOffer:${path}`

/** The key the panel offers to pin, once for a repository: its most used label, while nothing
 * is pinned and the offer hasn't been answered. Row space is scarce, so it's only an offer. */
export function pinOffer<K extends { id: string; kind: string; held: string }>(keys: K[], pinned: string[], path: string, store: OfferStore): K | undefined {
  if (pinned.length) return undefined
  try {
    if (store.getItem(offerKey(path))) return undefined
  } catch {
    return undefined
  }
  return keys.find((k) => k.kind === 'label' && !k.held)
}

/** Remembers that the offer to pin was answered, either way, for a repository. */
export function answerPinOffer(path: string, store: OfferStore): void {
  try {
    store.setItem(offerKey(path), '1')
  } catch {
    // It's offered again next time, which is no harm.
  }
}

/** Whether a value is one of the filter's picks. */
export function hasPick(picks: { key: string; value: string }[], key: string, value: string): boolean {
  return picks.some((p) => p.key === key && p.value === value)
}

/** The picks with one value added, or taken out if it was there: what a click on it asks for. */
export function togglePick(picks: { key: string; value: string }[], key: string, value: string): { key: string; value: string }[] {
  const rest = picks.filter((p) => !(p.key === key && p.value === value))
  const out = rest.length === picks.length ? [...picks, { key, value }] : rest
  return out.map((p) => ({ key: p.key, value: p.value }))
}

/** A picked value on its chip: a person by name, anything else as it is. */
export function chipText(p: { valueLabel: string; kind: string }): string {
  return valueName({ label: p.valueLabel, kind: p.kind })
}

/** "11 of 212 commits", or "11 commits" until the history has been counted. */
export function keptText(kept: number, total: number): string {
  if (total > 0) return `${kept.toLocaleString()} of ${total.toLocaleString()} commits`
  return commitsText(kept)
}

type ModeStore = Pick<Storage, 'getItem' | 'setItem'>
const modeKey = (path: string) => `gitrevenge.filterMode:${path}`

/** Whether a repository's filter shows only its commits: the default, unless Highlight was
 * the last mode chosen for it on this computer. */
export function loadOnly(path: string, store: ModeStore): boolean {
  try {
    return store.getItem(modeKey(path)) !== 'highlight'
  } catch {
    return true
  }
}

/** Remembers the mode for a repository. It's a convenience, so a blocked store is ignored. */
export function saveOnly(path: string, only: boolean, store: ModeStore): void {
  try {
    store.setItem(modeKey(path), only ? 'only' : 'highlight')
  } catch {
    // The filter still works; it just starts as Only these next time.
  }
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
