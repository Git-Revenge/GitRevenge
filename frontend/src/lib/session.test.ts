import { beforeEach, describe, expect, it, vi } from 'vitest'

// The backend isn't here: the bindings answer from `api`, and `fire` sends its events.
const { api, listeners } = vi.hoisted(() => ({
  api: {
    ChooseAndOpenRepo: vi.fn(),
    CloseRepo: vi.fn(),
    OpenRepo: vi.fn(),
    RecentRepos: vi.fn(),
    Refs: vi.fn(),
    RemoveRecentRepo: vi.fn(),
    RepoInfo: vi.fn(),
    RestoreSession: vi.fn(),
    SetActiveRepo: vi.fn(),
    TakeProblem: vi.fn(),
    TakeTrustQuestion: vi.fn(),
    TrustRepo: vi.fn(),
    WorkingTree: vi.fn(),
  },
  listeners: new Map<string, ((data: unknown) => void)[]>(),
}))

vi.mock('../../wailsjs/go/app/App.js', () => api)
vi.mock('../../wailsjs/runtime/runtime.js', () => ({
  EventsOn: (name: string, listen: (data: unknown) => void) => {
    listeners.set(name, [...(listeners.get(name) ?? []), listen])
    return () => {}
  },
}))

const fire = (name: string, data: unknown) => listeners.get(name)?.forEach((listen) => listen(data))
const tab = (path: string, head = 'main') => ({ path, name: path.slice(1), head })
/** Lets everything that's ready to run, run. */
const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

let session: (typeof import('./session.svelte'))['session']

beforeEach(async () => {
  // The session is one object for the whole app, so each test loads the module afresh.
  vi.resetModules()
  listeners.clear()
  for (const fn of Object.values(api)) fn.mockReset().mockResolvedValue(undefined)
  api.RecentRepos.mockResolvedValue([])
  api.TakeProblem.mockResolvedValue(null)
  api.RestoreSession.mockResolvedValue({ tabs: [tab('/a'), tab('/b'), tab('/c')], active: '/c', failed: [], configTab: '', configTabId: 'tab:config', insightsTab: '', insightsTabId: 'tab:insights', ignoreTab: '', ignoreTabId: 'tab:ignore' })
  ;({ session } = await import('./session.svelte'))
  await session.restore()
})

describe('a change that moves a branch', () => {
  it('reads the tab’s label again', async () => {
    api.RepoInfo.mockResolvedValue(tab('/b', 'feature'))
    fire('repo:changed', { path: '/b', kinds: ['head'] })
    await settle()
    expect(session.tabs.map((t) => t.head)).toEqual(['main', 'feature', 'main'])
  })

  it('leaves the label alone for a change that can’t have moved one', async () => {
    fire('repo:changed', { path: '/b', kinds: ['worktree', 'index'] })
    await settle()
    expect(api.RepoInfo).not.toHaveBeenCalled()
  })

  // The answer used to go to the place the tab had when it was asked for. A tab closed in the
  // meantime moves the ones after it, so the answer landed on a neighbour, or past the end.
  it('puts the label on its own tab when another closes while it’s being read', async () => {
    const reading = Promise.withResolvers<ReturnType<typeof tab>>()
    api.RepoInfo.mockReturnValue(reading.promise)
    fire('repo:changed', { path: '/c', kinds: ['refs'] })
    await settle()
    await session.close('/a')
    reading.resolve(tab('/c', 'feature'))
    await settle()
    expect(session.tabs).toEqual([tab('/b'), tab('/c', 'feature')])
  })

  it('drops the label of a tab that closed while it was being read', async () => {
    const reading = Promise.withResolvers<ReturnType<typeof tab>>()
    api.RepoInfo.mockImplementation((path: string) => (path === '/c' ? reading.promise : Promise.resolve(tab(path))))
    fire('repo:changed', { path: '/c', kinds: ['refs'] })
    await settle()
    await session.close('/c')
    reading.resolve(tab('/c', 'feature'))
    await settle()
    expect(session.tabs).toEqual([tab('/a'), tab('/b')])
  })
})
