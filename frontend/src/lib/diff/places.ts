import { parseConflicts } from './markers'

// A conflicted file as the conflict tool works on it (issue #597): the text every side agrees
// on, with a "place" wherever they don't. The result the user edits never holds conflict
// markers: an undecided place is simply empty until a side is taken or something is typed.

/** One conflicted place: what each side has there. base is null when it isn't known. */
export interface Place {
  ours: string
  base: string | null
  theirs: string
}

export type Side = 'ours' | 'base' | 'theirs'
/** What has been decided for a place. 'suggested' holds the sides merged by token, waiting to
 * be kept, which counts as undecided; 'combined' is that suggestion, kept. */
export type Status = 'undecided' | 'suggested' | 'ours' | 'theirs' | 'both' | 'combined' | 'edited'

/** A place's sides merged by token (internal/semdiff): text to suggest when ok, and either way a
 * line saying what each side did or why they can't be combined. */
export interface Suggestion {
  ok: boolean
  text: string
  reason: string
  origins: { from: number; to: number; side: string }[]
  /** What each side changed against the original, in its own text. */
  ours: { from: number; to: number }[]
  theirs: { from: number; to: number }[]
}

/** A place in the result: where it is and what was decided. from and to are offsets in the
 * result's text; an undecided place is empty, so they're equal. */
export interface PlaceState {
  status: Status
  from: number
  to: number
}

/** A file split into the text all sides share and the places they don't. */
export interface Parsed {
  /** Shared text and places in order: text[0], places[0], text[1], … text[places.length]. */
  text: string[]
  places: Place[]
}

/** The merged text to work from: the re-merge that keeps each place's original when it's the
 * same merge as the file on disk, and otherwise the file on disk as it is.
 *
 * The two can split places differently: git's default conflict style joins places a line or
 * two apart, and diff3's never does (#974). So they're compared by what each side reads: taking
 * every place's ours, and then every place's theirs, gives the same text from both unless
 * something in the file was resolved or edited. internal/app's placesOf does the same. */
export function sourceOf(file: { merged: { text: string }; diff3?: string }): string {
  const merged = file.merged.text
  const diff3 = file.diff3 ?? ''
  if (!diff3) return merged
  const same = taking(merged, 'ours') === taking(diff3, 'ours') && taking(merged, 'theirs') === taking(diff3, 'theirs')
  return same ? diff3 : merged
}

/** text with every conflicted place replaced by one side's version of it. */
function taking(text: string, side: 'ours' | 'theirs'): string {
  let out = ''
  let at = 0
  for (const r of parseConflicts(text)) {
    out += text.slice(at, r.from) + r[side]
    at = r.to
  }
  return out + text.slice(at)
}

/** Splits merged text at its conflict markers. */
export function parsePlaces(merged: string): Parsed {
  const text: string[] = []
  const places: Place[] = []
  let at = 0
  for (const r of parseConflicts(merged)) {
    text.push(merged.slice(at, r.from))
    places.push({ ours: r.ours, base: r.base, theirs: r.theirs })
    at = r.to
  }
  text.push(merged.slice(at))
  return { text, places }
}

/** The text a place holds once decided. "both" keeps ours, then theirs; a suggestion, kept or
 * not, is its merged text; "edited" starts from both, for writing it by hand. */
export function textFor(place: Place, status: Status, suggestion?: Suggestion | null): string {
  if (status === 'ours') return place.ours
  if (status === 'theirs') return place.theirs
  if (status === 'both' || status === 'edited') return place.ours + place.theirs
  if ((status === 'suggested' || status === 'combined') && suggestion?.ok) return suggestion.text
  return ''
}

/** How a place starts: with its suggestion when the sides could be combined, else undecided. */
export const startStatus = (suggestion?: Suggestion | null): Status => (suggestion?.ok ? 'suggested' : 'undecided')

/** The result to start from, with every place suggested or undecided (and so empty), and where
 * each is. */
export function startingResult(parsed: Parsed, suggestions: (Suggestion | null)[] = []): { doc: string; places: PlaceState[] } {
  let doc = ''
  const places: PlaceState[] = []
  parsed.text.forEach((t, i) => {
    doc += t
    if (i >= parsed.places.length) return
    const s = suggestions[i]
    const status = startStatus(s)
    const text = textFor(parsed.places[i], status, s)
    places.push({ status, from: doc.length, to: doc.length + text.length })
    doc += text
  })
  return { doc, places }
}

/** The places after one of them takes new text: later ones move along. */
export function withChoice(places: PlaceState[], index: number, status: Status, text: string): PlaceState[] {
  const p = places[index]
  const delta = text.length - (p.to - p.from)
  return places.map((q, i) => (i === index ? { status, from: p.from, to: p.from + text.length } : i > index ? { ...q, from: q.from + delta, to: q.to + delta } : q))
}

/** Whether a change to the result between from and to (in the old text) lands inside a place,
 * which makes that place edited by hand. A change that only touches its edge from outside
 * doesn't count, except that typing at an empty place fills it. */
export function touches(p: PlaceState, from: number, to: number): boolean {
  if (p.from === p.to) return from <= p.from && to >= p.from
  if (from === to) return from >= p.from && from < p.to
  return from < p.to && to > p.from
}

export const undecided = (places: PlaceState[]) => places.filter((p) => p.status === 'undecided').length
/** Places holding a suggestion not yet kept. They count as undecided too. */
export const suggested = (places: PlaceState[]) => places.filter((p) => p.status === 'suggested').length
const open = (p: PlaceState) => p.status === 'undecided' || p.status === 'suggested'

/** The next place after (or before) the current one, preferring those still to decide,
 * suggestions included. It wraps round. -1 when there are no places. */
export function nextPlace(places: PlaceState[], current: number, by: 1 | -1): number {
  const n = places.length
  if (!n) return -1
  const any = places.some(open)
  for (let step = 1; step <= n; step++) {
    const i = (((current + by * step) % n) + n) % n
    if (!any || open(places[i])) return i
  }
  return current
}

/** What a place says about itself in its heading. */
export function placeLabel(index: number, status: Status): string {
  const what = {
    undecided: 'choose a side, or type here',
    suggested: 'suggested: both sides combined',
    ours: 'took ours',
    theirs: 'took theirs',
    both: 'took both, ours first',
    combined: 'kept the suggestion',
    edited: "you're writing this one by hand",
  }[status]
  return `Place ${index + 1} · ${what}`
}

const lines = (s: string) => (s === '' ? [] : s.replace(/\n$/, '').split('\n'))

/** One side of a place with a few lines around it, for the read-only panes above the result:
 * the lines, which of them are the place's own, and the first line's number on that side. */
export function sideView(parsed: Parsed, index: number, side: Side, context = 3): { lines: string[]; from: number; to: number; first: number } {
  const of = (p: Place) => (side === 'ours' ? p.ours : side === 'theirs' ? p.theirs : (p.base ?? ''))
  // The file as this side would have it: the shared text with this side's version of each place.
  let before = ''
  for (let i = 0; i < index; i++) before += parsed.text[i] + of(parsed.places[i])
  before += parsed.text[index]
  const head = lines(before)
  const own = lines(of(parsed.places[index]))
  const tail = lines(parsed.text[index + 1] ?? '')
  const lead = head.slice(Math.max(0, head.length - context))
  return { lines: [...lead, ...own, ...tail.slice(0, context)], from: lead.length, to: lead.length + own.length, first: head.length - lead.length + 1 }
}

/** Where a place's lines are in a side's real file (1-based, inclusive), for asking who last
 * changed them. The real file can differ from the shared text around it, so the lines are
 * found by what they say, nearest to where they're expected. null when the side has no lines
 * there or they can't be found. */
export function placeLines(sideText: string, parsed: Parsed, index: number, side: Side): { start: number; end: number } | null {
  const of = (p: Place) => (side === 'ours' ? p.ours : side === 'theirs' ? p.theirs : (p.base ?? ''))
  const own = lines(of(parsed.places[index]))
  if (!own.length) return null
  let before = ''
  for (let i = 0; i < index; i++) before += parsed.text[i] + of(parsed.places[i])
  const expected = lines(before + parsed.text[index]).length
  const all = lines(sideText)
  let best = -1
  for (let at = 0; at + own.length <= all.length; at++) {
    if (own.every((l, k) => all[at + k] === l) && (best < 0 || Math.abs(at - expected) < Math.abs(best - expected))) best = at
  }
  return best < 0 ? null : { start: best + 1, end: best + own.length }
}

/** A side's changed stretches of a place (offsets in the place's text) as [from, to) marks on
 * each of its lines. */
export function lineMarks(text: string, stretches: { from: number; to: number }[]): [number, number][][] {
  const out: [number, number][][] = []
  let start = 0
  for (const line of lines(text)) {
    const end = start + line.length
    out.push(stretches.filter((s) => s.from < end && s.to > start).map((s) => [Math.max(s.from, start) - start, Math.min(s.to, end) - start] as [number, number]))
    start = end + 1
  }
  return out
}

/** A highlighted line's tokens split where marks start and end, with the marked pieces
 * flagged. */
export function markTokens<T extends { text: string }>(tokens: T[], marks: [number, number][]): (T & { mark: boolean })[] {
  if (!marks.length) return tokens.map((t) => ({ ...t, mark: false }))
  const out: (T & { mark: boolean })[] = []
  let at = 0
  for (const t of tokens) {
    const cuts = new Set([0, t.text.length])
    for (const [a, b] of marks) for (const c of [a - at, b - at]) if (c > 0 && c < t.text.length) cuts.add(c)
    const sorted = [...cuts].sort((x, y) => x - y)
    for (let k = 0; k + 1 < sorted.length; k++) {
      const from = at + sorted[k]
      out.push({ ...t, text: t.text.slice(sorted[k], sorted[k + 1]), mark: marks.some(([a, b]) => from >= a && from < b) })
    }
    at += t.text.length
  }
  return out
}
