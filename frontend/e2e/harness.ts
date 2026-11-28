// Mounts one real component in a real browser, against a stubbed Go backend. A test picks the
// component with ?mount= and supplies the backend's answers through window.__scenario. It isn't
// part of the app's build: only index.html is an entry there.
import { mount, unmount } from 'svelte'
import '../src/style.css'
import type { Scenario } from './support'
import type { DiffTarget } from '../src/lib/diff/DiffView.svelte'
import { guardLinks } from '../src/lib/linkGuard'
import * as bindings from '../wailsjs/go/app/App.js'

type Harness = Window &
  typeof globalThis & {
    __scenario?: Scenario
    __calls: [string, ...unknown[]][]
    __events: [string, ...unknown[]][]
    __ready: boolean
    /** Sends an event from the stubbed backend. */
    __emit: (name: string, data: unknown) => void
    /** Changes what the stubbed backend answers from now on. */
    __respond: (responses: Record<string, unknown>) => void
    /** Swaps the graph for another, as a reload does: the numbered commits, in this order. */
    __graph: (commits: number[]) => void
    /** Tells the app the repository changed on disk, as the watcher would. */
    __changed: (path: string) => void
    go: unknown
    runtime: unknown
  }

const win = window as Harness

// The harness page carries the app's Content-Security-Policy (see vite.config.ts). Anything
// the policy stops fails the test, as an uncaught error does: either the app needs it and the
// policy is wrong, or something got into the page that shouldn't have.
document.addEventListener('securitypolicyviolation', (e) => {
  // The HTML preview parses a repository's page with DOMParser before cleaning it. The parsed
  // page runs nothing, but the browser still reports each on… attribute it reads there as
  // stopped. That report is the policy and the cleaning agreeing, not something to fix.
  if (e.violatedDirective === 'script-src-attr' && /\/previews\/html\.ts/.test(e.sourceFile ?? '')) return
  setTimeout(() => {
    throw new Error(`Content-Security-Policy stopped ${e.violatedDirective}: ${e.blockedURI || 'inline'}${e.sourceFile ? ` (${e.sourceFile}:${e.lineNumber})` : ''}`)
  })
})
const scenario = win.__scenario ?? {}

// A test that answers for a method the Go API doesn't have has drifted from the backend: the
// method was renamed or removed, nothing calls the old name, and the test would go on passing
// against an answer nobody asks for. The generated bindings say what the methods are.
const SCENARIO_DATA = new Set(['File', 'Images', 'lazy'])
function checkMethods(what: string, names: string[]) {
  const unknown = names.filter((n) => !(n in bindings) && !SCENARIO_DATA.has(n))
  if (unknown.length) throw new Error(`The test's ${what} include ${unknown.join(', ')}, which the Go API (wailsjs/go/app/App) doesn't have.`)
}
checkMethods('responses', Object.keys(scenario.responses ?? {}))
checkMethods('failures', Object.keys(scenario.failures ?? {}))
checkMethods('delays', Object.keys(scenario.delays ?? {}))
checkMethods('byArg', Object.keys(scenario.byArg ?? {}))

const responses: Record<string, unknown> = { ...scenario.responses }
win.__calls = []
win.__respond = (r) => {
  checkMethods('responses', Object.keys(r))
  Object.assign(responses, r)
}
win.__changed = () => {}
win.__events = []

const graphRow = (i: number) => ({
  index: i,
  col: 0,
  color: 0,
  up: i ? [{ from: 0, to: 0, color: 0 }] : [],
  down: [{ from: 0, to: 0, color: 0 }],
  width: 1,
  id: String(i).padStart(40, '0'),
  parents: 1,
  author: 'A',
  authorTime: 1793500000 - i * 60,
  subject: `Commit ${i}`,
})

win.__graph = (commits) => {
  scenario.graphData = commits.map((n, index) => ({ ...graphRow(n), index }))
}

type Group = { name: string; paths: string[] }

// The untrusted repository's state, as the backend keeps it: refused until TrustRepo is called,
// with the question to ask handed over once after each refusal.
let trusted = false
let asking = false

// Behaviour the stub has to get right for a test to mean anything; everything else just answers.
const behaviours: Record<string, (...args: never[]) => unknown> = {
  OpenRepo: (path: string) => {
    if (scenario.untrusted?.path === path && !trusted) {
      asking = true
      throw new Error(`${scenario.untrusted.name} has git settings or hooks of its own that run programs, and you haven't said it may.`)
    }
    asking = false
    return 'OpenRepo' in responses ? structuredClone(responses.OpenRepo) : { path, name: path.split('/').pop(), head: 'main' }
  },
  TakeTrustQuestion: () => {
    const question = asking ? structuredClone(scenario.untrusted) : null
    asking = false
    return question
  },
  TrustRepo: (path: string) => {
    if (scenario.untrusted?.path === path) trusted = true
  },
  GraphRows: (_path: string, start: number, count: number) => {
    const total = scenario.graphData?.length ?? scenario.graphRows ?? 0
    return { rows: Array.from({ length: Math.max(0, Math.min(count, total - start)) }, (_, k) => scenario.graphData?.[start + k] ?? graphRow(start + k)), refs: scenario.graphRefs ?? {}, chips: scenario.graphChips, total, loading: false, paused: false, estimate: total }
  },
  // A lane is started by the first commit that wears its colour; the refs on that commit name it.
  LaneNames: (_path: string, colors: number[]) => {
    if ('LaneNames' in responses) return responses.LaneNames
    const total = scenario.graphData?.length ?? scenario.graphRows ?? 0
    type Stub = { id: string; color: number; subject: string; down: { from: number; to: number; color: number }[] }
    const rows = Array.from({ length: Math.min(total, 500) }, (_, k) => (scenario.graphData?.[k] ?? graphRow(k)) as Stub)
    return colors.map((color) => {
      const tip = rows.findIndex((r) => r.color === color)
      // Before its own first commit, a lane may have been opened by a merge on another lane.
      const opener = rows.findIndex((r) => r.color !== color && r.down.some((s) => s.color === color && s.from !== s.to))
      if (opener >= 0 && (tip < 0 || opener < tip)) return { color, row: opener, commit: rows[opener].id, refs: [], merge: rows[opener].subject }
      const commit = tip < 0 ? '' : rows[tip].id
      return { color, row: tip, commit, refs: (scenario.graphRefs?.[commit] ?? []) as unknown[] }
    })
  },
  // The stub's commits are numbered, so a commit's row is its number, unless a test says otherwise.
  CommitRow: (_path: string, id: string) => {
    if ('CommitRow' in responses) return responses.CommitRow
    const total = scenario.graphData?.length ?? scenario.graphRows ?? 0
    return /^\d{40}$/.test(id) && Number(id) < total ? Number(id) : -1
  },
  // A selection's commits are its rows' (#1018), oldest first: the highest rows first.
  SelectedCommits: (_path: string, picks: { lo: number; hi: number }[]) => {
    if ('SelectedCommits' in responses) return responses.SelectedCommits
    type Stub = { index: number; id: string; parents: number; author: string; authorTime: number; subject: string }
    const total = scenario.graphData?.length ?? scenario.graphRows ?? 0
    const rows = [...new Set(picks.flatMap((p) => Array.from({ length: p.hi - p.lo + 1 }, (_, k) => p.lo + k)))].sort((a, b) => b - a)
    const commits = rows.map((i) => {
      const r = (scenario.graphData?.[i] ?? graphRow(i)) as Stub
      return { index: i, id: r.id, parents: r.parents, author: r.author, authorTime: r.authorTime, subject: r.subject }
    })
    const authors = new Set(commits.map((c) => c.author))
    const times = commits.map((c) => c.authorTime)
    return {
      ids: commits.map((c) => c.id),
      commits: commits.slice(0, 100),
      authors: authors.size,
      author: authors.size === 1 ? commits[0].author : '',
      first: Math.min(...times),
      last: Math.max(...times),
      merges: commits.filter((c) => c.parents > 1).length,
      head: false,
      stale: false,
      // The stub draws one straight lane: each row's parent is the row below it.
      line: rows.length > 1 && rows.every((r, k) => k === 0 || r === rows[k - 1] - 1),
      base: rows[0] + 1 < total ? ((scenario.graphData?.[rows[0] + 1] ?? graphRow(rows[0] + 1)) as Stub).id : '',
    }
  },
  SelectionFiles: (_path: string, ids: string[]) => ('SelectionFiles' in responses ? responses.SelectionFiles : { files: [], total: 0, read: ids.length }),
  CompareCommits: () => ('CompareCommits' in responses ? responses.CompareCommits : { files: [], onlyFrom: 0, onlyTo: 0 }),
  SquashOffer: () => ('SquashOffer' in responses ? responses.SquashOffer : { ok: true }),
  // The stub's commits are numbered, so a commit's row is its number.
  CommitRows: (_path: string, ids: string[]) => {
    if ('CommitRows' in responses) return responses.CommitRows
    const total = scenario.graphData?.length ?? scenario.graphRows ?? 0
    return ids.map((id) => (/^\d{40}$/.test(id) && Number(id) < total ? Number(id) : -1))
  },
  // An author's folders are asked for one folder at a time: a test answers by the folder
  // asked about, '' for the top, and a folder it doesn't name has nothing under it.
  MapAuthorAreas: (_path: string, _identities: unknown, _days: number, under: string) => {
    const by = responses.MapAuthorAreas as Record<string, unknown> | undefined
    return structuredClone(by?.[under] ?? { under, areas: [], more: 0, commits: 0 })
  },
  GroupAction: (action: string, args: string[]) => {
    const groups = responses.Groups as Group[]
    if (action === 'move') {
      const [g] = groups.splice(
        groups.findIndex((x) => x.name === args[0]),
        1,
      )
      groups.splice(Number(args[1]), 0, g)
    } else if (action === 'move-repo') {
      const g = groups.find((x) => x.name === args[0])!
      g.paths.splice(g.paths.indexOf(args[1]), 1)
      g.paths.splice(Number(args[2]), 0, args[1])
    }
    return null
  },
}

win.go = {
  app: {
    App: new Proxy(
      {},
      {
        get:
          (_, method: string) =>
          async (...args: unknown[]) => {
            win.__calls.push([method, ...args])
            const delay = scenario.delays?.[method]
            if (delay) await new Promise((resolve) => setTimeout(resolve, delay))
            const failure = scenario.failures?.[method]
            if (failure) throw new Error(failure)
            if (behaviours[method]) return (behaviours[method] as (...a: unknown[]) => unknown)(...args)
            const by = scenario.byArg?.[method]
            const key = by ? String(args[by.index]) : ''
            if (by && key in by.answers) return structuredClone(by.answers[key])
            // A copy, as the real backend would send, so a component can't edit the stub's state.
            return method in responses ? structuredClone(responses[method]) : null
          },
      },
    ),
  },
}
const listeners = new Map<string, ((data: unknown) => void)[]>()
win.__emit = (name, data) => listeners.get(name)?.forEach((listen) => listen(data))
const EventsOnMultiple = (name: string, listen: (data: unknown) => void) => {
  listeners.set(name, [...(listeners.get(name) ?? []), listen])
  return () => listeners.set(name, listeners.get(name)!.filter((l) => l !== listen))
}
const BrowserOpenURL = (url: string) => void win.__events.push(['open-url', url])
const ClipboardSetText = async (text: string) => (win.__events.push(['copy', text]), true)
// As main.ts does for the app.
guardLinks(window, BrowserOpenURL)
win.runtime = new Proxy({ EventsOnMultiple, BrowserOpenURL, ClipboardSetText } as Record<string, unknown>, { get: (o, k: string) => o[k] ?? (() => {}) })

const record =
  (name: string) =>
  (...args: unknown[]) =>
    win.__events.push([name, ...args.map((a) => (a && typeof a === 'object' && 'short' in a ? (a as { short: string }).short : a))])

const host = document.getElementById('host')!
document.documentElement.dataset.theme = scenario.theme ?? 'light'
document.body.style.cssText = 'margin:0;height:100vh;background:var(--paper)'
host.style.cssText = 'display:flex;height:100vh'

const mounts = {
  async lazy() {
    const { default: Lazy } = await import('../src/lib/ui/Lazy.svelte')
    const { default: Icon } = await import('../src/lib/ui/Icon.svelte')
    const load = scenario.failures?.lazy ? () => Promise.reject(new Error(scenario.failures!.lazy)) : async () => ({ default: Icon })
    mount(Lazy, { target: host, props: { load, name: 'search' } })
  },
  async lazyDiff() {
    const { default: Lazy } = await import('../src/lib/ui/Lazy.svelte')
    const target = { mode: 'commit', commit: 'a'.repeat(40), file: 'src/lazy-file.ts' }
    mount(Lazy, { target: host, props: { load: () => import('../src/lib/diff/DiffView.svelte'), path: '/repo', target, onClose: record('close'), onOpen: record('open') } })
  },
  async blame() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: BlameView } = await import('../src/lib/diff/BlameView.svelte')
    // [ and ] step along the track (#1078), as the app's own key handler runs them.
    const { commands } = await import('../src/lib/commands.svelte')
    window.addEventListener('keydown', (e) => commands.dispatch(e))
    mount(BlameView, { target: host, props: { path: '/r', file: 'a.txt', writeAs: scenario.blameWriteAs ?? '', onClose: record('close'), onOpen: record('open'), onReveal: record('reveal') } })
  },
  async math() {
    const { default: MathView } = await import('../src/lib/markdown/MathView.svelte')
    mount(MathView, { target: host, props: { tex: 'x^2' } })
  },
  async loading() {
    const { default: Loading } = await import('../src/lib/ui/Loading.svelte')
    mount(Loading, { target: host, props: {} })
  },
  async sidebar() {
    const { default: Sidebar } = await import('../src/lib/Sidebar.svelte')
    if (scenario.commandKeys) {
      const { commands } = await import('../src/lib/commands.svelte')
      window.addEventListener('keydown', (e) => commands.dispatch(e))
    }
    mount(Sidebar, {
      target: host,
      props: {
        path: '/repo',
        selectedCommit: scenario.selectedCommit,
        onPick: record('pick'),
        onActivate: record('checkout'),
        onNewBranch: record('new-branch'),
        onManageRemotes: record('manage-remotes'),
        onManageWorktrees: record('manage-worktrees'),
        onContext: record('context'),
        onMenu: record('menu'),
        openDialog: record('dialog'),
        filterPicks: scenario.filterPicks ?? [],
        onFilterPick: record('filter'),
        pinnedKeys: scenario.pinnedKeys ?? [],
        onPin: record('pin'),
        onSearch: record('search'),
        diffShown: (scenario.diffShown ?? null) as DiffTarget | null,
        onOpenFile: record('open-file'),
        onHistorySearch: record('history-search'),
      },
    })
  },
  // A file Find in files found something in (#980).
  async foundFile() {
    if (scenario.commandKeys) {
      const { commands } = await import('../src/lib/commands.svelte')
      window.addEventListener('keydown', (e) => commands.dispatch(e))
    }
    const { default: FoundFileView } = await import('../src/lib/diff/FoundFileView.svelte')
    mount(FoundFileView, { target: host, props: { path: '/repo', target: scenario.foundTarget as DiffTarget, onClose: record('close'), onOpen: record('open') } })
  },
  async start() {
    host.style.cssText = 'display:block'
    const { default: StartPage } = await import('../src/lib/StartPage.svelte')
    mount(StartPage, { target: host, props: {} })
  },
  async graph() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: GraphView } = await import('../src/lib/graph/GraphView.svelte')
    // A test about the saved view has the app read it first, as the app does when it starts.
    if (scenario.responses?.GraphViews) await (await import('../src/lib/theme.svelte')).theme.load()
    const wip = scenario.wip ? { changes: scenario.wip.changes, active: false, open: record('wip') } : undefined
    mount(GraphView, { target: host, props: { path: '/repo', wip, onSelect: record('select'), onSelection: (s: { count: number } | null) => win.__events.push(['selection', s?.count ?? 0]), onNote: (row: { index: number }) => win.__events.push(['note', row.index]), onContext: (row: { index: number }) => win.__events.push(['context-row', row.index]), onDrop: record('drop'), onFileHistory: record('file-history'), ...(scenario.suggests ? { onFilterAdd: record('filter-add'), onFilterDrop: record('filter-drop') } : {}), ...(scenario.finds ? { onFindInFiles: record('find-in-files') } : {}), ...(scenario.histories ? { onHistorySearch: record('history-search') } : {}), history: (scenario.graphHistory ?? null) as never, sparse: scenario.sparse ?? false, highlight: scenario.highlight ?? null, filterPicks: scenario.filterPicks ?? [], pins: scenario.pins ?? [], ...(scenario.graphChips ? { onFilterPick: record('filter-pick') } : {}), trailersUnread: scenario.trailersUnread ?? 0 } })
  },
  async commitMenu() {
    const { default: ContextMenu } = await import('../src/lib/ContextMenu.svelte')
    const { commitMenuItems } = await import('../src/lib/menus')
    const refs = (scenario.responses?.Refs ?? []) as never[]
    const ctx = { path: '/r', current: () => scenario.current ?? 'main', openDialog: record('dialog'), openRebase: record('rebase'), openPreview: record('preview'), openReflog: record('reflog') }
    const row = { ...graphRow(0), id: 'a'.repeat(40) } as never
    const items = commitMenuItems(row, refs, ctx)
    const { session } = await import('../src/lib/session.svelte')
    Object.assign(win, { __sessionError: () => session.error })
    mount(ContextMenu, { target: host, props: { x: scenario.menuX ?? 200, y: 100, items, onClose: record('close') } })
  },
  async applyPatches() {
    const { default: ApplyPatchesDialog } = await import('../src/lib/ApplyPatchesDialog.svelte')
    mount(ApplyPatchesDialog, { target: host, props: { path: '/r', file: scenario.patchFile as never, onClose: record('close') } })
    const { default: Toasts } = await import('../src/lib/ui/Toasts.svelte')
    mount(Toasts, { target: document.body })
  },
  async applyDiff() {
    const { default: ApplyDiffDialog } = await import('../src/lib/ApplyDiffDialog.svelte')
    mount(ApplyDiffDialog, { target: host, props: { path: '/r', file: scenario.patchFile as never, onClose: record('close') } })
    const { default: Toasts } = await import('../src/lib/ui/Toasts.svelte')
    mount(Toasts, { target: document.body })
  },
  async savePatches() {
    const { default: SavePatchesDialog } = await import('../src/lib/SavePatchesDialog.svelte')
    mount(SavePatchesDialog, { target: host, props: { path: '/r', start: scenario.patchStart as never, onClose: record('close') } })
    const { default: Toasts } = await import('../src/lib/ui/Toasts.svelte')
    mount(Toasts, { target: document.body })
  },
  async archive() {
    const { default: ArchiveDialog } = await import('../src/lib/ArchiveDialog.svelte')
    const what = (scenario.archive ?? { rev: 'v1.1.0', label: 'v1.1.0' }) as { rev: string; label: string }
    mount(ArchiveDialog, { target: host, props: { path: 'A:\\tidewater', rev: what.rev, label: what.label, onClose: record('close') } })
    const { default: Toasts } = await import('../src/lib/ui/Toasts.svelte')
    mount(Toasts, { target: document.body })
  },
  async rebaseEditor() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: RebaseEditor } = await import('../src/lib/RebaseEditor.svelte')
    mount(RebaseEditor, { target: host, props: { path: '/r', base: 'b'.repeat(40), onClose: record('close'), onDone: record('done') } })
  },
  async markdownPreview() {
    const { previewerFor } = await import('../src/lib/diff/previews/registry')
    const { path, text, data } = scenario.responses?.File as { path: string; text: string; data?: string }
    const component = previewerFor(path)!.component
    // A binary previewer reads the file's bytes, given here as base64.
    const loadData = async () => data ?? ''
    // Pictures in the repository, by path, as data URLs.
    const images = (scenario.responses?.Images ?? {}) as Record<string, string>
    mount(component, { target: host, props: { path, text, loadImage: async (file: string) => images[file] ?? '', openFile: record('openFile'), loadData } })
  },
  async cloneDialog() {
    const { default: CloneDialog } = await import('../src/lib/CloneDialog.svelte')
    // Read the saved settings first, as the app does when it starts.
    await (await import('../src/lib/prefs.svelte')).prefs.load()
    mount(CloneDialog, { target: host, props: { onClose: record('close') } })
  },
  async upstreamDialog() {
    const { default: Dialog } = await import('../src/lib/Dialog.svelte')
    const { upstreamDialog } = await import('../src/lib/menus')
    const refs = (scenario.responses?.Refs ?? []) as { kind: string; short: string }[]
    const ref = refs.find((r) => r.kind === 'local') as never
    mount(Dialog, { target: host, props: { spec: upstreamDialog('/r', ref, refs as never), onClose: record('close') } })
  },
  async workingTree() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: WorkingTreePanel } = await import('../src/lib/WorkingTreePanel.svelte')
    if (scenario.commandKeys) {
      const { commands } = await import('../src/lib/commands.svelte')
      window.addEventListener('keydown', (e) => commands.dispatch(e))
    }
    mount(WorkingTreePanel, { target: host, props: { path: '/r', openFile: record('open'), ...(scenario.diffFile ? { current: { mode: 'unstaged', file: scenario.diffFile } } : {}), onCommitted: record('committed'), onNewBranch: () => win.__events.push(['new-branch']), ...(scenario.head ? { head: scenario.head } : {}) } })
  },
  async repoView() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: RepoView } = await import('../src/lib/RepoView.svelte')
    mount(RepoView, { target: host, props: { path: '/r' } })
    // What an author's card asks the People tab for, which the app shell would then show.
    const { session } = await import('../src/lib/session.svelte')
    session.tabs = [{ path: '/r', name: 'tidewater', head: scenario.current ?? 'main' } as never]
    // A scenario that names the checked-out branch has the tab open, so the menus name it.
    if (scenario.current) session.active = '/r'
    const { peopleTab } = await import('../src/lib/people/tab.svelte')
    Object.assign(win, { __peopleTab: () => ({ open: peopleTab.open, repo: peopleTab.repo, ask: peopleTab.ask && JSON.parse(JSON.stringify(peopleTab.ask)) }) })
    // Runs a command by its id, as picking it in the palette does.
    const { commands } = await import('../src/lib/commands.svelte')
    Object.assign(win, { __runCommand: (id: string) => commands.all().find((c) => c.id === id)?.run() })
  },
  // Patches from a dropped file or the clipboard (#1025): the repository view on screen, with
  // the drop overlay and toasts. scenario.tabs 'none' has no repository open, 'other' has one
  // that isn't on screen.
  async patchSources() {
    await mounts.repoView()
    const { session } = await import('../src/lib/session.svelte')
    session.active = '/r'
    if (scenario.tabs === 'none') session.tabs = []
    if (scenario.tabs === 'none' || scenario.tabs === 'other') session.active = 'gitrevenge:gitconfig'
    const { default: Toasts } = await import('../src/lib/ui/Toasts.svelte')
    mount(Toasts, { target: document.body })
    const { default: DropOverlay } = await import('../src/lib/DropOverlay.svelte')
    mount(DropOverlay, { target: document.body })
  },
  // Trailer care, the dialog where a repository says how its trailers are read.
  async trailerCare() {
    const { default: TrailerCare } = await import('../src/lib/filters/TrailerCare.svelte')
    const { session } = await import('../src/lib/session.svelte')
    session.tabs = [{ path: '/r', name: 'tidewater', head: 'main' } as never]
    mount(TrailerCare, { target: host, props: { repo: '/r', onClose: record('close') } })
  },
  // The strip above the graph while a smart filter is on.
  async filterStrip() {
    const { default: FilterStrip } = await import('../src/lib/filters/FilterStrip.svelte')
    const s: NonNullable<Scenario['filterStrip']> = scenario.filterStrip ?? { filter: null }
    mount(FilterStrip, {
      target: host,
      props: {
        filter: s.filter as never,
        only: s.only ?? true,
        onlyOffered: s.onlyOffered ?? true,
        onRemove: record('remove'),
        onAdd: record('add'),
        onMode: record('mode'),
        onClear: record('clear'),
        onStep: record('step'),
        onHistorySearch: record('history-search'),
        onHistoryStop: record('history-stop'),
        onHistoryCarryOn: record('history-carry-on'),
        onHistoryRemove: record('history-remove'),
      },
    })
  },
  // The status bar's line about reading trailers, for the repository on screen.
  async trailerProgress() {
    const { default: TrailerProgress } = await import('../src/lib/TrailerProgress.svelte')
    mount(TrailerProgress, { target: host, props: { path: '/r' } })
  },
  // The People tab, opened on a repository as the app shell opens it.
  async people() {
    host.style.cssText = 'display:block;height:100vh'
    const { session } = await import('../src/lib/session.svelte')
    session.tabs = [{ path: '/r', name: 'tidewater', head: 'main' } as never]
    const { peopleTab } = await import('../src/lib/people/tab.svelte')
    const { default: PeopleView } = await import('../src/lib/people/PeopleView.svelte')
    peopleTab.open = true
    peopleTab.wanted = '/r'
    // Someone an author's card in the graph asked for.
    if (scenario.peopleAsk) peopleTab.ask = scenario.peopleAsk as never
    mount(PeopleView, { target: host })
    // What the tab bar's close button does, and what the tab bar reads to mark an edit.
    Object.assign(win, { __closePeople: () => peopleTab.requestClose(), __peopleTab: () => ({ open: peopleTab.open, dirty: peopleTab.dirty }) })
  },
  // The repository view under the operation strip, as the app shell stacks them.
  async bisect() {
    host.style.cssText = 'display:flex;flex-direction:column;height:100vh'
    const { default: OperationBanner } = await import('../src/lib/OperationBanner.svelte')
    const { default: RepoView } = await import('../src/lib/RepoView.svelte')
    mount(OperationBanner, { target: host, props: { path: '/r' } })
    const rest = document.createElement('div')
    rest.style.cssText = 'flex:1;min-height:0'
    host.append(rest)
    mount(RepoView, { target: rest, props: { path: '/r' } })
    // The shell also turns key presses into commands.
    const { commands } = await import('../src/lib/commands.svelte')
    window.addEventListener('keydown', (e) => void commands.dispatch(e))
  },
  async previewDialog() {
    const { default: PreviewDialog } = await import('../src/lib/PreviewDialog.svelte')
    // As in the app, closing takes the dialog's props away: it must not read them afterwards.
    let closed = false
    const until = <T,>(v: T) => (closed ? undefined : v) as T
    const close = record('close')
    const props = {
      get path() { return until('/r') },
      get kind() { return until(scenario.previewKind ?? 'merge') },
      get target() { return until('feature/metric-units') },
      get label() { return until('feature/metric-units') },
      get current() { return until('main') },
      onClose: () => { closed = true; close() },
      onEdit: record('edit'),
    }
    const { session } = await import('../src/lib/session.svelte')
    Object.assign(win, { __sessionError: () => session.error })
    mount(PreviewDialog, { target: host, props })
  },
  // The operation strip over the repository view, with the toasts it raises.
  async operation() {
    await mounts.bisect()
    const { default: Toasts } = await import('../src/lib/ui/Toasts.svelte')
    mount(Toasts, { target: document.body })
    const { session } = await import('../src/lib/session.svelte')
    win.__changed = (path) => (session.revision[path] = (session.revision[path] ?? 0) + 1)
    // What the strip's toasts asked to compare (#1040): the repository view takes it in the app.
    const { versionsAsk } = await import('../src/lib/versionsAsk.svelte')
    Object.assign(win, { __versionsAsk: () => (versionsAsk.ask ? { ...versionsAsk.ask } : null) })
  },
  async conflictView() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: ConflictView } = await import('../src/lib/diff/ConflictView.svelte')
    const { session } = await import('../src/lib/session.svelte')
    win.__changed = (path) => (session.revision[path] = (session.revision[path] ?? 0) + 1)
    Object.assign(win, { __sessionError: () => session.error })
    mount(ConflictView, { target: host, props: { path: '/r', file: scenario.diffFile ?? 'src/feed.js', onClose: record('close'), onOpen: record('open') } })
  },
  async worktreesDialog() {
    const { default: WorktreesDialog } = await import('../src/lib/WorktreesDialog.svelte')
    mount(WorktreesDialog, { target: host, props: { path: '/r', onClose: () => {} } })
  },
  async settings() {
    const { default: SettingsDialog } = await import('../src/lib/SettingsDialog.svelte')
    // A scenario that answers for a repository's preferences has that repository open, so the
    // dialog shows its section.
    if (responses.RepoPreferences) {
      const { session } = await import('../src/lib/session.svelte')
      session.tabs = [{ path: '/r', name: 'tidewater' } as (typeof session.tabs)[number]]
      session.active = '/r'
    }
    mount(SettingsDialog, { target: host, props: { onClose: record('close') } })
  },
  // Opening a repository the app refuses for want of trust, then the dialog that asks.
  async trustDialog() {
    const { default: TrustDialog } = await import('../src/lib/TrustDialog.svelte')
    const { session } = await import('../src/lib/session.svelte')
    mount(TrustDialog, { target: host })
    Object.assign(win, { __open: (path: string) => session.open(path), __tabs: () => session.tabs.map((t) => t.path), __problem: () => session.problem?.text ?? '' })
  },
  async about() {
    const { default: AboutDialog } = await import('../src/lib/about/AboutDialog.svelte')
    mount(AboutDialog, { target: host, props: { version: '0.11.0', onClose: record('close') } })
  },
  async errorBanner() {
    const { mount: show, unmount } = await import('svelte')
    const { default: ErrorBanner } = await import('../src/lib/ErrorBanner.svelte')
    const { default: Toolbar } = await import('../src/lib/Toolbar.svelte')
    const { session } = await import('../src/lib/session.svelte')
    host.style.cssText = 'display:block'
    const banner = host.appendChild(document.createElement('div'))
    session.tabs = [{ path: '/r', name: 'r', head: 'main' } as never]
    session.active = '/r'
    // The toolbar's commands are how the banner pulls and pushes.
    if (scenario.toolbar) mount(Toolbar, { target: host.appendChild(document.createElement('div')), props: { path: '/r' } })
    // As App does: the banner shows whatever the session last failed at, and is made again
    // when that changes.
    let shown: Record<string, never> | null = null
    let last: unknown = null
    setInterval(() => {
      if (session.problem === last) return
      last = session.problem
      if (shown) void unmount(shown)
      shown = session.problem ? show(ErrorBanner, { target: banner, props: { problem: session.problem } }) : null
    }, 20)
    const app = (win.go as { app: { App: Record<string, (...a: unknown[]) => Promise<unknown>> } }).app.App
    Object.assign(win, {
      __sessionError: () => session.error,
      /** Makes a backend call through the session, as a command would, so a failure lands in the banner. */
      __call: (method: string, ...args: unknown[]) => session.run(() => app[method](...args)),
      __say: (text: string) => void (session.error = text),
    })
  },
  async repoPanel() {
    const { default: RepoPanel } = await import('../src/lib/RepoPanel.svelte')
    // Dialogs and menus as the app shows them, loaded when first asked for.
    const openDialog = async (spec: unknown) => {
      const { default: Dialog } = await import('../src/lib/Dialog.svelte')
      const d = mount(Dialog, { target: host, props: { spec: spec as never, onClose: () => void unmount(d) } })
    }
    const onMenu = async (items: unknown[], x: number, y: number) => {
      const { default: ContextMenu } = await import('../src/lib/ContextMenu.svelte')
      const m = mount(ContextMenu, { target: host, props: { x, y, items: items as never, onClose: () => void unmount(m) } })
    }
    mount(RepoPanel, { target: host, props: { path: '/r', filter: '', openDialog: (s) => void openDialog(s), onMenu: (i, x, y) => void onMenu(i, x, y) } })
    // The toolbar shows what the panel's This clone section runs (#1005). Not waited for: the
    // panel is ready before it, as tests of its first answers need.
    void import('../src/lib/Toolbar.svelte').then(({ default: Toolbar }) => mount(Toolbar, { target: host, props: { path: '/r' } }))
  },
  async diff() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: DiffView } = await import('../src/lib/diff/DiffView.svelte')
    const { prefs } = await import('../src/lib/prefs.svelte')
    await prefs.load()
    if (scenario.commandKeys) {
      const { commands } = await import('../src/lib/commands.svelte')
      window.addEventListener('keydown', (e) => commands.dispatch(e))
    }
    const target = scenario.diffTarget ?? { mode: 'commit', commit: 'c0ffee'.padEnd(40, '0'), file: 'internal/tides/forecast.go' }
    mount(DiffView, { target: host, props: { path: '/r', target, onClose: record('close'), onOpen: record('open') } })
  },
  // A file's history, as the diff view's History opens it.
  async fileHistory() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: FileHistoryView } = await import('../src/lib/diff/FileHistoryView.svelte')
    mount(FileHistoryView, { target: host, props: { path: '/r', file: 'new.txt', onClose: record('close'), onOpen: record('open') } })
  },
  async closingDiff() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: DiffView } = await import('../src/lib/diff/DiffView.svelte')
    const { session } = await import('../src/lib/session.svelte')
    const state = win as unknown as { __diffTarget: unknown; __sessionError: () => string }
    state.__diffTarget = { mode: 'unstaged', file: 'a.txt' }
    state.__sessionError = () => session.error
    mount(DiffView, { target: host, props: { path: '/r', onClose: record('close'), onOpen: record('open'), get target() { return state.__diffTarget } } as never })
  },
  async stackedDialogs() {
    const { default: Dialog } = await import('../src/lib/Dialog.svelte')
    const spec = (title: string) => ({ title, submit: 'OK', run: async () => {} })
    mount(Dialog, { target: host, props: { spec: spec('Below'), onClose: record('close-below') } })
    mount(Dialog, { target: host, props: { spec: spec('Above'), onClose: record('close-above') } })
  },
  // The Insights tab, opened the way a repository's tab menu and the command palette open it.
  async insights() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: InsightsView } = await import('../src/lib/insights/InsightsView.svelte')
    const { insights } = await import('../src/lib/insights/insights.svelte')
    const { session } = await import('../src/lib/session.svelte')
    session.tabs = [{ path: '/r', name: 'tidewater', head: 'main' } as never]
    session.active = '/r'
    mount(InsightsView, { target: host })
    await insights.show()
  },
  // The whole shell, for what happens between its tabs. It restores its session from the stub.
  async app() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: App } = await import('../src/App.svelte')
    const { session } = await import('../src/lib/session.svelte')
    mount(App, { target: host })
    Object.assign(win, { __select: (id: string) => session.select(id), __active: () => session.active })
  },
}

switch (new URLSearchParams(location.search).get('mount')) {
  case 'about':
    await mounts.about()
    break
  case 'trustDialog':
    await mounts.trustDialog()
    break
  case 'settings':
    await mounts.settings()
    break
  case 'errorBanner':
    await mounts.errorBanner()
    break
  case 'lazy':
    await mounts.lazy()
    break
  case 'lazyDiff':
    await mounts.lazyDiff()
    break
  case 'blame':
    await mounts.blame()
    break
  case 'math':
    await mounts.math()
    break
  case 'loading':
    await mounts.loading()
    break
  case 'sidebar':
    await mounts.sidebar()
    break
  case 'foundFile':
    await mounts.foundFile()
    break
  case 'start':
    await mounts.start()
    break
  case 'graph':
    await mounts.graph()
    break
  case 'previewDialog':
    await mounts.previewDialog()
    break
  case 'commitMenu':
    await mounts.commitMenu()
    break
  case 'applyPatches':
    await mounts.applyPatches()
    break
  case 'applyDiff':
    await mounts.applyDiff()
    break
  case 'savePatches':
    await mounts.savePatches()
    break
  case 'archive':
    await mounts.archive()
    break
  case 'rebaseEditor':
    await mounts.rebaseEditor()
    break
  case 'markdownPreview':
    await mounts.markdownPreview()
    break
  case 'upstreamDialog':
    await mounts.upstreamDialog()
    break
  case 'cloneDialog':
    await mounts.cloneDialog()
    break
  case 'workingTree':
    await mounts.workingTree()
    break
  case 'repoView':
    await mounts.repoView()
    break
  case 'patchSources':
    await mounts.patchSources()
    break
  case 'people':
    await mounts.people()
    break
  case 'trailerProgress':
    await mounts.trailerProgress()
    break
  case 'filterStrip':
    await mounts.filterStrip()
    break
  case 'trailerCare':
    await mounts.trailerCare()
    break
  case 'conflictView':
    await mounts.conflictView()
    break
  case 'operation':
    await mounts.operation()
    break
  case 'bisect':
    await mounts.bisect()
    break
  case 'worktreesDialog':
    await mounts.worktreesDialog()
    break
  case 'repoPanel':
    await mounts.repoPanel()
    break
  case 'closingDiff':
    await mounts.closingDiff()
    break
  case 'diff':
    await mounts.diff()
    break
  case 'stackedDialogs':
    await mounts.stackedDialogs()
    break
  case 'app':
    await mounts.app()
    break
  case 'insights':
    await mounts.insights()
    break
  case 'fileHistory':
    await mounts.fileHistory()
    break
  default:
    throw new Error('Unknown component to mount.')
}
win.__ready = true
