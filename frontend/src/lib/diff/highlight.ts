import { LanguageDescription, type LanguageSupport } from '@codemirror/language'
import { languages } from '@codemirror/language-data'
import { classHighlighter, highlightCode } from '@lezer/highlight'

/** One styled piece of a line. cls is a `tok-*` class, or '' for plain text. */
export interface Token {
  text: string
  cls: string
  /** Set on text the diff marks as changed within the line. */
  changed?: boolean
  /** The mark's kind, when it's formatting or a rename rather than a change. */
  kind?: 'format' | 'rename'
}

/**
 * Splits a line's tokens where changed text starts and ends, and flags the changed pieces.
 * marks are [from, to) offsets into the line, in UTF-16 code units as JavaScript counts.
 */
export function withMarks(tokens: Token[], marks: readonly ([number, number] | [number, number, 'format' | 'rename'])[] | undefined): Token[] {
  if (!marks?.length) return tokens
  const sorted = [...marks].sort((a, b) => a[0] - b[0])
  const out: Token[] = []
  let at = 0
  for (const t of tokens) {
    let start = 0
    while (start < t.text.length) {
      const pos = at + start
      const inside = sorted.find(([f, e]) => pos >= f && pos < e)
      const kind = inside?.[2]
      // The piece runs to the next edge of a mark, or the token's end.
      let end = t.text.length
      for (const [f, e] of sorted) {
        if (f > pos && f - at < end) end = f - at
        if (e > pos && e - at < end) end = e - at
      }
      const piece: Token = { text: t.text.slice(start, end), cls: t.cls }
      if (inside) piece.changed = true
      if (kind) piece.kind = kind
      out.push(piece)
      start = end
    }
    at += t.text.length
  }
  return out
}

const cache = new Map<string, Promise<LanguageSupport | null>>()

/** Loads the language for a file name, or null if none matches. Languages load lazily. */
export function languageFor(filename: string): Promise<LanguageSupport | null> {
  const desc = LanguageDescription.matchFilename(languages, filename)
  if (!desc) return Promise.resolve(null)
  let p = cache.get(desc.name)
  if (!p) {
    p = desc.load().catch(() => null)
    cache.set(desc.name, p)
  }
  return p
}

/** Above this size, highlighting is skipped; parsing would stall the UI. */
const MAX_HIGHLIGHT = 1 << 20

/**
 * Highlights text and returns tokens per line. Lines match splitLines(text) by index;
 * terminators are not included in the tokens.
 */
export function highlightLines(text: string, lang: LanguageSupport | null): Token[][] {
  if (!lang || text.length > MAX_HIGHLIGHT) {
    return text.split('\n').map((line) => [{ text: line.replace(/\r$/, ''), cls: '' }])
  }
  const lines: Token[][] = [[]]
  const tree = lang.language.parser.parse(text)
  highlightCode(
    text,
    tree,
    classHighlighter,
    (t, cls) => lines[lines.length - 1].push({ text: t.replace(/\r$/, ''), cls }),
    () => lines.push([]),
  )
  return lines
}
