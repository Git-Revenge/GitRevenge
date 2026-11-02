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
    go: unknown
    runtime: unknown
  }

const win = window as Harness
const scenario = win.__scenario ?? {}
const responses: Record<string, unknown> = { ...scenario.responses }
win.__calls = []
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
    const total = scenario.graphRows ?? 0
    return { rows: Array.from({ length: Math.max(0, Math.min(count, total - start)) }, (_, k) => graphRow(start + k)), refs: scenario.graphRefs ?? {}, total, loading: false, paused: false, estimate: total }
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
win.runtime = new Proxy({ EventsOnMultiple } as Record<string, unknown>, { get: (o, k: string) => o[k] ?? (() => {}) })

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
    mount(GraphView, { target: host, props: { path: '/repo', onSelect: record('select'), onContext: (row: { index: number }) => win.__events.push(['context-row', row.index]), onDrop: record('drop') } })
  },
  async commitMenu() {
    const { default: ContextMenu } = await import('../src/lib/ContextMenu.svelte')
    const { commitMenuItems } = await import('../src/lib/menus')
    const refs = (scenario.responses?.Refs ?? []) as never[]
    const ctx = { path: '/r', current: () => scenario.current ?? 'main', openDialog: record('dialog'), openRebase: record('rebase'), openReflog: record('reflog') }
    const row = { ...graphRow(0), id: 'a'.repeat(40) } as never
    const items = commitMenuItems(row, refs, ctx)
    mount(ContextMenu, { target: host, props: { x: scenario.menuX ?? 200, y: 100, items, onClose: record('close') } })
  },
  async rebaseEditor() {
    host.style.cssText = 'display:block;height:100vh'
    const { default: RebaseEditor } = await import('../src/lib/RebaseEditor.svelte')
    mount(RebaseEditor, { target: host, props: { path: '/r', base: 'b'.repeat(40), onClose: record('close') } })
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
    mount(WorkingTreePanel, { target: host, props: { path: '/r', openFile: () => {}, onCommitted: record('committed') } })
  },
  async repoView() {
    const { default: RepoView } = await import('../src/lib/RepoView.svelte')
    mount(RepoView, { target: host, props: { path: '/r' } })
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
  case 'lazy':
    await mounts.lazy()
    break
  case 'lazyDiff':
    await mounts.lazyDiff()
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
  case 'commitMenu':
    await mounts.commitMenu()
    break
  case 'rebaseEditor':
    await mounts.rebaseEditor()
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
