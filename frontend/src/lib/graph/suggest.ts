// What the search box's list of suggestions says and does: the words, with nothing of the
// window in them, so they can be tested alone.
import type { app } from '../../../wailsjs/go/models'

export type Suggestion = app.TrailerSuggestion
export type Suggestions = app.TrailerSuggestions

/** A piece of text, and whether it's the part that was typed. */
export interface Part {
  text: string
  hit: boolean
}

/** Text cut where it contains what was typed, whatever the case, so that part can be marked. */
export function markParts(text: string, match: string): Part[] {
  const want = match.trim().toLowerCase()
  if (!want) return text ? [{ text, hit: false }] : []
  const lower = text.toLowerCase()
  const out: Part[] = []
  let at = 0
  for (;;) {
    const found = lower.indexOf(want, at)
    if (found < 0) break
    if (found > at) out.push({ text: text.slice(at, found), hit: false })
    out.push({ text: text.slice(found, found + want.length), hit: true })
    at = found + want.length
  }
  if (at < text.length) out.push({ text: text.slice(at), hit: false })
  return out
}

/** A person's label as a name and an address: "Sam Ortiz <sam@x>" is "Sam Ortiz" and
 * "sam@x". Anything else is all name. */
export function personParts(label: string): { name: string; email: string } {
  const open = label.lastIndexOf(' <')
  if (open < 0 || !label.endsWith('>')) return { name: label, email: '' }
  return { name: label.slice(0, open), email: label.slice(open + 2, -1) }
}

/** What a suggestion shows as its value: a person by name, anything else as it is. */
export function suggestionName(s: Suggestion): string {
  return s.kind === 'person' ? personParts(s.label).name : s.label
}

/** What Tab writes in the box for a suggestion: the key and a colon, ready for a value, or
 * the key and the value. */
export function completion(s: Suggestion): string {
  return s.value ? `${s.keyLabel}: ${suggestionName(s)}` : `${s.keyLabel}: `
}

/** Whether a suggestion is a key to choose a value of, not a value to filter by. */
export function isKey(s: Suggestion): boolean {
  return !s.value
}

/** How a suggestion is read out: "Reviewed-by: Sam Ortiz, 12 commits". */
export function suggestionLabel(s: Suggestion): string {
  const n = `${s.commits.toLocaleString()} ${s.commits === 1 ? 'commit' : 'commits'}`
  return isKey(s) ? `${s.keyLabel}, a trailer on ${n}` : `${s.keyLabel}: ${suggestionName(s)}, ${n}`
}

/** The next row from one, by a step, in a list of n rows: it stops at either end. */
export function stepRow(at: number, by: number, n: number): number {
  return Math.max(0, Math.min(n - 1, at + by))
}
