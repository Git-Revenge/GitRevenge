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
    return { rows: Array.from({ length: Math.max(0, Math.min(count, total - start)) }, (_, k) => graphRow(start + k)), refs: {}, total, loading: false, paused: false, estimate: total }
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
win.runtime = new Proxy({ EventsOnMultiple: () => () => {} } as Record<string, unknown>, { get: (o, k: string) => o[k] ?? (() => {}) })

const record =
  (name: string) =>
  (...args: unknown[]) =>
    win.__events.push([name, ...args.map((a) => (a && typeof a === 'object' && 'short' in a ? (a as { short: string }).short : a))])

const host = document.getElementById('host')!
document.documentElement.dataset.theme = scenario.theme ?? 'light'
document.body.style.cssText = 'margin:0;height:100vh;background:var(--paper)'
host.style.cssText = 'display:flex;height:100vh'

const mounts = {
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
  async upstreamDialog() {
    const { default: Dialog } = await import('../src/lib/Dialog.svelte')
    const { upstreamDialog } = await import('../src/lib/menus')
    const refs = (scenario.responses?.Refs ?? []) as { kind: string; short: string }[]
    const ref = refs.find((r) => r.kind === 'local') as never
    mount(Dialog, { target: host, props: { spec: upstreamDialog('/r', ref, refs as never), onClose: record('close') } })
  },
}

switch (new URLSearchParams(location.search).get('mount')) {
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
  case 'upstreamDialog':
    await mounts.upstreamDialog()
    break
  default:
    throw new Error('Unknown component to mount.')
}
win.__ready = true
