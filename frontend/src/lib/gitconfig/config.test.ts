import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { app, git } from '../../../wailsjs/go/models'

type Handler = (payload: unknown) => void

// The backend, its events and the session aren't here: each test says what the backend answers.
const { api, handlers, session } = vi.hoisted(() => ({
  api: {
    CloseGitConfig: vi.fn(),
    EffectiveGitConfig: vi.fn(),
    GitConfigScopes: vi.fn(),
    OpenGitConfig: vi.fn(),
    ReloadGitConfig: vi.fn(),
    SaveGitConfig: vi.fn(),
    SetGitConfigTab: vi.fn(),
  },
  handlers: {} as Record<string, Handler[]>,
  session: {
    tabs: [] as { path: string; name: string; head: string }[],
    active: '',
    configTabId: 'tab:gitconfig',
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

const GLOBAL = '/home/me/.gitconfig'
const local = (repo: string) => `${repo}/.git/config`
const tab = (path: string) => ({ path, name: path.slice(1), head: 'main' })
const doc = (path: string, over: Partial<app.ConfigDoc> = {}) => ({ path, text: '', entries: [], exists: true, dirty: false, changedOnDisk: false, ...over }) as app.ConfigDoc
const entry = (key: string, scope: string, file: string) => ({ scope, file, key, value: 'x', implicit: false }) as git.ConfigEntry
const saved = (path: string, over: Partial<app.ConfigDoc> = {}) => ({ saved: true, doc: doc(path, over), diskText: '' })
const refused = (path: string, over: Partial<app.ConfigDoc>, diskText = '') => ({ saved: false, doc: doc(path, { dirty: true, ...over }), diskText })

const fire = (name: string, payload: unknown) => (handlers[name] ?? []).forEach((h) => h(payload))
/** Lets work an event handler started, and didn't wait for, finish. */
const settle = () => vi.advanceTimersByTimeAsync(0)

let gitConfig: (typeof import('./config.svelte'))['gitConfig']
let problemText: (typeof import('./config.svelte'))['problemText']

beforeEach(async () => {
  vi.useFakeTimers()
  // The tab is one object for the whole app, so each test loads the module afresh.
  vi.resetModules()
  for (const name of Object.keys(handlers)) delete handlers[name]
  session.tabs = []
  session.active = ''
  for (const fn of Object.values(api)) fn.mockReset().mockResolvedValue(undefined)
  api.GitConfigScopes.mockImplementation(async (repo: string) => ({ global: GLOBAL, globalAlt: '', local: repo ? local(repo) : '' }))
  api.OpenGitConfig.mockImplementation(async (path: string) => doc(path))
  api.ReloadGitConfig.mockImplementation(async (path: string) => doc(path))
  api.EffectiveGitConfig.mockResolvedValue([])
  ;({ gitConfig, problemText } = await import('./config.svelte'))
})

afterEach(() => {
  vi.useRealTimers()
})

describe('opening the tab', () => {
  it('shows global config when no repository is asked for', async () => {
    await gitConfig.show()
    expect(gitConfig.open).toBe(true)
    expect(gitConfig.scope).toEqual({ kind: 'global', path: GLOBAL, repo: '' })
    expect(gitConfig.doc).toEqual(doc(GLOBAL))
    expect(session.active).toBe(session.configTabId)
    expect(api.SetGitConfigTab).toHaveBeenLastCalledWith(GLOBAL)
  })

  it("shows a repository's own config, with what git sees from that repository", async () => {
    session.tabs = [tab('/a'), tab('/b')]
    api.EffectiveGitConfig.mockImplementation(async (repo: string) => [entry('user.name', 'global', repo || 'no repository')])
    await gitConfig.show('/b')
    expect(gitConfig.scopes.map((s) => s.path)).toEqual([GLOBAL, local('/a'), local('/b')])
    expect(gitConfig.scope).toEqual({ kind: 'local', path: local('/b'), repo: '/b' })
    expect(gitConfig.effective).toEqual([entry('user.name', 'global', '/b')])
  })

  it("falls back to global config for a repository whose config can't be found", async () => {
    session.tabs = [tab('/broken')]
    api.GitConfigScopes.mockImplementation(async (repo: string) => {
      if (repo) throw new Error('not a repository')
      return { global: GLOBAL, globalAlt: '', local: '' }
    })
    await gitConfig.show('/broken')
    expect(gitConfig.selected).toBe(GLOBAL)
    expect(gitConfig.scopes).toHaveLength(1)
    expect(gitConfig.error).toBe('')
  })

  it('keeps the file it had selected when shown again without a repository', async () => {
    session.tabs = [tab('/a')]
    await gitConfig.show('/a')
    await gitConfig.show()
    expect(gitConfig.selected).toBe(local('/a'))
  })

  it('says why a file could not be opened', async () => {
    api.OpenGitConfig.mockRejectedValue(new Error('permission denied'))
    await gitConfig.show()
    expect(gitConfig.error).toBe('permission denied')
    expect(gitConfig.doc).toBeUndefined()
    expect(gitConfig.busy).toBe(false)
  })

  it('restores the file selected last session, without taking the window', async () => {
    session.tabs = [tab('/a')]
    session.active = '/a'
    await gitConfig.restore(local('/a'))
    expect(gitConfig.open).toBe(true)
    expect(gitConfig.selected).toBe(local('/a'))
    expect(session.active).toBe('/a')
  })

  it('restores to global config when the saved file is no longer offered', async () => {
    await gitConfig.restore(local('/closed'))
    expect(gitConfig.selected).toBe(GLOBAL)
  })

  it('stays closed when no tab was saved', async () => {
    await gitConfig.restore('')
    expect(gitConfig.open).toBe(false)
    expect(api.GitConfigScopes).not.toHaveBeenCalled()
  })
})

describe('showKey', () => {
  beforeEach(() => {
    session.tabs = [tab('/a')]
  })

  it('opens the file that sets the key, however git spells its path and the key', async () => {
    api.EffectiveGitConfig.mockResolvedValue([entry('User.Name', 'global', '/Home/Me/.gitconfig'), entry('core.editor', 'local', local('/a'))])
    await gitConfig.showKey('/a', 'user.name')
    expect(gitConfig.selected).toBe(GLOBAL)
    expect(gitConfig.extra).toEqual([])
    expect(gitConfig.view).toBe('settings')
    expect(gitConfig.focusKey).toBe('user.name')
  })

  it('takes the last file to set it', async () => {
    api.EffectiveGitConfig.mockResolvedValue([entry('user.name', 'global', GLOBAL), entry('user.name', 'local', local('/a'))])
    await gitConfig.showKey('/a', 'user.name')
    expect(gitConfig.selected).toBe(local('/a'))
  })

  it("stays on the repository's file when only system config sets the key", async () => {
    api.EffectiveGitConfig.mockResolvedValue([entry('user.name', 'system', '/etc/gitconfig')])
    await gitConfig.showKey('/a', 'user.name')
    expect(gitConfig.selected).toBe(local('/a'))
    expect(gitConfig.focusKey).toBe('user.name')
  })

  it('adds an included file to the files it offers', async () => {
    api.EffectiveGitConfig.mockResolvedValue([entry('user.name', 'global', '/home/me/work.inc')])
    await gitConfig.showKey('/a', 'user.name')
    expect(gitConfig.scope).toEqual({ kind: 'file', path: '/home/me/work.inc', repo: '/a' })
    expect(gitConfig.scopes.map((s) => s.path)).toEqual([GLOBAL, local('/a'), '/home/me/work.inc'])
    expect(gitConfig.doc).toEqual(doc('/home/me/work.inc'))
  })
})

describe('unsaved edits', () => {
  beforeEach(async () => {
    session.tabs = [tab('/a')]
    await gitConfig.show('/a')
  })

  it('are listed for every file that has them', async () => {
    expect(await gitConfig.apply(async (path) => doc(path, { dirty: true }))).toBe(true)
    await gitConfig.select(GLOBAL)
    expect(gitConfig.dirtyPaths).toEqual([local('/a')])
    await gitConfig.apply(async (path) => doc(path, { dirty: true }))
    expect(gitConfig.dirtyPaths.toSorted()).toEqual([local('/a'), GLOBAL].toSorted())
  })

  it('leave the working copy alone, with the reason, when an edit fails', async () => {
    const edit = () => Promise.reject(new Error('invalid key: nope'))
    expect(await gitConfig.apply(edit)).toBe(false)
    expect(gitConfig.error).toBe('invalid key: nope')
    expect(gitConfig.doc).toEqual(doc(local('/a')))
  })

  it('make closing ask first', async () => {
    await gitConfig.apply(async (path) => doc(path, { dirty: true }))
    gitConfig.requestClose()
    await settle()
    expect(gitConfig.confirmClose).toBe(true)
    expect(gitConfig.open).toBe(true)
    expect(api.CloseGitConfig).not.toHaveBeenCalled()

    await gitConfig.close()
    expect(gitConfig.confirmClose).toBe(false)
    expect(gitConfig.open).toBe(false)
    expect(gitConfig.dirtyPaths).toEqual([])
  })

  it("aren't asked about when there are none", async () => {
    gitConfig.requestClose()
    await settle()
    expect(gitConfig.confirmClose).toBe(false)
    expect(gitConfig.open).toBe(false)
  })
})

describe('closing', () => {
  it('closes every open file and forgets the tab', async () => {
    session.tabs = [tab('/a')]
    await gitConfig.show('/a')
    await gitConfig.select(GLOBAL)
    await gitConfig.close()
    expect(api.CloseGitConfig.mock.calls.map(([path]) => path).toSorted()).toEqual([local('/a'), GLOBAL].toSorted())
    expect(gitConfig.docs).toEqual({})
    expect(gitConfig.selected).toBe('')
    expect(api.SetGitConfigTab).toHaveBeenLastCalledWith('')
  })

  it('moves to the last repository tab when it was the active one', async () => {
    session.tabs = [tab('/a'), tab('/b')]
    await gitConfig.show()
    await gitConfig.close()
    expect(session.active).toBe('/b')
  })

  it('moves to the start page when no repository is open', async () => {
    await gitConfig.show()
    await gitConfig.close()
    expect(session.active).toBe('')
  })

  it('leaves the active tab alone when it was in the background', async () => {
    session.tabs = [tab('/a'), tab('/b')]
    await gitConfig.show()
    session.active = '/a'
    await gitConfig.close()
    expect(session.active).toBe('/a')
  })

  it("still closes when the backend can't close a file", async () => {
    await gitConfig.show()
    api.CloseGitConfig.mockRejectedValue(new Error('gone'))
    await gitConfig.close()
    expect(gitConfig.open).toBe(false)
    expect(gitConfig.docs).toEqual({})
  })
})

describe('a config file changing on disk', () => {
  const path = local('/a')

  beforeEach(async () => {
    session.tabs = [tab('/a')]
    await gitConfig.show('/a')
  })

  it('is read again when it has no edits', async () => {
    api.OpenGitConfig.mockResolvedValue(doc(path, { changedOnDisk: true }))
    api.ReloadGitConfig.mockResolvedValue(doc(path, { text: 'theirs' }))
    await gitConfig.onDisk(path)
    expect(gitConfig.doc).toEqual(doc(path, { text: 'theirs' }))
    expect(gitConfig.outside).toEqual({})
  })

  it('keeps unsaved edits, and is flagged instead', async () => {
    api.OpenGitConfig.mockResolvedValue(doc(path, { text: 'mine', dirty: true, changedOnDisk: true }))
    await gitConfig.onDisk(path)
    expect(api.ReloadGitConfig).not.toHaveBeenCalled()
    expect(gitConfig.doc?.text).toBe('mine')
    expect(gitConfig.outside).toEqual({ [path]: true })
  })

  // Saving writes the file, which the watcher reports like any other change.
  it("isn't flagged when the change was the tab's own save", async () => {
    api.OpenGitConfig.mockResolvedValue(doc(path, { text: 'saved' }))
    await gitConfig.onDisk(path)
    api.OpenGitConfig.mockResolvedValue(doc(path, { text: 'edited since', dirty: true }))
    await gitConfig.onDisk(path)
    expect(api.ReloadGitConfig).not.toHaveBeenCalled()
    expect(gitConfig.doc?.text).toBe('edited since')
    expect(gitConfig.outside).toEqual({})
  })

  it('updates what git sees even when the file is not one that is open', async () => {
    api.OpenGitConfig.mockClear()
    api.EffectiveGitConfig.mockResolvedValue([entry('user.name', 'global', '/home/me/work.inc')])
    await gitConfig.onDisk('/home/me/work.inc')
    expect(api.OpenGitConfig).not.toHaveBeenCalled()
    expect(gitConfig.effective).toEqual([entry('user.name', 'global', '/home/me/work.inc')])
  })

  it('is left alone once the tab has closed', async () => {
    await gitConfig.close()
    api.OpenGitConfig.mockClear()
    api.EffectiveGitConfig.mockClear()
    await gitConfig.onDisk(path)
    expect(api.OpenGitConfig).not.toHaveBeenCalled()
    expect(api.EffectiveGitConfig).not.toHaveBeenCalled()
  })

  // Without the error the old values would stay on screen as if they were current.
  it("says so when it can't be re-read", async () => {
    api.OpenGitConfig.mockRejectedValue(new Error('permission denied'))
    await gitConfig.onDisk(path)
    expect(gitConfig.error).toBe(`Couldn't re-read ${path}: permission denied`)
    expect(gitConfig.outside).toEqual({})
  })

  it('says so when reading the new contents fails', async () => {
    api.OpenGitConfig.mockResolvedValue(doc(path, { changedOnDisk: true }))
    api.ReloadGitConfig.mockRejectedValue(new Error('file vanished'))
    await gitConfig.onDisk(path)
    expect(gitConfig.error).toBe(`Couldn't re-read ${path}: file vanished`)
  })

  it('is heard about from the backend', async () => {
    api.OpenGitConfig.mockResolvedValue(doc(path, { changedOnDisk: true }))
    api.ReloadGitConfig.mockResolvedValue(doc(path, { text: 'theirs' }))
    fire('gitconfig:changed', { path })
    await settle()
    expect(gitConfig.doc?.text).toBe('theirs')
  })

  it('is handled once, however often the tab is shown', async () => {
    await gitConfig.show('/a')
    await gitConfig.close()
    await gitConfig.show('/a')
    expect(handlers['gitconfig:changed']).toHaveLength(1)
    expect(handlers['repo:changed']).toHaveLength(1)
  })

  it("covers a repository's local file when the repository reports a config change", async () => {
    api.OpenGitConfig.mockClear()
    fire('repo:changed', { path: '/a', kinds: ['refs', 'index'] })
    fire('repo:changed', { path: '/elsewhere', kinds: ['config'] })
    await settle()
    expect(api.OpenGitConfig).not.toHaveBeenCalled()

    api.OpenGitConfig.mockResolvedValue(doc(path, { changedOnDisk: true }))
    api.ReloadGitConfig.mockResolvedValue(doc(path, { text: 'theirs' }))
    fire('repo:changed', { path: '/a', kinds: ['config'] })
    await settle()
    expect(gitConfig.doc?.text).toBe('theirs')
  })
})

describe('saving', () => {
  beforeEach(async () => {
    await gitConfig.show()
  })

  it('first sends the edits a view was holding back, then keeps what was saved', async () => {
    const order: string[] = []
    gitConfig.flush = async () => void order.push('flush')
    api.SaveGitConfig.mockImplementation(async () => {
      order.push('save')
      return saved(GLOBAL, { text: 'saved' })
    })
    await gitConfig.save()
    expect(order).toEqual(['flush', 'save'])
    expect(api.SaveGitConfig).toHaveBeenCalledWith(GLOBAL, false)
    expect(gitConfig.doc).toEqual(doc(GLOBAL, { text: 'saved' }))
    expect(gitConfig.error).toBe('')
  })

  it('stops for a file that changed on disk, and overwrites it only when told to', async () => {
    api.OpenGitConfig.mockResolvedValue(doc(GLOBAL, { dirty: true, changedOnDisk: true }))
    await gitConfig.onDisk(GLOBAL)
    api.SaveGitConfig.mockResolvedValue(refused(GLOBAL, { changedOnDisk: true }, 'theirs'))
    await gitConfig.save()
    expect(gitConfig.conflict).toEqual({ path: GLOBAL, disk: 'theirs' })
    expect(gitConfig.doc?.dirty).toBe(true)

    api.SaveGitConfig.mockResolvedValue(saved(GLOBAL))
    await gitConfig.save(true)
    expect(api.SaveGitConfig).toHaveBeenLastCalledWith(GLOBAL, true)
    expect(gitConfig.conflict).toBeNull()
    expect(gitConfig.outside).toEqual({})
    expect(gitConfig.dirtyPaths).toEqual([])
  })

  it("explains a file git couldn't parse", async () => {
    api.SaveGitConfig.mockResolvedValue(refused(GLOBAL, { problem: { line: 3, message: 'bad section header' } as app.ConfigProblem }))
    await gitConfig.save()
    expect(gitConfig.error).toBe('Not saved: line 3: bad section header')
    expect(gitConfig.conflict).toBeNull()
  })

  it('reports a save the backend rejects', async () => {
    api.SaveGitConfig.mockRejectedValue(new Error('disk full'))
    await gitConfig.save()
    expect(gitConfig.error).toBe('disk full')
    expect(gitConfig.doc).toEqual(doc(GLOBAL))
    expect(gitConfig.busy).toBe(false)
  })

  it('shows what git sees after the save', async () => {
    api.SaveGitConfig.mockResolvedValue(saved(GLOBAL))
    api.EffectiveGitConfig.mockResolvedValue([entry('user.name', 'global', GLOBAL)])
    await gitConfig.save()
    expect(gitConfig.effective).toEqual([entry('user.name', 'global', GLOBAL)])
  })
})

describe('after an outside change to an edited file', () => {
  beforeEach(async () => {
    await gitConfig.show()
    api.OpenGitConfig.mockResolvedValue(doc(GLOBAL, { text: 'mine', dirty: true, changedOnDisk: true }))
    await gitConfig.onDisk(GLOBAL)
  })

  it('reload drops the edits, the ones a view was holding back too', async () => {
    gitConfig.cancel = vi.fn()
    api.ReloadGitConfig.mockResolvedValue(doc(GLOBAL, { text: 'theirs' }))
    await gitConfig.reload()
    expect(gitConfig.cancel).toHaveBeenCalled()
    expect(gitConfig.doc).toEqual(doc(GLOBAL, { text: 'theirs' }))
    expect(gitConfig.outside).toEqual({})
  })

  it('keeping mine keeps the edits and drops the flag', () => {
    gitConfig.keepMine()
    expect(api.ReloadGitConfig).not.toHaveBeenCalled()
    expect(gitConfig.doc?.text).toBe('mine')
    expect(gitConfig.outside).toEqual({})
  })
})

describe('problemText', () => {
  it('names the line when there is one', () => {
    expect(problemText({ line: 3, message: 'bad section header' } as app.ConfigProblem)).toBe('line 3: bad section header')
    expect(problemText({ line: 0, message: 'not a config file' } as app.ConfigProblem)).toBe('not a config file')
  })
})
