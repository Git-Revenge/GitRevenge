import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { git } from '../../../wailsjs/go/models'

// The backend and the session aren't here: the bindings read and write the `disk` below.
const { api, session } = vi.hoisted(() => ({
  api: {
    CheckIgnorePath: vi.fn(),
    IgnoreSources: vi.fn(),
    IgnoreStats: vi.fn(),
    ReadIgnoreFile: vi.fn(),
    SaveIgnoreFile: vi.fn(),
    SetIgnoreTab: vi.fn(),
  },
  session: {
    tabs: [] as { path: string; name: string; head: string }[],
    active: '',
    ignoreTabId: 'tab:ignore',
    async select(path: string) {
      this.active = path
    },
  },
}))

vi.mock('../../../wailsjs/go/app/App.js', () => api)
vi.mock('../session.svelte', () => ({ session }))

const ROOT = '.gitignore'
const EXCLUDE = '.git/info/exclude'
const tab = (path: string) => ({ path, name: path.slice(1), head: 'main' })
const source = (name: string) => ({ source: name, path: name, scope: 'repo', exists: true, rules: 1 }) as git.IgnoreSource
const stats = (repo: string) => ({ rules: [{ source: repo }], truncated: false }) as unknown as git.IgnoreStats

/** Each repository's ignore files as they are on disk. */
let disk: Record<string, Record<string, string>>

/** Lets work that wasn't waited for, such as the counts, finish. */
const settle = () => vi.advanceTimersByTimeAsync(0)

let ignoreRules: (typeof import('./ignoreRules.svelte'))['ignoreRules']

beforeEach(async () => {
  vi.useFakeTimers()
  // The tab is one object for the whole app, so each test loads the module afresh.
  vi.resetModules()
  session.tabs = [tab('/a'), tab('/b')]
  session.active = '/a'
  disk = {
    '/a': { [ROOT]: 'node_modules\n', [EXCLUDE]: '*.log\n' },
    '/b': { [ROOT]: 'dist\n', [EXCLUDE]: '' },
  }
  for (const fn of Object.values(api)) fn.mockReset().mockResolvedValue(undefined)
  api.IgnoreSources.mockImplementation(async (repo: string) => Object.keys(disk[repo]).map(source))
  api.IgnoreStats.mockImplementation(async (repo: string) => stats(repo))
  api.ReadIgnoreFile.mockImplementation(async (repo: string, src: string) => disk[repo][src])
  api.SaveIgnoreFile.mockImplementation(async (repo: string, src: string, _base: string, text: string) => {
    disk[repo][src] = text
  })
  ;({ ignoreRules } = await import('./ignoreRules.svelte'))
})

afterEach(() => {
  vi.useRealTimers()
})

describe('opening the tab', () => {
  it("reads the repository's ignore files and shows the first", async () => {
    await ignoreRules.show('/b')
    await settle()
    expect(ignoreRules.open).toBe(true)
    expect(ignoreRules.repo).toBe('/b')
    expect(ignoreRules.repoName).toBe('b')
    expect(ignoreRules.sources.map((s) => s.source)).toEqual([ROOT, EXCLUDE])
    expect(ignoreRules.source?.source).toBe(ROOT)
    expect(ignoreRules.doc).toEqual({ base: 'dist\n', text: 'dist\n' })
    expect(ignoreRules.stats).toEqual(stats('/b'))
    expect(session.active).toBe(session.ignoreTabId)
    expect(api.SetIgnoreTab).toHaveBeenLastCalledWith('/b')
  })

  it('opens at a file and line, however the file is spelled', async () => {
    await ignoreRules.show('/a', '.git\\info\\Exclude', 4)
    expect(ignoreRules.selected).toBe(EXCLUDE)
    expect(ignoreRules.doc).toEqual({ base: '*.log\n', text: '*.log\n' })
    expect(ignoreRules.view).toBe('rules')
    expect(ignoreRules.focusLine).toBe(4)
  })

  it("restores last session's repository without taking the window", async () => {
    await ignoreRules.restore('/b')
    expect(ignoreRules.open).toBe(true)
    expect(ignoreRules.repo).toBe('/b')
    expect(ignoreRules.doc?.text).toBe('dist\n')
    expect(session.active).toBe('/a')
  })

  it("shows the first open repository when the one it was opened for isn't open", async () => {
    await ignoreRules.restore('/closed')
    expect(ignoreRules.repo).toBe('/a')
    expect(ignoreRules.doc?.text).toBe('node_modules\n')
  })

  it('has nothing to show when no repository is open', async () => {
    session.tabs = []
    await ignoreRules.show('/a')
    expect(ignoreRules.repo).toBe('')
    expect(ignoreRules.sources).toEqual([])
    expect(api.IgnoreSources).not.toHaveBeenCalled()
  })

  it('stays closed when no tab was saved', async () => {
    await ignoreRules.restore('')
    expect(ignoreRules.open).toBe(false)
    expect(api.IgnoreSources).not.toHaveBeenCalled()
  })

  it("says why the files couldn't be listed", async () => {
    api.IgnoreSources.mockRejectedValue(new Error('not a repository'))
    await ignoreRules.show('/a')
    expect(ignoreRules.error).toBe('not a repository')
    expect(ignoreRules.sources).toEqual([])
  })
})

describe('edits', () => {
  beforeEach(async () => {
    await ignoreRules.show('/a')
  })

  it('mark a file as unsaved only while it differs from disk', () => {
    ignoreRules.edit('node_modules\ndist\n')
    expect(ignoreRules.doc).toEqual({ base: 'node_modules\n', text: 'node_modules\ndist\n' })
    expect(ignoreRules.dirty).toEqual([ROOT])
    ignoreRules.edit('node_modules\n')
    expect(ignoreRules.dirty).toEqual([])
  })

  it('are kept per file while another is selected', async () => {
    ignoreRules.edit('edited\n')
    await ignoreRules.select(EXCLUDE)
    expect(ignoreRules.doc?.text).toBe('*.log\n')
    ignoreRules.edit('*.tmp\n')
    expect(ignoreRules.dirty).toEqual([ROOT, EXCLUDE])
    await ignoreRules.select(ROOT)
    expect(ignoreRules.doc?.text).toBe('edited\n')
  })

  it('are dropped by revert', () => {
    ignoreRules.edit('edited\n')
    ignoreRules.revert()
    expect(ignoreRules.doc?.text).toBe('node_modules\n')
    expect(ignoreRules.dirty).toEqual([])
  })

  it('are dropped by reload, which reads the file again', async () => {
    ignoreRules.edit('edited\n')
    disk['/a'][ROOT] = 'theirs\n'
    await ignoreRules.reload()
    expect(ignoreRules.doc).toEqual({ base: 'theirs\n', text: 'theirs\n' })
  })

  it('survive a refresh, which re-reads only the files without them', async () => {
    await ignoreRules.select(EXCLUDE)
    ignoreRules.edit('mine\n')
    disk['/a'] = { [ROOT]: 'theirs\n', [EXCLUDE]: 'theirs too\n' }
    await ignoreRules.load()
    expect(ignoreRules.docs[ROOT]).toEqual({ base: 'theirs\n', text: 'theirs\n' })
    // The base stays too, so saving can tell the file changed underneath the edits.
    expect(ignoreRules.docs[EXCLUDE]).toEqual({ base: '*.log\n', text: 'mine\n' })
  })

  it('keep the tab on its repository until saved or reverted', async () => {
    ignoreRules.edit('edited\n')
    await ignoreRules.show('/b')
    expect(ignoreRules.repo).toBe('/a')
    expect(ignoreRules.doc?.text).toBe('edited\n')

    ignoreRules.revert()
    await ignoreRules.show('/b')
    expect(ignoreRules.repo).toBe('/b')
    expect(ignoreRules.doc).toEqual({ base: 'dist\n', text: 'dist\n' })
  })
})

describe('saving', () => {
  beforeEach(async () => {
    await ignoreRules.show('/a')
  })

  it('writes the edits with the text they were made from, then shows the file as it now is', async () => {
    // The backend may tidy what it writes, so the tab shows what it reads back.
    api.SaveIgnoreFile.mockImplementation(async (repo: string, src: string, _base: string, text: string) => {
      disk[repo][src] = text.trimEnd() + '\n'
    })
    ignoreRules.edit('node_modules\ndist')
    await ignoreRules.save()
    expect(api.SaveIgnoreFile).toHaveBeenCalledWith('/a', ROOT, 'node_modules\n', 'node_modules\ndist')
    expect(ignoreRules.doc).toEqual({ base: 'node_modules\ndist\n', text: 'node_modules\ndist\n' })
    expect(ignoreRules.dirty).toEqual([])
    expect(ignoreRules.saveError).toBe('')
    expect(ignoreRules.busy).toBe(false)
  })

  it('keeps the edits and the reason when the save is refused', async () => {
    api.SaveIgnoreFile.mockRejectedValue(new Error('the file changed on disk'))
    ignoreRules.edit('edited\n')
    await ignoreRules.save()
    expect(ignoreRules.saveError).toBe('the file changed on disk')
    expect(ignoreRules.doc).toEqual({ base: 'node_modules\n', text: 'edited\n' })
    expect(ignoreRules.busy).toBe(false)

    // A refresh from disk leaves the reason up; dropping the edits clears it.
    await ignoreRules.load()
    expect(ignoreRules.saveError).toBe('the file changed on disk')
    ignoreRules.revert()
    expect(ignoreRules.saveError).toBe('')
  })

  it('does nothing before a file is loaded', async () => {
    await ignoreRules.close()
    await ignoreRules.save()
    expect(api.SaveIgnoreFile).not.toHaveBeenCalled()
  })
})

describe('closing', () => {
  beforeEach(async () => {
    await ignoreRules.show('/a')
  })

  it('asks first when edits would be lost', async () => {
    ignoreRules.edit('edited\n')
    ignoreRules.requestClose()
    await settle()
    expect(ignoreRules.confirmClose).toBe(true)
    expect(ignoreRules.open).toBe(true)
    expect(ignoreRules.doc?.text).toBe('edited\n')

    await ignoreRules.close()
    expect(ignoreRules.confirmClose).toBe(false)
    expect(ignoreRules.open).toBe(false)
    expect(ignoreRules.dirty).toEqual([])
    expect(api.SetIgnoreTab).toHaveBeenLastCalledWith('')
  })

  it('closes at once when nothing is unsaved', async () => {
    ignoreRules.requestClose()
    await settle()
    expect(ignoreRules.confirmClose).toBe(false)
    expect(ignoreRules.open).toBe(false)
  })

  it('moves to the last repository tab when it was the active one', async () => {
    await ignoreRules.close()
    expect(session.active).toBe('/b')
  })

  it('moves to the start page when no repository is open', async () => {
    session.tabs = []
    await ignoreRules.close()
    expect(session.active).toBe('')
  })

  it('leaves the active tab alone when it was in the background', async () => {
    session.active = '/a'
    await ignoreRules.close()
    expect(session.active).toBe('/a')
  })

  it('discards the edits, so the file opens clean next time', async () => {
    ignoreRules.edit('edited\n')
    await ignoreRules.close()
    await ignoreRules.show('/a')
    expect(ignoreRules.doc).toEqual({ base: 'node_modules\n', text: 'node_modules\n' })
  })
})

describe('answers that arrive after the repository changed', () => {
  it('drop a file list and counts for the repository it left', async () => {
    const list = Promise.withResolvers<git.IgnoreSource[]>()
    const counts = Promise.withResolvers<git.IgnoreStats>()
    api.IgnoreSources.mockImplementation((repo: string) => (repo === '/a' ? list.promise : Promise.resolve([source('b.ignore')])))
    api.IgnoreStats.mockImplementation((repo: string) => (repo === '/a' ? counts.promise : Promise.resolve(stats(repo))))
    api.ReadIgnoreFile.mockResolvedValue('from b\n')

    const first = ignoreRules.show('/a')
    void ignoreRules.loadStats()
    await ignoreRules.show('/b')
    list.resolve([source(ROOT)])
    counts.resolve(stats('/a'))
    await first
    await settle()
    expect(ignoreRules.sources.map((s) => s.source)).toEqual(['b.ignore'])
    expect(ignoreRules.stats).toEqual(stats('/b'))
  })

  // ensureDoc used to store what it read without checking the repository was still the one it read for.
  it("don't put the old repository's file in the new one's place", async () => {
    const slow = Promise.withResolvers<string>()
    api.ReadIgnoreFile.mockImplementation((repo: string, src: string) => (repo === '/a' ? slow.promise : Promise.resolve(disk[repo][src])))

    const first = ignoreRules.show('/a')
    await settle()
    await ignoreRules.show('/b')
    slow.resolve('node_modules\n')
    await first
    expect(ignoreRules.repo).toBe('/b')
    expect(ignoreRules.doc).toEqual({ base: 'dist\n', text: 'dist\n' })
  })
})
