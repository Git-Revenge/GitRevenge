import type { app, trailerindex } from '../../../wailsjs/go/models'
import { commitsText, heldWhy, kindWord } from './filters'

// Words and sums for Trailer care: where a repository says how its trailers are read. It
// asks what the app won't decide, lists what was joined without asking, and shows every key
// with where it's offered and why. Nothing here changes a commit.

export type Question = trailerindex.Question
export type CareKey = app.CareKey

/** The slips of spelling that would all be read as one key, asked about together. */
export interface SlipGroup {
  key: string
  keyLabel: string
  commits: number
  slips: Question[]
}

/** The questions sorted for showing: slips grouped by the key they'd be read as, each group
 * where its first slip stood, and the questions about two keys holding the same things. */
export function groupQuestions(questions: Question[]): { slips: SlipGroup[]; same: Question[] } {
  const slips: SlipGroup[] = []
  const same: Question[] = []
  for (const q of questions) {
    if (q.kind !== 'spelling') {
      if (q.kind === 'same') same.push(q)
      continue
    }
    let group = slips.find((g) => g.key === q.key)
    if (!group) {
      group = { key: q.key, keyLabel: q.keyLabel, commits: q.commits, slips: [] }
      slips.push(group)
    }
    group.slips.push(q)
  }
  return { slips, same }
}

/** What two keys holding the same things have in common, after "Fixes and Closes ". */
export function sameText(q: Question): string {
  if (!q.shared) return 'both settle tickets, though no ticket is under both.'
  const shown = q.examples.join(', ')
  const rest = q.shared - q.examples.length
  return `hold the same things: ${shown}${rest > 0 ? ` and ${rest.toLocaleString()} more` : ''}.`
}

/** "232 questions", and with more than are listed, how many show. */
export function openText(open: number, listed: number): string {
  if (open === 0) return 'Nothing needs care.'
  const n = `${open.toLocaleString()} ${open === 1 ? 'question' : 'questions'}`
  return open > listed ? `${n}. The first ${listed.toLocaleString()} are here; answer some and the next ones show.` : n
}

/** How many people are written two ways, as a sentence. */
export function peopleText(n: number): string {
  return n === 1 ? '1 person is written two ways.' : `${n.toLocaleString()} people are written two ways.`
}

/** The words a rule uses for what a key's values are, with how each is said here. */
export const KINDS = [
  { value: 'people', label: 'People' },
  { value: 'tickets', label: 'Tickets' },
  { value: 'commits', label: 'Commits' },
  { value: 'labels', label: 'Labels' },
  { value: 'identifiers', label: 'Identifiers' },
  { value: 'text', label: 'Text' },
] as const

/** The word a rule uses for a kind of value, from the kind as a key carries it. */
export function kindRule(kind: string): string {
  return kindWord(kind)
}

/** Where a key shows, as its column says it. */
export const PLACES = [
  { value: 'rows', label: 'On rows' },
  { value: 'panel', label: 'Panel' },
  { value: 'held', label: 'Held back' },
] as const

export function placeLabel(where: string): string {
  return PLACES.find((p) => p.value === where)?.label ?? 'Panel'
}

/** Why a key shows where it does: a rule's word, or the app's guess and its reason. */
export function whyText(k: CareKey): string {
  if (k.whereSaid) return k.scope === 'global' ? 'Your global git config says so.' : 'This repository’s git config says so.'
  if (k.where === 'held') {
    const why = heldWhy(k.key.held)
    return why ? `A guess: it’s ${why}.` : 'A guess.'
  }
  return `A guess: its values are ${kindWord(k.key.kind)}, on ${commitsText(k.key.commits)}.`
}

/** The other ways a key is written, after its name: spellings joined without asking. */
export function alsoWritten(k: CareKey): string[] {
  return k.key.spellings.map((s) => s.text).filter((text) => text !== k.key.label)
}

export type SortBy = 'key' | 'kind' | 'values' | 'commits' | 'where'

const PLACE_ORDER: Record<string, number> = { rows: 0, panel: 1, held: 2 }

/** The keys in a column's order. Numbers start with the most; words run A to Z; `reverse`
 * turns either round. Keys that tie stay in the order they came, most commits first. */
export function sortKeys(keys: CareKey[], by: SortBy, reverse = false): CareKey[] {
  const text = (a: string, b: string) => a.localeCompare(b, undefined, { sensitivity: 'base' })
  const compare: Record<SortBy, (a: CareKey, b: CareKey) => number> = {
    key: (a, b) => text(a.key.label, b.key.label),
    kind: (a, b) => text(kindWord(a.key.kind), kindWord(b.key.kind)),
    values: (a, b) => b.key.values - a.key.values,
    commits: (a, b) => b.key.commits - a.key.commits,
    where: (a, b) => (PLACE_ORDER[a.where] ?? 1) - (PLACE_ORDER[b.where] ?? 1),
  }
  const sorted = keys.map((k, i) => ({ k, i })).sort((a, b) => compare[by](a.k, b.k) || a.i - b.i)
  if (reverse) sorted.reverse()
  return sorted.map((x) => x.k)
}

/** The keys a filter keeps: by name, by any way it's written, by its kind or by where it shows. */
export function filterKeys(keys: CareKey[], filter: string): CareKey[] {
  const want = filter.trim().toLowerCase()
  if (!want) return keys
  return keys.filter(
    (k) =>
      k.key.label.toLowerCase().includes(want) ||
      k.key.spellings.some((s) => s.text.toLowerCase().includes(want)) ||
      k.also.some((a) => a.toLowerCase().includes(want)) ||
      kindWord(k.key.kind).includes(want) ||
      placeLabel(k.where).toLowerCase().includes(want),
  )
}

/** "12 keys", or with a filter, "3 of 12 keys". */
export function keysText(shown: number, all: number): string {
  const n = `${all.toLocaleString()} ${all === 1 ? 'key' : 'keys'}`
  return shown === all ? n : `${shown.toLocaleString()} of ${n}`
}
