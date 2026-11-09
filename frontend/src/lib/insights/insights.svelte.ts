import {
  Fetch,
  GroupBatch,
  Groups,
  InsightsBranches,
  InsightsFetchedAt,
  InsightsHotspots,
  InsightsShipped,
  SaveFile,
  InsightsReleaseBranches,
  InsightsTags,
  InsightsThresholds,
  SetInsightsTab,
  SetInsightsThresholds,
} from '../../../wailsjs/go/app/App.js'
import type { app, git, settings } from '../../../wailsjs/go/models'
import { EventsOn } from '../../../wailsjs/runtime/runtime.js'
import { errorText } from '../errors'
import { samePath } from '../gitconfig/rows'
import { session } from '../session.svelte'
import { textToBase64 } from './export'
import { attention, hotFiles, sharedAsOf, type ConflictInfo, type Ranked, type Tagged } from './model'
import { attentionCSV, exportName, hotspotsCSV, shippedCSV, type ReportInputs } from './report'

/** What the tab looks at: one repository, or a project group's repositories together. */
export interface Scope {
  kind: 'repo' | 'group'
  /** A repository path, or a group name. */
  id: string
}

export type WindowKind = '7d' | '14d' | 'month' | 'releases'

export type View = 'overview' | 'shipped' | 'attention' | 'hotspots'

export const VIEWS: { id: View; label: string }[] = [
  { id: 'overview', label: 'Overview' },
  { id: 'shipped', label: 'Shipped' },
  { id: 'attention', label: 'Needs attention' },
  { id: 'hotspots', label: 'Hotspots' },
]

/** The defaults, until a scope's saved thresholds load. */
const DEFAULT_THRESHOLDS = { stale: 7, longLived: 14, behind: 50, size: 500 }

/** One repository's results. */
interface RepoData {
  shipped?: app.ShippedResult
  branches?: app.AttentionResult
  /** When the repository last fetched, Unix seconds; 0 if never. Display only. */
  fetchedAt: number
  /** Loaded when the Hotspots view is open. */
  hotspots?: app.HotspotsResult
  error: string
}

/** How many repositories compute at once in a group. */
const PARALLEL = 4

const encode = (s: Scope) => `${s.kind}:${s.id}`
const decode = (v: string): Scope | null => {
  const i = v.indexOf(':')
  const kind = v.slice(0, i)
  return kind === 'repo' || kind === 'group' ? { kind, id: v.slice(i + 1) } : null
}

/** Runs fn over items, at most limit at a time. */
async function each<T>(items: T[], limit: number, fn: (item: T) => Promise<void>) {
  let next = 0
  const worker = async () => {
    while (next < items.length) await fn(items[next++])
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker))
}

/** The Insights tab (#141, ADR-0008): its scope and inputs, and each repository's results. */
class InsightsTab {
  open = $state(false)
  scope = $state<Scope | null>(null)
  window = $state<WindowKind>('14d')
  from = $state('')
  to = $state('')
  /** An explicit as-of point, Unix seconds, or 0 for each target's tip. */
  asOf = $state(0)
  view = $state<View>('overview')
  thresholds = $state<app.Thresholds>({ ...DEFAULT_THRESHOLDS } as app.Thresholds)
  groups = $state<settings.Group[]>([])
  /** Releases a releases window can run between: tags, and Release branches (ADR 0009) at
   * the point they forked from the target. Newest first. */
  tags = $state<{ name: string; time: number; branch?: boolean }[]>([])
  data = $state<Record<string, RepoData>>({})
  /** Dry-run merge results by target and branch commit. They're kept apart from data
   * because cached checks are reported at once, often before the branch list arrives. */
  conflicts = $state<Record<string, ConflictInfo>>({})
  loading = $state(false)
  error = $state('')
  fetching = $state(false)

  /** The scope's repositories: the one, or the group's. */
  repos = $derived.by(() => {
    if (!this.scope) return []
    if (this.scope.kind === 'repo') return [this.scope.id]
    return this.groups.find((g) => g.name === this.scope?.id)?.paths ?? []
  })

  /** Every shipped item in scope, tagged by repository. */
  items = $derived.by((): Tagged<git.ShippedItem>[] =>
    this.repos.flatMap((repo) => (this.data[repo]?.shipped?.items ?? []).map((value) => ({ repo, value }))),
  )

  /** Every measured branch in scope, with its conflict check once known. */
  branches = $derived.by((): Ranked<app.BranchSignal>[] =>
    this.repos.flatMap((repo) => {
      const d = this.data[repo]
      const target = d?.branches?.window.targetSha ?? ''
      return (d?.branches?.branches ?? []).map((value) => ({ repo, value, conflict: this.conflicts[`${target}:${value.sha}`] }))
    }),
  )

  /** Conflict checks still running. */
  pending = $derived(this.branches.filter((b) => !b.conflict).length)

  /** The window the results share: the first repository's (a group shares one as-of point). */
  resolved = $derived(this.repos.map((r) => this.data[r]?.shipped?.window).find((w) => w))

  private listening = false
  private timer: ReturnType<typeof setTimeout> | undefined
  /** Bumped by each load, so an older one's late results are dropped. */
  private generation = 0

  /** A repository's short name: its tab's, else its folder's. */
  repoName = (path: string): string => session.tabs.find((t) => samePath(t.path, path))?.name ?? path.split(/[\\/]/).pop() ?? path

  /** The hot files across the scope, as the Hotspots view lists them. */
  hot = $derived(hotFiles(this.repos.flatMap((repo) => (this.data[repo]?.hotspots?.files ?? []).map((value) => ({ repo, value })))).slice(0, 20))

  /** What every export prints first: the scope, each repository's target commit, the window,
   * as-of point and thresholds. */
  reportInputs(): ReportInputs {
    return {
      scope: this.label(),
      thresholds: this.thresholds,
      windows: this.repos.flatMap((repo) => {
        const w = this.data[repo]?.shipped?.window
        return w ? [{ ...w, repo, name: this.repoName(repo) }] : []
      }),
    }
  }

  /** Saves one list as CSV, with the inputs at the top. */
  async exportCSV(what: 'shipped' | 'attention' | 'hotspots') {
    const inp = this.reportInputs()
    const text =
      what === 'shipped' ? shippedCSV(inp, this.items, this.repoName) : what === 'attention' ? attentionCSV(inp, attention(this.branches), this.repoName) : hotspotsCSV(inp, this.hot, this.repoName)
    const name = exportName(this.label(), what, inp.windows[0]?.asOf ?? 0, 'csv')
    try {
      await SaveFile('Export CSV', name, textToBase64(text))
    } catch (e) {
      this.error = errorText(e)
    }
  }

  label(s: Scope | null = this.scope): string {
    if (!s) return ''
    if (s.kind === 'group') return s.id
    return session.tabs.find((t) => samePath(t.path, s.id))?.name ?? s.id.split(/[\\/]/).pop() ?? s.id
  }

  /** Opens the tab on a scope (by default the active repository) and selects it. */
  async show(scope?: Scope) {
    this.listen()
    this.open = true
    this.groups = (await Groups().catch(() => null)) ?? []
    const repo = session.activeTab?.path
    await this.setScope(scope ?? this.scope ?? (repo ? { kind: 'repo', id: repo } : this.groups[0] ? { kind: 'group', id: this.groups[0].name } : null))
    await session.select(session.insightsTabId)
  }

  /** Restores the tab from the last session. */
  async restore(saved: string) {
    const s = decode(saved)
    if (!s) return
    this.listen()
    this.open = true
    this.groups = (await Groups().catch(() => null)) ?? []
    await this.setScope(s)
  }

  async close() {
    this.open = false
    if (session.active === session.insightsTabId) await session.select(session.tabs.at(-1)?.path ?? '')
    void SetInsightsTab('').catch(() => {})
  }

  async setScope(s: Scope | null) {
    this.scope = s
    if (!s) return
    // Only remembers the tab for next time.
    void SetInsightsTab(encode(s)).catch(() => {})
    // Release windows go by tags, which only make sense for one repository.
    if (s.kind === 'group' && this.window === 'releases') this.window = '14d'
    if (s.kind === 'repo') {
      const [tags, branches] = await Promise.all([InsightsTags(s.id).catch(() => null), InsightsReleaseBranches(s.id, '').catch(() => null)])
      this.tags = [...(tags ?? []), ...(branches ?? []).map((b) => ({ name: b.name, time: b.forkTime, branch: true }))].sort((a, b) => b.time - a.time || a.name.localeCompare(b.name))
    } else {
      this.tags = []
    }
    // Falling back to the defaults changes what's flagged, so it has to be said.
    this.thresholds = (await InsightsThresholds(encode(s)).catch((e) => {
      session.error = `Using the default thresholds: ${errorText(e)}`
      return null
    })) ?? ({ ...DEFAULT_THRESHOLDS } as app.Thresholds)
    await this.load()
  }

  async setWindow(w: WindowKind) {
    this.window = w
    if (w === 'releases' && this.tags.length >= 2 && !this.from) {
      this.to = this.tags[0].name
      this.from = this.tags[1].name
    }
    await this.load()
  }

  async setView(v: View) {
    this.view = v
    if (v === 'hotspots') await this.loadHotspots()
  }

  /** Saves the scope's thresholds and measures the branches against them. */
  async setThresholds(t: app.Thresholds) {
    if (!this.scope) return
    try {
      await SetInsightsThresholds(encode(this.scope), t)
    } catch (e) {
      this.error = errorText(e)
      return
    }
    this.thresholds = t
    await this.load()
  }

  async setAsOf(seconds: number) {
    this.asOf = seconds
    await this.load()
  }

  private inputs(asOf = this.asOf): app.InsightsInputs {
    return { target: '', window: this.window, since: 0, until: 0, from: this.from, to: this.to, asOf, thresholds: this.thresholds } as app.InsightsInputs
  }

  /** Computes every repository in scope, 4 at a time. A group shares one as-of point, the
   * latest of its repositories', so their windows line up. */
  async load(only?: string[]) {
    const gen = ++this.generation
    const repos = only ?? this.repos
    this.loading = true
    this.error = ''
    try {
      let asOf = this.asOf
      if (this.scope?.kind === 'group' && !asOf) {
        const own: number[] = []
        await each(this.repos, PARALLEL, async (repo) => {
          // Only for lining windows up; the pass below reports each repository's error.
          const r = await InsightsShipped(repo, this.inputs(0)).catch(() => null)
          if (r) own.push(r.window.asOf)
        })
        asOf = sharedAsOf(own)
      }
      await each(repos, PARALLEL, async (repo) => {
        const d: RepoData = { fetchedAt: 0, error: '' }
        try {
          const inputs = this.inputs(asOf)
          ;[d.shipped, d.branches, d.fetchedAt] = await Promise.all([InsightsShipped(repo, inputs), InsightsBranches(repo, inputs), InsightsFetchedAt(repo)])
          if (this.view === 'hotspots') d.hotspots = await InsightsHotspots(repo, inputs)
        } catch (e) {
          d.error = errorText(e)
        }
        if (gen === this.generation) this.data[repo] = d
      })
    } finally {
      if (gen === this.generation) this.loading = false
    }
  }

  /** Loads hotspots for repositories that don't have them for the current results. */
  async loadHotspots() {
    const gen = this.generation
    await each(this.repos, PARALLEL, async (repo) => {
      const d = this.data[repo]
      if (!d?.shipped || d.hotspots) return
      // The same as-of point the other results used, so a group's windows still line up.
      const h = await InsightsHotspots(repo, this.inputs(d.shipped.window.asOf)).catch((e) => {
        d.error = errorText(e)
        return undefined
      })
      if (gen === this.generation && h) d.hotspots = h
    })
  }

  /** Fetches the scope's repositories, then recomputes. */
  async fetch() {
    if (!this.scope) return
    this.fetching = true
    try {
      if (this.scope.kind === 'group') await GroupBatch(this.scope.id, 'fetch')
      else await Fetch(this.scope.id, '')
    } catch (e) {
      this.error = errorText(e)
    } finally {
      this.fetching = false
    }
    await this.load()
  }

  private listen() {
    if (this.listening) return
    this.listening = true
    EventsOn('insights:conflict', (c: ConflictInfo) => {
      this.conflicts[`${c.targetSha}:${c.sha}`] = c
    })
    // A repository's refs moved (a commit, merge or fetch): compute it again. The backend
    // cache makes repositories that didn't change free.
    // The global config may hold smart links, which decide Shipped's references.
    EventsOn('gitconfig:changed', () => {
      if (!this.open) return
      clearTimeout(this.timer)
      this.timer = setTimeout(() => void this.load(), 500)
    })
    EventsOn('repo:changed', (c: { path: string; kinds: string[] }) => {
      // Config holds the smart links that decide Shipped's references.
      if (!this.open || !(c.kinds.includes('refs') || c.kinds.includes('head') || c.kinds.includes('config'))) return
      const repo = this.repos.find((r) => samePath(r, c.path))
      if (!repo) return
      clearTimeout(this.timer)
      this.timer = setTimeout(() => void this.load(this.scope?.kind === 'group' ? undefined : [repo]), 500)
    })
  }
}

export const insights = new InsightsTab()
