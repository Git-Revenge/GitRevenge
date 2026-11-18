import { beforeEach, describe, expect, it, vi } from 'vitest'

// The backend isn't here: the bindings answer from `api`.
const { api } = vi.hoisted(() => ({
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
}))

vi.mock('../../wailsjs/go/app/App.js', () => api)
vi.mock('../../wailsjs/runtime/runtime.js', () => ({ EventsOn: () => () => {} }))

const tab = (path: string, head = 'main') => ({ path, name: path.slice(1), head })
/** Lets everything that's ready to run, run. */
const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

let session: (typeof import('./session.svelte'))['session']

beforeEach(async () => {
  vi.resetModules()
  for (const fn of Object.values(api)) fn.mockReset().mockResolvedValue(undefined)
  api.RecentRepos.mockResolvedValue([])
  api.TakeProblem.mockResolvedValue(null)
  api.RepoInfo.mockImplementation(async (path: string) => tab(path))
  api.RestoreSession.mockResolvedValue({ tabs: [tab('/a'), tab('/b'), tab('/c')], active: '/a', failed: [], configTab: '', configTabId: 'tab:config', insightsTab: '', insightsTabId: 'tab:insights', ignoreTab: '', ignoreTabId: 'tab:ignore' })
  ;({ session } = await import('./session.svelte'))
  await session.restore()
})

// The backend watches only the repository on screen, and starts as it's told of the switch.
describe('switching tabs', () => {
  it('opens the tab’s view once the backend is watching it, not before', async () => {
    const watching = Promise.withResolvers<void>()
    api.SetActiveRepo.mockReturnValue(watching.promise)
    const switched = session.select('/b')
    await settle()
    // A view that opened now could read the repository before anything watched it.
    expect(session.active).toBe('/a')
    watching.resolve()
    await switched
    expect(session.active).toBe('/b')
  })

  it('tells the backend of each switch in turn, so it ends up watching the last tab asked for', async () => {
    const first = Promise.withResolvers<void>()
    api.SetActiveRepo.mockImplementation((path: string) => (path === '/b' ? first.promise : Promise.resolve()))
    const toB = session.select('/b')
    const toC = session.select('/c')
    await settle()
    // The second waits for the first: sent together, the backend could take them either way round.
    expect(api.SetActiveRepo.mock.calls).toEqual([['/b']])
    first.resolve()
    await Promise.all([toB, toC])
    expect(api.SetActiveRepo.mock.calls).toEqual([['/b'], ['/c']])
    expect(session.active).toBe('/c')
  })

  it('reads the tab’s label again, since nothing was watching its branch', async () => {
    api.RepoInfo.mockResolvedValue(tab('/b', 'made-elsewhere'))
    await session.select('/b')
    await settle()
    expect(session.tabs.map((t) => t.head)).toEqual(['main', 'made-elsewhere', 'main'])
  })

  it('doesn’t ask for a label for a tab that isn’t a repository', async () => {
    await session.select('tab:insights')
    await session.select('')
    await settle()
    expect(api.RepoInfo).not.toHaveBeenCalled()
    expect(session.active).toBe('')
  })

  it('still switches when the backend couldn’t be told, and says so', async () => {
    api.SetActiveRepo.mockRejectedValue(new Error('settings are read-only'))
    await session.select('/b')
    expect(session.active).toBe('/b')
    expect(session.error).toContain('read-only')
    // And the next switch isn't held up by the one that failed.
    api.SetActiveRepo.mockResolvedValue(undefined)
    await session.select('/c')
    expect(session.active).toBe('/c')
  })

  it('reads the label of the tab that takes a closed one’s place', async () => {
    await session.select('/c')
    api.RepoInfo.mockClear()
    api.RepoInfo.mockResolvedValue(tab('/b', 'moved'))
    await session.close('/c')
    await settle()
    expect(session.active).toBe('/b')
    expect(session.tabs).toEqual([tab('/a'), tab('/b', 'moved')])
  })
})
