import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { app } from '../../../wailsjs/go/models'
import type { Scope } from './insights.svelte'

type Handler = (payload: unknown) => void

// The backend, its events and the session aren't here: each test says what the backend answers.
const { api, handlers, session } = vi.hoisted(() => ({
  api: {
    Fetch: vi.fn(),
    GroupBatch: vi.fn(),
    Groups: vi.fn(),
    InsightsBranches: vi.fn(),
    InsightsFetchedAt: vi.fn(),
    InsightsHotspots: vi.fn(),
    InsightsReleaseBranches: vi.fn(),
    InsightsShipped: vi.fn(),
    InsightsTags: vi.fn(),
    InsightsThresholds: vi.fn(),
    SaveFile: vi.fn(),
    SetInsightsTab: vi.fn(),
    SetInsightsThresholds: vi.fn(),
  },
  handlers: {} as Record<string, Handler[]>,
  session: {
    tabs: [] as { path: string; name: string; head: string }[],
    active: '',
    error: '',
    insightsTabId: 'tab:insights',
    get activeTab() {
      return this.tabs.find((t) => t.path === this.active)
    },
    async select(path: string) {
      this.active = path
    },
  },
}))

vi.mock('../../../wailsjs/go/app/App.js', () => api)
vi.mock('../../../wailsjs/runtime/runtime.js', () => ({
  EventsOn: (name: string, fn: Handler) => {
    ;(handlers[name] ??= []).push(fn)
    return () => {}
  },
}))
vi.mock('../session.svelte', () => ({ session }))

const REPO: Scope = { kind: 'repo', id: '/a' }
const TEAM: Scope = { kind: 'group', id: 'team' }
const DEFAULTS = { stale: 7, longLived: 14, behind: 50, size: 500 }
const SAVED = { stale: 3, longLived: 10, behind: 20, size: 100 } as app.Thresholds
const tab = (path: string) => ({ path, name: path.slice(1), head: 'main' })
const inputsAt = (asOf: number) => ({ asOf }) as app.InsightsInputs

/** Each repository's own as-of point (its target's tip), and the branches it has. */
let tips: Record<string, number>
let branchList: Record<string, { name: string; sha: string }[]>

const win = (repo: string, inputs: app.InsightsInputs) => ({ target: 'main', targetSha: `tip-${repo}`, asOf: inputs.asOf || tips[repo] })
const shipped = (repo: string, inputs: app.InsightsInputs) => ({ window: win(repo, inputs), items: [{ sha: `shipped-${repo}` }] }) as unknown as app.ShippedResult

const fire = (name: string, payload?: unknown) => (handlers[name] ?? []).forEach((h) => h(payload))
/** The repositories computed since the mocks were last cleared, in order. */
const computed = () => api.InsightsBranches.mock.calls.map(([repo]) => repo as string)
const asOfs = () => Object.fromEntries(Object.entries(insights.data).map(([repo, d]) => [repo, d.shipped?.window.asOf]))

let insights: (typeof import('./insights.svelte'))['insights']

beforeEach(async () => {
  vi.useFakeTimers()
  // The tab is one object for the whole app, so each test loads the module afresh.
  vi.resetModules()
  for (const name of Object.keys(handlers)) delete handlers[name]
  session.tabs = [tab('/a'), tab('/b')]
  session.active = '/a'
  session.error = ''
  tips = { '/a': 100, '/b': 300, '/c': 200 }
  branchList = {}
  for (const fn of Object.values(api)) fn.mockReset().mockResolvedValue(undefined)
  api.Groups.mockResolvedValue([{ name: 'team', paths: ['/a', '/b', '/c'] }])
  api.InsightsTags.mockResolvedValue([])
  api.InsightsReleaseBranches.mockResolvedValue([])
  api.InsightsThresholds.mockResolvedValue(SAVED)
  api.InsightsFetchedAt.mockResolvedValue(0)
  api.InsightsShipped.mockImplementation(async (repo: string, inputs: app.InsightsInputs) => shipped(repo, inputs))
  api.InsightsBranches.mockImplementation(async (repo: string, inputs: app.InsightsInputs) => ({ window: win(repo, inputs), thresholds: inputs.thresholds, branches: branchList[repo] ?? [] }))
  api.InsightsHotspots.mockImplementation(async (repo: string, inputs: app.InsightsInputs) => ({ window: win(repo, inputs), files: [], rework: [] }))
  ;({ insights } = await import('./insights.svelte'))
})

afterEach(() => {
  vi.useRealTimers()
})

describe('the scope', () => {
  it('is the active repository when the tab is first shown', async () => {
    await insights.show()
    expect(insights.open).toBe(true)
    expect(insights.scope).toEqual(REPO)
    expect(insights.repos).toEqual(['/a'])
    expect(insights.items).toEqual([{ repo: '/a', value: { sha: 'shipped-/a' } }])
    expect(insights.loading).toBe(false)
    expect(session.active).toBe(session.insightsTabId)
    expect(api.SetInsightsTab).toHaveBeenLastCalledWith('repo:/a')
  })

  it('is the first project group when no repository is active', async () => {
    session.active = ''
    await insights.show()
    expect(insights.scope).toEqual(TEAM)
    expect(insights.repos).toEqual(['/a', '/b', '/c'])
    expect(insights.items.map((i) => i.repo)).toEqual(['/a', '/b', '/c'])
    expect(api.SetInsightsTab).toHaveBeenLastCalledWith('group:team')
  })

  it('is empty, and nothing is computed, with neither', async () => {
    session.active = ''
    api.Groups.mockRejectedValue(new Error('no settings'))
    await insights.show()
    expect(insights.open).toBe(true)
    expect(insights.scope).toBeNull()
    expect(insights.repos).toEqual([])
    expect(computed()).toEqual([])
  })

  it('stays as it was when the tab is shown again from a repository', async () => {
    await insights.show(TEAM)
    session.active = '/a'
    await insights.show()
    expect(insights.scope).toEqual(TEAM)
  })

  it('is restored from last session without taking the window', async () => {
    await insights.restore('group:team')
    expect(insights.open).toBe(true)
    expect(insights.scope).toEqual(TEAM)
    expect(Object.keys(insights.data).toSorted()).toEqual(['/a', '/b', '/c'])
    expect(session.active).toBe('/a')
  })

  it('keeps the colons of a restored Windows path', async () => {
    await insights.restore('repo:C:\\Repos\\a')
    expect(insights.scope).toEqual({ kind: 'repo', id: 'C:\\Repos\\a' })
  })

  it.each(['', 'folder:/a', '/a'])('is not restored from %j', async (saved) => {
    await insights.restore(saved)
    expect(insights.open).toBe(false)
    expect(insights.scope).toBeNull()
    expect(computed()).toEqual([])
  })

  it("lists a repository's tags and release branches as releases, newest first", async () => {
    api.InsightsTags.mockResolvedValue([
      { name: 'v1', time: 100 },
      { name: 'v2', time: 200 },
    ])
    api.InsightsReleaseBranches.mockResolvedValue([
      { name: 'release/3', forkTime: 300 },
      { name: 'release/2', forkTime: 200 },
    ])
    await insights.setScope(REPO)
    expect(insights.tags).toEqual([
      { name: 'release/3', time: 300, branch: true },
      { name: 'release/2', time: 200, branch: true },
      { name: 'v2', time: 200 },
      { name: 'v1', time: 100 },
    ])

    await insights.setWindow('releases')
    expect(api.InsightsShipped).toHaveBeenLastCalledWith('/a', expect.objectContaining({ window: 'releases', from: 'release/2', to: 'release/3' }))
  })

  it("keeps the releases already chosen when the window turns to releases", async () => {
    api.InsightsTags.mockResolvedValue([
      { name: 'v1', time: 100 },
      { name: 'v2', time: 200 },
      { name: 'v3', time: 300 },
    ])
    await insights.setScope(REPO)
    insights.from = 'v1'
    insights.to = 'v2'
    await insights.setWindow('releases')
    expect([insights.from, insights.to]).toEqual(['v1', 'v2'])
  })

  // Releases are one repository's tags, so a group can't have a window between two.
  it('has no releases, or a releases window, for a group', async () => {
    api.InsightsTags.mockResolvedValue([
      { name: 'v1', time: 100 },
      { name: 'v2', time: 200 },
    ])
    await insights.setScope(REPO)
    await insights.setWindow('releases')
    await insights.setScope(TEAM)
    expect(insights.tags).toEqual([])
    expect(insights.window).toBe('14d')
  })
})

describe('thresholds', () => {
  it("are the scope's saved ones", async () => {
    await insights.show()
    expect(api.InsightsThresholds).toHaveBeenLastCalledWith('repo:/a')
    expect(insights.thresholds).toEqual(SAVED)
    expect(api.InsightsBranches).toHaveBeenLastCalledWith('/a', expect.objectContaining({ thresholds: SAVED }))
  })

  // The defaults flag different branches, so falling back to them is said out loud.
  it("are the defaults, with a warning, when the saved ones can't be read", async () => {
    api.InsightsThresholds.mockRejectedValue(new Error('settings are locked'))
    await insights.show()
    expect(insights.thresholds).toEqual(DEFAULTS)
    expect(session.error).toBe('Using the default thresholds: settings are locked')
    expect(computed()).toEqual(['/a'])
  })

  it('are saved for the scope, and the branches measured again', async () => {
    await insights.show(TEAM)
    api.InsightsBranches.mockClear()
    const next = { ...SAVED, stale: 30 } as app.Thresholds
    await insights.setThresholds(next)
    expect(api.SetInsightsThresholds).toHaveBeenCalledWith('group:team', next)
    expect(insights.thresholds).toEqual(next)
    expect(computed().toSorted()).toEqual(['/a', '/b', '/c'])
    expect(api.InsightsBranches).toHaveBeenLastCalledWith(expect.anything(), expect.objectContaining({ thresholds: next }))
  })

  it("stay as they were when they can't be saved", async () => {
    await insights.show()
    api.InsightsBranches.mockClear()
    api.SetInsightsThresholds.mockRejectedValue(new Error('settings are locked'))
    await insights.setThresholds({ ...SAVED, stale: 30 } as app.Thresholds)
    expect(insights.error).toBe('settings are locked')
    expect(insights.thresholds).toEqual(SAVED)
    expect(computed()).toEqual([])
  })
})

describe('computing', () => {
  it("lines a group's windows up on the latest of its repositories' as-of points", async () => {
    await insights.show(TEAM)
    expect(asOfs()).toEqual({ '/a': 300, '/b': 300, '/c': 300 })
    expect(insights.resolved?.asOf).toBe(300)
  })

  it('uses an as-of point that was chosen, for one repository or a group', async () => {
    await insights.show()
    await insights.setAsOf(150)
    expect(asOfs()).toEqual({ '/a': 150 })

    await insights.setScope(TEAM)
    expect(asOfs()).toEqual({ '/a': 150, '/b': 150, '/c': 150 })
  })

  it("reports one repository's failure against it, and still computes the rest", async () => {
    api.InsightsShipped.mockImplementation(async (repo: string, inputs: app.InsightsInputs) => {
      if (repo === '/b') throw new Error('not a repository')
      return shipped(repo, inputs)
    })
    await insights.show(TEAM)
    expect(insights.data['/b']).toEqual({ fetchedAt: 0, error: 'not a repository' })
    // The repository that failed has no say in where the others' windows line up.
    expect(asOfs()).toEqual({ '/a': 200, '/b': undefined, '/c': 200 })
    expect(insights.items.map((i) => i.repo)).toEqual(['/a', '/c'])
    expect(insights.error).toBe('')
    expect(insights.loading).toBe(false)
  })

  it('computes only the repositories asked for', async () => {
    await insights.show(TEAM)
    api.InsightsBranches.mockClear()
    tips['/c'] = 400
    await insights.load(['/c'])
    expect(computed()).toEqual(['/c'])
    expect(asOfs()).toEqual({ '/a': 300, '/b': 300, '/c': 400 })
  })

  it('drops the results of a load a newer one overtook', async () => {
    await insights.show()
    const slow = Promise.withResolvers<app.ShippedResult>()
    api.InsightsShipped.mockImplementationOnce(() => slow.promise)
    const first = insights.setAsOf(150)
    await insights.setAsOf(250)
    slow.resolve(shipped('/a', inputsAt(150)))
    await first
    expect(asOfs()).toEqual({ '/a': 250 })
    expect(insights.loading).toBe(false)
  })

  it('stays busy until the newest load is done', async () => {
    await insights.show()
    const slow = Promise.withResolvers<app.ShippedResult>()
    const first = insights.setAsOf(150)
    api.InsightsShipped.mockImplementationOnce(() => slow.promise)
    const second = insights.setAsOf(250)
    await first
    expect(insights.loading).toBe(true)
    slow.resolve(shipped('/a', inputsAt(250)))
    await second
    expect(insights.loading).toBe(false)
  })

  // Thresholds and releases are read before the load, and the scope can change while they are.
  it("doesn't leave a slower scope's thresholds and releases on the scope chosen after it", async () => {
    const slow = Promise.withResolvers<{ name: string; time: number }[]>()
    api.InsightsTags.mockImplementation(() => slow.promise)
    api.InsightsThresholds.mockImplementation(async (scope: string) => (scope === 'group:team' ? SAVED : DEFAULTS))
    const first = insights.setScope(REPO)
    await insights.setScope(TEAM)
    slow.resolve([{ name: 'v1', time: 100 }])
    await first
    expect(insights.scope).toEqual(TEAM)
    expect(insights.thresholds).toEqual(SAVED)
    expect(insights.tags).toEqual([])
  })

  it('leaves hotspots until their view is open, then computes them at the same as-of point', async () => {
    await insights.show(TEAM)
    expect(api.InsightsHotspots).not.toHaveBeenCalled()

    await insights.setView('hotspots')
    expect(api.InsightsHotspots.mock.calls.map(([repo, inputs]) => [repo, inputs.asOf]).toSorted()).toEqual([
      ['/a', 300],
      ['/b', 300],
      ['/c', 300],
    ])
    expect(insights.data['/b'].hotspots).toBeDefined()

    // While the view is open, a recompute brings them along.
    await insights.setAsOf(150)
    expect(insights.data['/b'].hotspots?.window.asOf).toBe(150)
  })
})

describe('fetching', () => {
  it('fetches the repository, then computes again', async () => {
    await insights.show()
    tips['/a'] = 500
    await insights.fetch()
    expect(api.Fetch).toHaveBeenCalledWith('/a', '')
    expect(asOfs()).toEqual({ '/a': 500 })
    expect(insights.fetching).toBe(false)
  })

  it("fetches all of a group's repositories together", async () => {
    await insights.show(TEAM)
    await insights.fetch()
    expect(api.GroupBatch).toHaveBeenCalledWith('team', 'fetch')
    expect(api.Fetch).not.toHaveBeenCalled()
  })

  // load() clears the error as it starts, so fetch() has to say its failure after it.
  it('still says the fetch failed once the results are computed again', async () => {
    await insights.show()
    api.Fetch.mockRejectedValue(new Error('could not resolve host'))
    await insights.fetch()
    expect(insights.fetching).toBe(false)
    expect(insights.error).toBe('could not resolve host')
  })
})

describe('conflict checks', () => {
  beforeEach(() => {
    branchList['/a'] = [
      { name: 'one', sha: 'b1' },
      { name: 'two', sha: 'b2' },
    ]
  })

  const check = (targetSha: string, sha: string) => ({ repo: '/a', targetSha, branch: '', sha, conflicts: true, files: ['x.go'] })

  it('join their branch, and the rest count as still running', async () => {
    await insights.show()
    expect(insights.pending).toBe(2)
    fire('insights:conflict', check('tip-/a', 'b2'))
    expect(insights.branches.map((b) => [b.value.name, b.conflict?.files])).toEqual([
      ['one', undefined],
      ['two', ['x.go']],
    ])
    expect(insights.pending).toBe(1)
  })

  // Cached checks are reported at once, often before the branch list arrives.
  it('are kept when they arrive before the branches they are for', async () => {
    branchList['/a'] = []
    await insights.show()
    fire('insights:conflict', check('tip-/a', 'b1'))
    branchList['/a'] = [{ name: 'one', sha: 'b1' }]
    await insights.load()
    expect(insights.branches[0].conflict?.conflicts).toBe(true)
    expect(insights.pending).toBe(0)
  })

  it("don't count for a branch once the target has moved on", async () => {
    await insights.show()
    fire('insights:conflict', check('an-older-tip', 'b1'))
    expect(insights.pending).toBe(2)
  })
})

describe('when a repository changes', () => {
  beforeEach(async () => {
    await insights.show()
    api.InsightsBranches.mockClear()
  })

  it.each(['refs', 'head', 'config'])('computes it again half a second after its %s change', async (kind) => {
    fire('repo:changed', { path: '/a', kinds: ['index', kind] })
    await vi.advanceTimersByTimeAsync(499)
    expect(computed()).toEqual([])
    await vi.advanceTimersByTimeAsync(1)
    expect(computed()).toEqual(['/a'])
  })

  it("leaves changes that can't move the results alone", async () => {
    fire('repo:changed', { path: '/a', kinds: ['index', 'worktree', 'state'] })
    await vi.advanceTimersByTimeAsync(5000)
    expect(computed()).toEqual([])
  })

  it('leaves repositories outside the scope alone', async () => {
    fire('repo:changed', { path: '/b', kinds: ['refs'] })
    await vi.advanceTimersByTimeAsync(5000)
    expect(computed()).toEqual([])
  })

  it('computes once for a burst of changes, after the last of them', async () => {
    fire('repo:changed', { path: '/a', kinds: ['refs'] })
    await vi.advanceTimersByTimeAsync(300)
    fire('repo:changed', { path: '/a', kinds: ['head'] })
    await vi.advanceTimersByTimeAsync(300)
    fire('gitconfig:changed')
    await vi.advanceTimersByTimeAsync(499)
    expect(computed()).toEqual([])
    await vi.advanceTimersByTimeAsync(1)
    expect(computed()).toEqual(['/a'])
  })

  it('knows the repository however its path is spelled', async () => {
    await insights.setScope({ kind: 'repo', id: 'C:\\Repos\\a' })
    api.InsightsBranches.mockClear()
    fire('repo:changed', { path: 'c:/repos/a/', kinds: ['refs'] })
    await vi.advanceTimersByTimeAsync(500)
    expect(computed()).toEqual(['C:\\Repos\\a'])
  })

  // A group shares one as-of point, so one repository moving can move every window.
  it('computes the whole group when one of its repositories changes', async () => {
    await insights.setScope(TEAM)
    api.InsightsBranches.mockClear()
    tips['/a'] = 900
    fire('repo:changed', { path: '/a', kinds: ['refs'] })
    await vi.advanceTimersByTimeAsync(500)
    expect(asOfs()).toEqual({ '/a': 900, '/b': 900, '/c': 900 })
  })

  it('computes everything again after global config changes', async () => {
    await insights.setScope(TEAM)
    api.InsightsBranches.mockClear()
    fire('gitconfig:changed')
    await vi.advanceTimersByTimeAsync(499)
    expect(computed()).toEqual([])
    await vi.advanceTimersByTimeAsync(1)
    expect(computed().toSorted()).toEqual(['/a', '/b', '/c'])
  })

  it('does nothing while the tab is closed', async () => {
    await insights.close()
    fire('repo:changed', { path: '/a', kinds: ['refs'] })
    fire('gitconfig:changed')
    await vi.advanceTimersByTimeAsync(5000)
    expect(computed()).toEqual([])
  })

  it('listens once, however often the tab is shown', async () => {
    await insights.close()
    await insights.show()
    await insights.restore('repo:/a')
    expect(Object.values(handlers).map((list) => list.length)).toEqual([1, 1, 1])
  })

  it('drops a recompute that was waiting when the tab closed', async () => {
    fire('repo:changed', { path: '/a', kinds: ['refs'] })
    await insights.close()
    await vi.advanceTimersByTimeAsync(5000)
    expect(computed()).toEqual([])
  })

  // A recompute left waiting would compute the old scope's repository, and that load would outdate the new scope's.
  it("doesn't lose the next repository's results to a recompute waiting for the last one", async () => {
    const slow = Promise.withResolvers<app.ShippedResult>()
    api.InsightsShipped.mockImplementation((repo: string, inputs: app.InsightsInputs) => (repo === '/b' ? slow.promise : Promise.resolve(shipped(repo, inputs))))
    fire('repo:changed', { path: '/a', kinds: ['refs'] })
    const switching = insights.setScope({ kind: 'repo', id: '/b' })
    await vi.advanceTimersByTimeAsync(500)
    slow.resolve(shipped('/b', inputsAt(0)))
    await switching
    expect(insights.data['/b']?.shipped).toBeDefined()
  })
})

describe('closing', () => {
  it('forgets the tab and moves to the last repository tab', async () => {
    await insights.show()
    await insights.close()
    expect(insights.open).toBe(false)
    expect(session.active).toBe('/b')
    expect(api.SetInsightsTab).toHaveBeenLastCalledWith('')
  })

  it('moves to the start page when no repository is open', async () => {
    await insights.show()
    session.tabs = []
    await insights.close()
    expect(session.active).toBe('')
  })

  it('leaves the active tab alone when it was in the background', async () => {
    await insights.show()
    session.active = '/a'
    await insights.close()
    expect(session.active).toBe('/a')
  })
})
