// Find in files (#980): what the Find panel shows and how it reads a search's options.
// The search itself is git grep, run by the backend (#979); these turn its lines into rows,
// mark matches, and find the same matches again in a whole file once one is opened.

/** What to find, as the backend takes it (git.GrepOptions). */
export interface FindOptions {
  text: string
  matchCase: boolean
  wholeWord: boolean
  regex: boolean
  untracked: boolean
  /** A commit, as a full ID, or "" for the working tree. */
  rev: string
  paths: string[]
}

/** A line git found (git.GrepLine): its matches are [from, to) in UTF-16 units, as JS counts. */
export interface FoundLine {
  file: string
  line: number
  text: string
  cutStart: boolean
  cutEnd: boolean
  matches: { from: number; to: number }[]
}

/** A row of the results: a file, then its lines. */
export type FindRow = { type: 'file'; file: string; count: number } | { type: 'line'; file: string; found: FoundLine }

/** The results as rows: each file once, before its lines. Git lists a file's lines together. */
export function findRows(lines: FoundLine[]): FindRow[] {
  const rows: FindRow[] = []
  let head = null as Extract<FindRow, { type: 'file' }> | null
  for (const l of lines) {
    if (head?.file !== l.file) {
      head = { type: 'file', file: l.file, count: 0 }
      rows.push(head)
    }
    head.count++
    rows.push({ type: 'line', file: l.file, found: l })
  }
  return rows
}

/** A file's name and the folder it's in, the folder with its trailing slash. */
export function splitPath(file: string): { folder: string; name: string } {
  const i = file.lastIndexOf('/')
  return { folder: file.slice(0, i + 1), name: file.slice(i + 1) }
}

/** A line's text in pieces, the matches marked. */
export function markedParts(found: FoundLine): { text: string; hit: boolean }[] {
  const parts: { text: string; hit: boolean }[] = []
  let at = 0
  for (const m of found.matches) {
    if (m.from < at || m.to <= m.from) continue
    if (m.from > at) parts.push({ text: found.text.slice(at, m.from), hit: false })
    parts.push({ text: found.text.slice(m.from, m.to), hit: true })
    at = m.to
  }
  if (at < found.text.length) parts.push({ text: found.text.slice(at), hit: false })
  return parts
}

/** A found line as the panel shows it: without its indent, and from a little before its first
 * match, so the match is in view in a narrow sidebar. */
export function inView(found: FoundLine, before = 16): FoundLine {
  const first = found.matches[0]?.from ?? 0
  const indent = found.text.length - found.text.trimStart().length
  let cut = first > before ? first - before : Math.min(indent, first)
  // Never between the halves of a character outside the BMP.
  if (cut > 0 && /[\udc00-\udfff]/.test(found.text[cut])) cut--
  if (cut <= 0) return found
  return {
    ...found,
    text: found.text.slice(cut),
    cutStart: found.cutStart || first > before,
    matches: found.matches.map((m) => ({ from: m.from - cut, to: m.to - cut })),
  }
}

/** The Files box: patterns between commas, "!" before one to leave it out. */
export function filePatterns(box: string): string[] {
  return box
    .split(',')
    .map((p) => p.trim())
    .filter((p) => p && p !== '!')
}

/** "2 lines in 2 files · 0.2 s", or what's found so far. */
export function findSummary(lines: number, files: number, ms?: number): string {
  const l = lines === 1 ? '1 line' : `${lines.toLocaleString()} lines`
  const f = files === 1 ? '1 file' : `${files.toLocaleString()} files`
  if (ms === undefined) return `${l} in ${f}`
  return `${l} in ${f} · ${ms < 100 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`}`
}

/** The search as a JS regular expression, or null when JS can't read one git can. */
export function findPattern(o: Pick<FindOptions, 'text' | 'matchCase' | 'regex'>): RegExp | null {
  const source = o.regex ? o.text : o.text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  try {
    // m: ^ and $ are a line's ends, as grep reads them.
    return new RegExp(source, o.matchCase ? 'gm' : 'gim')
  } catch {
    return null
  }
}

/** git grep's word character: an ASCII letter, digit or underscore, or any character past
 * ASCII, since git reads bytes. */
const isWord = (c: string | undefined) => c !== undefined && /[\w\u0080-￿]/.test(c)

/** Every match of the search in a whole file, [from, to) in UTF-16 units: the marks an opened
 * file shows and F7 steps through. A match can't span lines, as git's can't. */
export function matchesIn(doc: string, o: Pick<FindOptions, 'text' | 'matchCase' | 'regex' | 'wholeWord'>, limit = 100_000): { from: number; to: number }[] {
  const re = findPattern(o)
  const out: { from: number; to: number }[] = []
  if (!re || !o.text) return out
  for (let m = re.exec(doc); m && out.length < limit; m = re.exec(doc)) {
    if (m[0].length === 0) {
      re.lastIndex++
      continue
    }
    const from = m.index
    const to = from + m[0].length
    if (m[0].includes('\n')) continue
    if (o.wholeWord && (isWord(doc[from - 1]) || isWord(doc[to]))) continue
    out.push({ from, to })
  }
  return out
}

/** Which of a file's matches is on line n (1-based) or the first after it, for where an
 * opened file starts. lineStarts[i] is where line i+1 starts. */
export function matchAtLine(matches: { from: number }[], lineStarts: number[], n: number): number {
  const start = lineStarts[Math.max(0, Math.min(n, lineStarts.length) - 1)] ?? 0
  const i = matches.findIndex((m) => m.from >= start)
  return i < 0 ? Math.max(0, matches.length - 1) : i
}

/** The line (1-based) that position pos is on, given where each line starts. */
export function lineAt(starts: number[], pos: number): number {
  let lo = 0
  let hi = starts.length - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (starts[mid] <= pos) lo = mid
    else hi = mid - 1
  }
  return lo + 1
}

/** Where each line of doc starts. */
export function lineStarts(doc: string): number[] {
  const starts = [0]
  for (let i = doc.indexOf('\n'); i >= 0; i = doc.indexOf('\n', i + 1)) starts.push(i + 1)
  return starts
}
