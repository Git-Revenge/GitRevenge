import { diffLines } from './diff/lineDiff'

// A changed pair of two versions of a branch, read (#1039): range-diff's diff-of-diffs turned
// into what the Marked and Side by side views draw. Each of its lines is indented four spaces,
// then carries an outer mark (this version against the old: space, + or -), then the commit's
// own diff line with its inner mark.

export type Outer = ' ' | '+' | '-'
/** A line's inner mark: the commit's own diff (+, - or context), a hunk header, or plain text
 * (the message and its metadata). */
export type Inner = '+' | '-' | ' ' | 'hunk' | 'text'

export interface Line {
  outer: Outer
  inner: Inner
  text: string
}

export interface Section {
  /** A file's path, "Commit message" or "Metadata". */
  name: string
  kind: 'file' | 'message' | 'metadata'
  lines: Line[]
}

const kindOf = (name: string): Section['kind'] => (name === 'Commit message' ? 'message' : name === 'Metadata' ? 'metadata' : 'file')

/** A file's name as range-diff writes it, without "(new)" or "(deleted)". */
const cleanName = (name: string) => name.replace(/ \((new|deleted|mode change)\)$/, '').replace(/:.*$/, '')

/** Reads a pair's text (its header line, then its diff-of-diffs) into sections. A section that
 * starts again with the same name is the same section. */
export function parsePair(text: string): Section[] {
  const sections: Section[] = []
  let at: Section | null = null
  const open = (raw: string) => {
    const name = raw.trim()
    const clean = kindOf(name) === 'file' ? cleanName(name) : name
    at = sections.find((s) => s.name === clean) ?? null
    if (!at) {
      at = { name: clean, kind: kindOf(clean), lines: [] }
      sections.push(at)
    }
  }
  const lines = text.split('\n').slice(1)
  for (const raw of lines) {
    if (!raw.startsWith('    ')) continue
    const line = raw.slice(4)
    if (line.startsWith('@@ ')) {
      open(line.slice(3))
      continue
    }
    const outer = line[0] as Outer
    if (outer !== ' ' && outer !== '+' && outer !== '-') continue
    const rest = line.slice(1)
    const header = rest.trimStart()
    if (outer === ' ' && header.startsWith('## ')) {
      open(header.slice(3).replace(/ ##$/, ''))
      continue
    }
    if (!at) open('Metadata')
    const s = at as unknown as Section
    if (s.kind !== 'file') {
      // The message is indented four spaces under its header; metadata isn't.
      s.lines.push({ outer, inner: 'text', text: s.kind === 'message' ? rest.replace(/^ {4}/, '') : rest })
    } else if (rest.startsWith('@@')) {
      s.lines.push({ outer, inner: 'hunk', text: rest })
    } else {
      const inner = (rest[0] ?? ' ') as Inner
      s.lines.push({ outer, inner: inner === '+' || inner === '-' ? inner : ' ', text: rest.slice(1) })
    }
  }
  // A section whose lines all match in both versions says nothing: range-diff only writes it
  // for context. The metadata is only shown when it changed.
  return sections.filter((s) => s.lines.some((l) => l.outer !== ' '))
}

/** The files a pair's change touches, from its diff-of-diffs. */
export function filesOf(text: string): string[] {
  const files = new Set<string>()
  for (const raw of text.split('\n')) {
    const m = raw.match(/^ {4}(?:@@ |[ +-]\s*## )(.+?)(?: ##)?$/)
    if (m && kindOf(m[1].trim()) === 'file') files.add(cleanName(m[1].trim()))
  }
  return [...files]
}

/** A piece of a line, marked when it's what differs from the other version. */
export interface Piece {
  text: string
  marked: boolean
}

/** A Marked row: a line, with the words that differ from its partner picked out. */
export interface MarkedRow extends Line {
  pieces: Piece[]
}

const tokens = (s: string) => s.match(/\s+|[A-Za-z0-9_]+|[^\sA-Za-z0-9_]/g) ?? []

/** The words of b that a doesn't have, as pieces of b; and the other way round. */
function wordPieces(a: string, b: string): { a: Piece[]; b: Piece[] } {
  const ta = tokens(a)
  const tb = tokens(b)
  const ops = diffLines(ta, tb)
  const pa: Piece[] = []
  const pb: Piece[] = []
  const push = (to: Piece[], text: string, marked: boolean) => {
    const last = to[to.length - 1]
    if (last && last.marked === marked) last.text += text
    else to.push({ text, marked })
  }
  for (const op of ops) {
    if (op.kind === 'eq') {
      push(pa, ta[op.oldIndex], false)
      push(pb, tb[op.newIndex], false)
    } else if (op.kind === 'del') push(pa, ta[op.oldIndex], true)
    else push(pb, tb[op.newIndex], true)
  }
  return { a: pa, b: pb }
}

/** The Marked view of a section: every line, in order. In each run of lines that differ, the
 * nth gone line is set against the nth new one, and the words that differ are marked. */
export function markedRows(s: Section): MarkedRow[] {
  const rows: MarkedRow[] = s.lines.map((l) => ({ ...l, pieces: [{ text: l.text, marked: false }] }))
  let i = 0
  while (i < rows.length) {
    if (rows[i].outer === ' ') {
      i++
      continue
    }
    let j = i
    while (j < rows.length && rows[j].outer !== ' ') j++
    const gone = rows.slice(i, j).filter((r) => r.outer === '-')
    const added = rows.slice(i, j).filter((r) => r.outer === '+')
    for (let k = 0; k < Math.min(gone.length, added.length); k++) {
      const w = wordPieces(gone[k].text, added[k].text)
      gone[k].pieces = w.a
      added[k].pieces = w.b
    }
    i = j
  }
  return rows
}

/** A Side by side row: the old version's line beside the new one's, either missing. */
export interface SideRow {
  left: MarkedRow | null
  right: MarkedRow | null
}

/** The Side by side view of a section: lines the same in both side by side, and each run that
 * differs as its gone lines beside its new ones. */
export function sideRows(s: Section): SideRow[] {
  const marked = markedRows(s)
  const rows: SideRow[] = []
  let i = 0
  while (i < marked.length) {
    if (marked[i].outer === ' ') {
      rows.push({ left: marked[i], right: marked[i] })
      i++
      continue
    }
    let j = i
    while (j < marked.length && marked[j].outer !== ' ') j++
    const gone = marked.slice(i, j).filter((r) => r.outer === '-')
    const added = marked.slice(i, j).filter((r) => r.outer === '+')
    for (let k = 0; k < Math.max(gone.length, added.length); k++) rows.push({ left: gone[k] ?? null, right: added[k] ?? null })
    i = j
  }
  return rows
}

/** How much differs, said under the title: "the message is the same · 1 line differs". */
export function differsSaid(sections: Section[]): string {
  const message = sections.some((s) => s.kind === 'message')
  const files = sections.filter((s) => s.kind === 'file')
  let lines = 0
  for (const s of files) {
    let i = 0
    while (i < s.lines.length) {
      if (s.lines[i].outer === ' ') {
        i++
        continue
      }
      let j = i
      let gone = 0
      let added = 0
      while (j < s.lines.length && s.lines[j].outer !== ' ') {
        if (s.lines[j].outer === '-') gone++
        else added++
        j++
      }
      lines += Math.max(gone, added)
      i = j
    }
  }
  const parts = [message ? 'the message changed' : 'the message is the same']
  if (files.length) parts.push(lines === 1 ? '1 line differs' : `${lines} lines differ`)
  return parts.join(' · ')
}

type PairLike = { kind: string; new: number; subject: string; text: string }

/** The pairs a reader steps through: those that changed in some way. */
export const opens = (p: { kind: string }) => p.kind === 'changed' || p.kind === 'message' || p.kind === 'around'

/** For a pair changed only around it: the latest pair before it, in the new version, that
 * changed a file it touches. That's what moved the lines around it. */
export function aroundBy<P extends PairLike>(pairs: P[], index: number): P | null {
  const p = pairs[index]
  if (!p || p.kind !== 'around') return null
  const mine = new Set(filesOf(p.text))
  let best: P | null = null
  for (const q of pairs) {
    if (q === p || q.new <= 0 || q.new >= p.new || q.kind !== 'changed') continue
    if (!filesOf(q.text).some((f) => mine.has(f))) continue
    if (!best || q.new > best.new) best = q
  }
  return best
}
