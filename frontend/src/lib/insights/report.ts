// What leaves the Insights tab (#252): a Markdown summary and CSV files. Each starts with the
// inputs it came from, including every repository's target commit, so anyone with those
// commits can reproduce it (ADR-0008).
import { toCSV } from './export'
import { byReference, latest, type Branch, type Hot, type Item, type Ranked, type Tagged } from './model'

/** One repository's resolved window (app.InsightsWindow). */
export interface RepoWindow {
  repo: string
  name: string
  target: string
  targetSha: string
  asOf: number
  since: number
  until: number
  range: string
  /** The commits a releases window measures ("sha..sha"), for release branches. */
  points?: string
  /** The branch kind patterns in use (ADR 0009), keyed by kind. */
  kinds?: Record<string, string[]>
  /** The smart links that find ticket references, "name pattern" each; empty for the
   * built-in PROJ-123 and #123. */
  references?: string[]
}

/** A thresholds set (app.Thresholds). */
export interface Limits {
  stale: number
  longLived: number
  behind: number
  size: number
}

export interface ReportInputs {
  scope: string
  windows: RepoWindow[]
  thresholds: Limits
}

/** A ranked branch with the measures the reports print (app.BranchSignal). */
export interface Signal extends Branch {
  staleDays: number
  ageDays: number
  behind: number
  added: number
  removed: number
}

/** A shipped item with its size and cycle (git.ShippedItem). */
export interface Shipped extends Item {
  files: number
  added: number
  removed: number
  merge: boolean
  cycle: number
  /** A dependency update a bot opened, grouped apart in Shipped. */
  bot?: boolean
}

/** "1 item", "2 items". */
const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

const iso = (seconds: number) => new Date(seconds * 1000).toISOString().replace('.000Z', 'Z')

/** The window as one line: a tag range, or absolute UTC bounds. */
function windowLine(w: RepoWindow): string {
  return w.range ? w.range + (w.points && w.points !== w.range ? ` (commits ${w.points})` : '') : `${iso(w.since)} to ${iso(w.until)}`
}

/** The kind patterns in use, one line: "docs gh-pages gh-docs; bot dependabot/* …". */
export function kindsLine(kinds: Record<string, string[]> | undefined): string {
  if (!kinds) return ''
  return ['trunk', 'release', 'environment', 'docs', 'bot', 'feature']
    .map((k) => `${k} ${kinds[k]?.length ? kinds[k].join(' ') : '(none)'}`)
    .join('; ')
}

/** The inputs, one entry per line, shared by every export. */
export function inputLines(inp: ReportInputs): [string, string][] {
  const lines: [string, string][] = [['Scope', inp.scope]]
  for (const w of inp.windows) lines.push([`Repository ${w.name}`, `${w.target} @ ${w.targetSha}`])
  const first = inp.windows[0]
  if (first) {
    lines.push(['Window', windowLine(first)])
    lines.push(['As of', iso(first.asOf)])
  }
  const kinds = first ? kindsLine(first.kinds) : ''
  if (kinds) lines.push(['Branch kinds', kinds])
  if (first) lines.push(['References', first.references?.length ? first.references.join('; ') : 'built in: PROJ-123 and #123'])
  const t = inp.thresholds
  lines.push(['Thresholds', `stale ${t.stale}d, long-lived ${t.longLived}d, behind ${t.behind}, size ${t.size} lines`])
  return lines
}

/** The words for a branch's flags and conflict, in the summary. */
function branchNotes(b: Ranked<Signal>): string {
  const notes: string[] = []
  if (b.conflict?.conflicts) notes.push(`would conflict (${b.conflict.files.join(', ')})`)
  const f = b.value.flags
  if (f.includes('stale')) notes.push(`no commits for ${b.value.staleDays} days`)
  if (f.includes('long-lived')) notes.push(`${b.value.ageDays} days old`)
  if (f.includes('drifting')) notes.push(`${b.value.behind} behind`)
  if (f.includes('big-batch')) notes.push(`${b.value.added + b.value.removed} lines`)
  if (!b.conflict) notes.push('conflict check pending')
  return notes.join(', ')
}

export interface SummaryParts {
  shipped: boolean
  attention: boolean
  hotspots: boolean
  groupByReference: boolean
}

/** The Markdown summary. Lists are in the same order the tab shows them. */
export function summary(
  inp: ReportInputs,
  items: Tagged<Shipped>[],
  flagged: Ranked<Signal>[],
  hot: Tagged<Hot>[],
  parts: SummaryParts,
  repoName: (path: string) => string,
): string {
  const multi = inp.windows.length > 1
  const first = inp.windows[0]
  const out: string[] = [`# ${inp.scope}: ${first ? windowLine(first) : ''}`, '']
  for (const [k, v] of inputLines(inp)) out.push(`${k}: ${v}  `)
  out.push('')
  const where = (repo: string, author: string) => (multi ? `${repoName(repo)}, ${author}` : author)

  if (parts.shipped) {
    // Bot items (dependency updates) are listed apart, after the work people did.
    const work = items.filter((i) => !i.value.bot)
    const deps = items.filter((i) => i.value.bot)
    out.push(`## Shipped (${items.length})`, '')
    if (parts.groupByReference) {
      for (const g of byReference(work)) {
        out.push(`**${g.ref || 'No reference'}**`)
        for (const i of g.items) out.push(`- ${i.value.title} (${where(i.repo, i.value.author)})`)
        out.push('')
      }
    } else {
      for (const i of latest(work)) out.push(`- ${i.value.title} (${where(i.repo, i.value.author)})`)
      out.push('')
    }
    if (deps.length) {
      out.push(`**Dependency updates (${deps.length})**`)
      for (const i of latest(deps)) out.push(`- ${i.value.title} (${where(i.repo, i.value.author)})`)
      out.push('')
    }
  }
  if (parts.attention) {
    out.push(`## Needs attention (${flagged.length})`, '')
    for (const b of flagged) out.push(`- ${b.value.name}${multi ? ` (${repoName(b.repo)})` : ''}${b.value.local ? ', local only' : ''}: ${branchNotes(b)}`)
    if (flagged.length === 0) out.push('Nothing is past a threshold or conflicting.')
    out.push('')
  }
  if (parts.hotspots) {
    out.push(`## Hotspots`, '')
    for (const h of hot) {
      const v = h.value
      const open = v.openBranches ? `, ${plural(v.openBranches, 'open branch touches', 'open branches touch')} it` : ''
      out.push(`- ${v.path}${multi ? ` (${repoName(h.repo)})` : ''}: ${plural(v.lines, 'line', 'lines')} in ${plural(v.items, 'item', 'items')}${open}`)
    }
    out.push('')
  }
  return out.join('\n').trimEnd() + '\n'
}

/** A CSV with the inputs first, a blank row, then the table. */
function withInputs(inp: ReportInputs, table: (string | number)[][]): string {
  return toCSV([...inputLines(inp).map(([k, v]) => [k, v]), [], ...table])
}

export function shippedCSV(inp: ReportInputs, items: Tagged<Shipped>[], repoName: (p: string) => string): string {
  const rows: (string | number)[][] = [['repository', 'commit', 'committed_utc', 'title', 'author', 'email', 'references', 'files', 'added', 'removed', 'merge', 'cycle_hours', 'revert', 'fixup', 'bot']]
  for (const { repo, value: i } of latest(items)) {
    rows.push([repoName(repo), i.sha, iso(i.committed), i.title, i.author, i.email, i.refs.join(' '), i.files, i.added, i.removed, i.merge ? 'yes' : 'no', i.cycle ? (i.cycle / 3600).toFixed(1) : '', i.revert ? 'yes' : 'no', i.fixup ? 'yes' : 'no', i.bot ? 'yes' : 'no'])
  }
  return withInputs(inp, rows)
}

export function attentionCSV(inp: ReportInputs, flagged: Ranked<Signal>[], repoName: (p: string) => string): string {
  const rows: (string | number)[][] = [['repository', 'branch', 'commit', 'local', 'author', 'conflicts', 'conflict_files', 'stale_days', 'age_days', 'behind', 'lines', 'flags']]
  for (const b of flagged) {
    const v = b.value
    rows.push([
      repoName(b.repo),
      v.name,
      v.sha,
      v.local ? 'yes' : 'no',
      v.author,
      b.conflict ? (b.conflict.conflicts ? 'yes' : 'no') : 'pending',
      b.conflict?.files.join(' ') ?? '',
      v.staleDays,
      v.ageDays,
      v.behind,
      v.added + v.removed,
      v.flags.join(' '),
    ])
  }
  return withInputs(inp, rows)
}

export function hotspotsCSV(inp: ReportInputs, hot: Tagged<Hot>[], repoName: (p: string) => string): string {
  const rows: (string | number)[][] = [['repository', 'path', 'lines', 'items', 'open_branches']]
  for (const h of hot) rows.push([repoName(h.repo), h.value.path, h.value.lines, h.value.items, h.value.openBranches])
  return withInputs(inp, rows)
}

/** A file name for an export: "insights-tidewater-shipped-2026-07-22.csv". */
export function exportName(scope: string, what: string, asOf: number, ext: string): string {
  const slug = scope.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'scope'
  return `insights-${slug}-${what}-${iso(asOf).slice(0, 10)}.${ext}`
}
