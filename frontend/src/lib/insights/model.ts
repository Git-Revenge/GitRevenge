// The Insights tab's pure logic (#249): combining per-repository results and ordering them.
// Every order has a full tie-break, so the same results always display the same way
// (ADR-0008).

/** The fields of a shipped item the model reads (git.ShippedItem). */
export interface Item {
  sha: string
  title: string
  author: string
  email: string
  committed: number
  refs: string[]
  revert: boolean
  fixup: boolean
}

/** The fields of a branch signal the model reads (app.BranchSignal). */
export interface Branch {
  name: string
  sha: string
  author: string
  local: boolean
  lastCommit: number
  flags: string[]
}

/** A dry-run merge result (the insights:conflict event). */
export interface ConflictInfo {
  repo: string
  targetSha: string
  branch: string
  sha: string
  conflicts: boolean
  files: string[]
}

/** Something from one repository of the scope. */
export interface Tagged<T> {
  repo: string
  value: T
}

/** A branch with its conflict check, once known. */
export interface Ranked<T extends Branch = Branch> extends Tagged<T> {
  conflict?: ConflictInfo
}

const DAY = 24 * 60 * 60

/** Branches that need attention: a flag, or a conflict. Conflicts first, then the most
 * flags, then the most recent tip, then repository and name. */
export function attention<T extends Branch>(branches: Ranked<T>[]): Ranked<T>[] {
  return branches
    .filter((b) => b.value.flags.length > 0 || b.conflict?.conflicts)
    .sort((a, b) => {
      const ca = a.conflict?.conflicts ? 1 : 0
      const cb = b.conflict?.conflicts ? 1 : 0
      return (
        cb - ca ||
        b.value.flags.length - a.value.flags.length ||
        b.value.lastCommit - a.value.lastCommit ||
        a.repo.localeCompare(b.repo, 'en') ||
        a.value.name.localeCompare(b.value.name, 'en')
      )
    })
}

/** The newest shipped items first, ties by repository and SHA. */
export function latest<T extends Item>(items: Tagged<T>[]): Tagged<T>[] {
  return [...items].sort((a, b) => b.value.committed - a.value.committed || a.repo.localeCompare(b.repo, 'en') || a.value.sha.localeCompare(b.value.sha, 'en'))
}

export interface Tiles {
  shipped: number
  people: number
  attention: number
  conflicts: number
  rework: number
}

export function tiles(items: Tagged<Item>[], branches: Ranked[]): Tiles {
  const flagged = attention(branches)
  return {
    shipped: items.length,
    people: new Set(items.map((i) => i.value.email.toLowerCase())).size,
    attention: flagged.length,
    conflicts: flagged.filter((b) => b.conflict?.conflicts).length,
    rework: items.filter((i) => i.value.revert || i.value.fixup).length,
  }
}

export interface DayCount {
  /** The UTC day's start, Unix seconds. */
  day: number
  count: number
}

/** Items per UTC day, for every day from since to until, empty days included. */
export function perDay(items: Tagged<Item>[], since: number, until: number): DayCount[] {
  const start = Math.floor(since / DAY) * DAY
  const days: DayCount[] = []
  for (let d = start; d <= until; d += DAY) days.push({ day: d, count: 0 })
  for (const i of items) {
    const at = Math.floor((i.value.committed - start) / DAY)
    if (at >= 0 && at < days.length) days[at].count++
  }
  return days
}

/** A UTC date as "Oct 16", with the year when it differs from yearOf's. */
export function utcDay(seconds: number, yearOf = seconds): string {
  const d = new Date(seconds * 1000)
  const sameYear = d.getUTCFullYear() === new Date(yearOf * 1000).getUTCFullYear()
  return d.toLocaleDateString('en', { month: 'short', day: 'numeric', ...(sameYear ? {} : { year: 'numeric' }), timeZone: 'UTC' })
}

/** A UTC time as "Oct 29, 17:42". */
export function utcMoment(seconds: number): string {
  const d = new Date(seconds * 1000)
  const date = d.toLocaleDateString('en', { month: 'short', day: 'numeric', timeZone: 'UTC' })
  const time = d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', timeZone: 'UTC' })
  return `${date}, ${time}`
}

/** How long ago a moment was, roughly, for fetch freshness: "2 hours ago". Display only. */
/** A project group in the scope menu, short enough for the rail's width. */
export const groupLabel = (name: string, repos: number) => `${name} · ${repos} ${repos === 1 ? 'repo' : 'repos'}`

export function ago(seconds: number, now = Date.now() / 1000): string {
  const s = Math.max(0, now - seconds)
  if (s < 90) return 'just now'
  if (s < 90 * 60) return `${Math.round(s / 60)} minutes ago`
  if (s < 36 * 3600) return `${Math.round(s / 3600)} hours ago`
  return `${Math.round(s / DAY)} days ago`
}

/** The group scope's shared as-of point: the latest of its repositories' own. */
export function sharedAsOf(asOfs: number[]): number {
  return asOfs.reduce((m, v) => Math.max(m, v), 0)
}

/** Shipped items that share a ticket reference. */
export interface RefGroup<T extends Item = Item> {
  /** The reference, or '' for items without one. */
  ref: string
  items: Tagged<T>[]
  /** The median cycle time of the merges among them, seconds; 0 when none is known. */
  cycle: number
}

/** The median of the known (non-zero) values, or 0. */
export function median(values: number[]): number {
  const known = values.filter((v) => v > 0).sort((a, b) => a - b)
  if (known.length === 0) return 0
  const mid = Math.floor(known.length / 2)
  return known.length % 2 ? known[mid] : (known[mid - 1] + known[mid]) / 2
}

/** Groups items by their first reference: the biggest groups first, then the most recent,
 * then by reference; items without one come last. Items keep latest() order inside. */
export function byReference<T extends Item & { cycle: number }>(items: Tagged<T>[]): RefGroup<T>[] {
  const groups = new Map<string, Tagged<T>[]>()
  for (const i of latest(items)) {
    const ref = i.value.refs[0] ?? ''
    groups.set(ref, [...(groups.get(ref) ?? []), i])
  }
  return [...groups.entries()]
    .map(([ref, list]) => ({ ref, items: list, cycle: median(list.map((i) => i.value.cycle)) }))
    .sort(
      (a, b) =>
        Number(a.ref === '') - Number(b.ref === '') ||
        b.items.length - a.items.length ||
        b.items[0].value.committed - a.items[0].value.committed ||
        a.ref.localeCompare(b.ref, 'en'),
    )
}

/** Items by the UTC day they landed, newest day first. */
export function byDay<T extends Item>(items: Tagged<T>[]): { day: number; items: Tagged<T>[] }[] {
  const days = new Map<number, Tagged<T>[]>()
  for (const i of latest(items)) {
    const day = Math.floor(i.value.committed / DAY) * DAY
    days.set(day, [...(days.get(day) ?? []), i])
  }
  return [...days.entries()].map(([day, list]) => ({ day, items: list }))
}

/** Seconds as a short duration: "40 min", "5.5 h", "3.2 days". */
export function duration(seconds: number): string {
  const h = seconds / 3600
  if (h < 1) return `${Math.round(seconds / 60)} min`
  if (h < 48) return `${h.toFixed(h < 10 ? 1 : 0)} h`
  return `${(h / 24).toFixed(1)} days`
}

/** The fields of a hot file the model reads (app.HotFile). */
export interface Hot {
  path: string
  lines: number
  items: number
  openBranches: number
}

/** Hot files from every repository, most lines first, then most items, repository and path. */
export function hotFiles<T extends Hot>(files: Tagged<T>[]): Tagged<T>[] {
  return [...files].sort(
    (a, b) =>
      b.value.lines - a.value.lines ||
      b.value.items - a.value.items ||
      a.repo.localeCompare(b.repo, 'en') ||
      a.value.path.localeCompare(b.value.path, 'en'),
  )
}
