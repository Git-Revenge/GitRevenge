import type { app, git } from '../../../wailsjs/go/models'

// Words for the history search (#982): a chip on the filter strip for the commits where a
// text appeared or went away (git log -S), or where a line with it changed (-G). The search
// runs in the backend (#981); these say what it's doing and what each commit did.

export type HistoryOptions = Pick<git.HistoryOptions, 'text' | 'matchCase' | 'regex' | 'lines' | 'paths' | 'since'>
type State = Pick<app.HistoryState, 'found' | 'searched' | 'total' | 'backTo' | 'running' | 'stopped' | 'ms' | 'estimateMs' | 'problem'> & { options: HistoryOptions }

/** A search for text, with every other option as it starts: plain text, any case, the whole
 * history, every file. */
export function historyOptions(text: string, o: Partial<HistoryOptions> = {}): HistoryOptions {
  return { text, matchCase: false, regex: false, lines: false, paths: [], since: 0, ...o }
}

/** A new search's Only in, while the timeline is focused on a folder or file (#1062): that
 * path, unless the search names files of its own. Only in reads git's patterns, so a path with
 * a pattern's characters in it is given as it's spelled. */
export function inFocus(o: HistoryOptions, focusPath: string): HistoryOptions {
  if (!focusPath || o.paths?.length) return o
  const literal = /[*?[\\]/.test(focusPath) || /^[!:]/.test(focusPath)
  return { ...o, paths: [literal ? `:(literal)${focusPath}` : focusPath] }
}

/** Names a search's options, so what's worked out for one isn't used for another. */
export function historyKey(o: HistoryOptions): string {
  return JSON.stringify([o.text, o.matchCase, o.regex, o.lines, o.paths ?? [], o.since])
}

/** What the chip says before the text: what a commit has to have done. */
export function chipVerb(o: Pick<HistoryOptions, 'lines'>): string {
  return o.lines ? 'changes a line with' : 'adds or removes'
}

/** How the chip is said aloud, as its button: "Commits that add or remove km/h, options". */
export function chipName(o: Pick<HistoryOptions, 'text' | 'lines'>): string {
  return `Commits that ${o.lines ? 'change a line with' : 'add or remove'} ${o.text}, options`
}

const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']
const SHORT = MONTHS.map((m) => m.slice(0, 3))

/**
 * When a search has got back to, roughly: a day in the last two months ("Oct 20"), a month in
 * the last year ("March 2026"), and before that a part of a year ("late 2024"). Every commit
 * since then has been searched, so it needn't be exact.
 */
export function backToText(unix: number, now = new Date()): string {
  const d = new Date(unix * 1000)
  const days = (now.getTime() - d.getTime()) / 86_400_000
  if (days < 61) return `${SHORT[d.getMonth()]} ${d.getDate()}`
  if (days < 365) return `${MONTHS[d.getMonth()]} ${d.getFullYear()}`
  const part = d.getMonth() < 4 ? 'early' : d.getMonth() < 8 ? 'mid' : 'late'
  return `${part} ${d.getFullYear()}`
}

/** A date as the options write it: "Jan 1, 2025". */
export function dayText(unix: number): string {
  const d = new Date(unix * 1000)
  return `${SHORT[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`
}

/** "about 20 minutes", "about 40 seconds", "about 2 hours": an estimate, so rounded. */
export function aboutText(ms: number): string {
  const s = Math.max(1, Math.round(ms / 1000))
  if (s < 60) return `about ${s} ${s === 1 ? 'second' : 'seconds'}`
  const m = Math.round(s / 60)
  if (m < 60) return `about ${m} ${m === 1 ? 'minute' : 'minutes'}`
  const h = Math.round(m / 6) / 10
  return `about ${h} ${h === 1 ? 'hour' : 'hours'}`
}

/** How long something took, shortly: "0.2 s", "40 s", "5 min 47 s". */
export function tookText(ms: number): string {
  if (ms < 10_000) return `${(Math.max(ms, 100) / 1000).toFixed(1)} s`
  const s = Math.round(ms / 1000)
  if (s < 60) return `${s} s`
  const m = Math.floor(s / 60)
  return s % 60 ? `${m} min ${s % 60} s` : `${m} min`
}

/** A count of commits: "1,511", or "1.5 million" for a big history. */
export function bigCount(n: number): string {
  if (n >= 1_000_000) return `${(Math.round(n / 100_000) / 10).toLocaleString()} million`
  return n.toLocaleString()
}

const commits = (n: number) => `${n.toLocaleString()} ${n === 1 ? 'commit' : 'commits'}`

/** A partial clone's files that were never downloaded end a search: not a mistake, only
 * where what's on this machine runs out (#981 words it). */
export const PARTIAL_CLONE = 'This is a partial clone'

export type HistoryTone = 'running' | 'stopped' | 'done' | 'problem' | 'partial'

/** What the strip says about the search: under way, stopped, done, or ended early. */
export function historyStatus(h: State, now = new Date()): { tone: HistoryTone; text: string } {
  const back = h.backTo > 0 ? backToText(h.backTo, now) : ''
  const found = `${h.found.toLocaleString()} found`
  if (h.running) return { tone: 'running', text: back ? `Searching… ${found}, back to ${back}.` : `Searching… ${found}.` }
  if (h.problem) return { tone: h.problem.startsWith(PARTIAL_CLONE) ? 'partial' : 'problem', text: h.problem }
  if (h.stopped) return { tone: 'stopped', text: back ? `Stopped at ${back}: ${found}. Older commits weren’t searched.` : 'Stopped before any commit was searched.' }
  // The strip's count already says how many commits there are: that one counts a stash's row,
  // which a search leaves out as git does every merge, so no second count stands beside it.
  const all = h.options.since ? `${commits(h.searched)} since ${dayText(h.options.since)}` : 'the whole history'
  return { tone: 'done', text: `Searched ${all} · ${tookText(h.ms)}` }
}

/** After this long, a search with far longer to go offers to narrow itself. */
export const NARROW_AFTER = 5_000
/** How much longer a search has to have to go before narrowing is offered. */
export const NARROW_LEFT = 60_000

/** Whether to offer to narrow the search: it has run a few seconds, has more than a minute
 * to go, and is over the whole history and every file. */
export function offersNarrowing(h: State): boolean {
  return h.running && h.ms >= NARROW_AFTER && h.estimateMs - h.ms > NARROW_LEFT && !h.options.since && !h.options.paths?.length
}

/** "All 1.5 million commits will take about 20 minutes. Narrow it?" */
export function narrowText(h: Pick<State, 'total' | 'estimateMs'>): string {
  return `All ${bigCount(h.total)} commits will take ${aboutText(h.estimateMs)}. Narrow it?`
}

/** A year ago, as a search's since: what "The last year" searches back to. */
export function lastYear(now = new Date()): number {
  return Math.floor(now.getTime() / 1000) - 365 * 86_400
}

/** Which choice of "As far back as" a since is: the whole history, the last year (a day
 * either way, as it moves on while the search is there), or a date. */
export function sinceChoice(since: number, now = new Date()): 'all' | 'year' | 'date' {
  if (!since) return 'all'
  return Math.abs(since - lastYear(now)) < 86_400 ? 'year' : 'date'
}

/** A date input's value ("2025-01-31") as a since: the start of that day where the user is. */
export function sinceOfDay(value: string): number {
  const [y, m, d] = value.split('-').map(Number)
  if (!y || !m || !d) return 0
  return Math.floor(new Date(y, m - 1, d).getTime() / 1000)
}

/** A since as a date input's value. */
export function dayOfSince(since: number): string {
  if (!since) return ''
  const d = new Date(since * 1000)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/** What a commit did with the text, beside it on the graph: "+1 map.js", "−1 feed.js",
 * "±1 checks.md" where it changed a line and stayed, or "+3 in 2 files". In words as well as
 * colour, and `says` names each file in full, for a tooltip. */
export interface ChangeMark {
  label: string
  tone: 'plus' | 'minus' | 'even'
  says: string
}

/** What a commit did with the text in one file (git.HistoryChange in Go): how many times it's
 * in the lines added and removed, or with lines, how many such lines. Wails makes no class for
 * it, as it only comes inside a map, so it's said here. */
export interface HistoryChange {
  file: string
  added: number
  removed: number
}

const fileName = (file: string) => file.slice(file.lastIndexOf('/') + 1)

function fileSays(c: HistoryChange, text: string): string {
  const d = c.added - c.removed
  const times = Math.abs(d) > 1 ? `, ${Math.abs(d)} times` : ''
  if (d > 0) return `Brings ${text} into ${c.file}${times}`
  if (d < 0) return `Takes ${text} out of ${c.file}${times}`
  return `Changes ${c.added === 1 ? 'a line' : `${c.added} lines`} with ${text} in ${c.file}`
}

export function changeMark(changes: HistoryChange[] | undefined, text: string): ChangeMark | null {
  if (!changes?.length) return null
  const d = changes.reduce((n, c) => n + c.added - c.removed, 0)
  const tone = d > 0 ? 'plus' : d < 0 ? 'minus' : 'even'
  const size = d === 0 ? changes.reduce((n, c) => n + c.added, 0) : Math.abs(d)
  const sign = d > 0 ? '+' : d < 0 ? '−' : '±'
  const where = changes.length === 1 ? fileName(changes[0].file) : `in ${changes.length} files`
  return { label: `${sign}${size} ${where}`, tone, says: changes.map((c) => fileSays(c, text)).join('\n') }
}

/** A mark's own words for each file, in the commit panel's list: "+1", "−2", "±1". */
export function fileMark(c: HistoryChange): { label: string; tone: ChangeMark['tone'] } {
  const d = c.added - c.removed
  if (d > 0) return { label: `+${d}`, tone: 'plus' }
  if (d < 0) return { label: `−${-d}`, tone: 'minus' }
  return { label: `±${c.added}`, tone: 'even' }
}

/** The count beside a way of matching in the options, once a search that way has finished. */
export function countText(n: number | undefined): string {
  return n === undefined ? '' : commits(n)
}
