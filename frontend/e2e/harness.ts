// Mounts one real component in a real browser, against a stubbed Go backend. A test picks the
// component with ?mount= and supplies the backend's answers through window.__scenario. It isn't
// part of the app's build: only index.html is an entry there.
import { mount } from 'svelte'
import '../src/style.css'
import type { Scenario } from './support'

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
    /** Tells the app the repository changed on disk, as the watcher would. */
    __changed: (path: string) => void
    go: unknown
    runtime: unknown
  }

const win = window as Harness
const scenario = win.__scenario ?? {}
const responses: Record<string, unknown> = { ...scenario.responses }
win.__calls = []
win.__respond = (r) => void Object.assign(responses, r)
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

type Group = { name: string; paths: string[] }

// Behaviour the stub has to get right for a test to mean anything; everything else just answers.
const behaviours: Record<string, (...args: never[]) => unknown> = {
  GraphRows: (_path: string, start: number, count: number) => {
    const total = scenario.graphData?.length ?? scenario.graphRows ?? 0
    return { rows: Array.from({ length: Math.max(0, Math.min(count, total - start)) }, (_, k) => scenario.graphData?.[start + k] ?? graphRow(start + k)), refs: scenario.graphRefs ?? {}, total, loading: false, paused: false, estimate: total }
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
    mount(BlameView, { target: host, props: { path: '/r', file: 'a.txt', onClose: record('close'), onOpen: record('open'), onReveal: record('reveal') } })
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
      },
    })
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
    mount(GraphView, { target: host, props: { path: '/repo', wip, onSelect: record('select'), onNote: (row: { index: number }) => win.__events.push(['note', row.index]), onContext: (row: { index: number }) => win.__events.push(['context-row', row.index]), onDrop: record('drop') } })
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
  async rebaseEditor() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: RebaseEditor } = await import('../src/lib/RebaseEditor.svelte')
    mount(RebaseEditor, { target: host, props: { path: '/r', base: 'b'.repeat(40), onClose: record('close') } })
  },
  async markdownPreview() {
    const { previewerFor } = await import('../src/lib/diff/previews/registry')
    const { path, text, data } = scenario.responses?.File as { path: string; text: string; data?: string }
    const component = previewerFor(path)!.component
    // A binary previewer reads the file's bytes, given here as base64.
    const loadData = async () => data ?? ''
    mount(component, { target: host, props: { path, text, loadImage: async () => '', openFile: record('openFile'), loadData } })
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
    mount(SettingsDialog, { target: host, props: { onClose: record('close') } })
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
    mount(RepoPanel, { target: host, props: { path: '/r', filter: '', openDialog: () => {} } })
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
}

switch (new URLSearchParams(location.search).get('mount')) {
  case 'about':
    await mounts.about()
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
  case 'rebaseEditor':
    await mounts.rebaseEditor()
    break
  case 'markdownPreview':
    await mounts.markdownPreview()
    break
  case 'upstreamDialog':
    await mounts.upstreamDialog()
    break
  case 'workingTree':
    await mounts.workingTree()
    break
  case 'repoView':
    await mounts.repoView()
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
  case 'stackedDialogs':
    await mounts.stackedDialogs()
    break
  default:
    throw new Error('Unknown component to mount.')
}
win.__ready = true
