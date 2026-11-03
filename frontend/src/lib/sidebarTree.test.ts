import { describe, expect, it } from 'vitest'
import type { Ref } from './graph/rowCache'
import { buildPanels, careCount, matchParts, needsCare, panelMatches, panelRows, refStatus, refTooltip, stackPlace, type RowOptions, type TreeRow } from './sidebarTree'

const ref = (short: string, extra: Partial<Ref> = {}): Ref => ({
  name: short,
  short,
  kind: 'local',
  commit: short,
  head: false,
  upstream: '',
  ahead: 0,
  behind: 0,
  gone: false,
  ...extra,
})
const remote = (short: string, extra: Partial<Ref> = {}) => ref(short, { kind: 'remote', ...extra })
const tag = (short: string) => ref(short, { kind: 'tag' })

/** One line per row: indent, then the folder's "name/" or the branch's label. */
function outline(rows: TreeRow[]): string[] {
  return rows.map((r) => '  '.repeat(r.depth) + (r.type === 'folder' ? r.label + '/' : r.label))
}
const branches = (refs: Ref[], options?: RowOptions) => panelRows(buildPanels(refs).branches, options)
const folder = (rows: TreeRow[], label: string) => {
  const row = rows.find((r) => r.type === 'folder' && r.label === label)
  if (row?.type !== 'folder') throw new Error(`no folder ${label}`)
  return row
}
const branch = (rows: TreeRow[], short: string) => {
  const row = rows.find((r) => r.type === 'ref' && r.ref.short === short)
  if (row?.type !== 'ref') throw new Error(`no row for ${short}`)
  return row
}

describe('panels', () => {
  it('sorts refs into their panels', () => {
    const p = buildPanels([ref('main'), remote('origin/main'), tag('v1'), ref('stash@{0}', { kind: 'stash', subject: 'WIP' })])
    expect(outline(panelRows(p.branches))).toEqual(['main'])
    expect(outline(panelRows(p.remotes))).toEqual(['origin/', '  main'])
    expect(outline(panelRows(p.tags))).toEqual(['v1'])
    expect(outline(panelRows(p.stashes))).toEqual(['WIP'])
  })

  it('leaves out hidden branches unless they are being shown', () => {
    const refs = [ref('main'), ref('wip', { hidden: true })]
    expect(outline(panelRows(buildPanels(refs).branches))).toEqual(['main'])
    expect(outline(panelRows(buildPanels(refs, { showHidden: true }).branches))).toEqual(['main', 'wip'])
  })
})

describe('nesting', () => {
  it('nests at every slash and labels rows with the last part', () => {
    const rows = branches([ref('a/b/one'), ref('a/b/two'), ref('a/three')])
    expect(outline(rows)).toEqual(['a/', '  b/', '    one', '    two', '  three'])
    expect(branch(rows, 'a/b/one').ref.short).toBe('a/b/one')
  })

  it('keeps a folder with one branch as one flat row', () => {
    expect(outline(branches([ref('gitkraken/empty-tree')]))).toEqual(['gitkraken/empty-tree'])
  })

  it('flattens a chain of folders holding one branch', () => {
    expect(outline(branches([ref('a/b/c/only')]))).toEqual(['a/b/c/only'])
    expect(outline(branches([ref('a/one'), ref('a/b/c/only')]))).toEqual(['a/', '  b/c/only', '  one'])
  })

  it('keeps a remote as a folder, even with one branch', () => {
    const rows = panelRows(buildPanels([remote('upstream/main')]).remotes)
    expect(outline(rows)).toEqual(['upstream/', '  main'])
    expect(folder(rows, 'upstream').remote).toBe(true)
  })

  it('nests remotes by remote, then folder, flattening below the remote', () => {
    const rows = panelRows(buildPanels([remote('origin/main'), remote('origin/release/v0.7'), remote('origin/release/v0.8'), remote('origin/feature/x')]).remotes)
    expect(outline(rows)).toEqual(['origin/', '  release/', '    v0.7', '    v0.8', '  feature/x', '  main'])
    expect(folder(rows, 'release').remote).toBe(false)
  })

  it('never splits a stash subject at a slash', () => {
    const p = buildPanels([ref('stash@{0}', { kind: 'stash', subject: 'On feature/x: pointers' })])
    expect(outline(panelRows(p.stashes))).toEqual(['On feature/x: pointers'])
  })
})

describe('stacks', () => {
  it('puts a stacked branch under its parent, across folders, with its full name', () => {
    const rows = branches([ref('feature/base'), ref('feature/other'), ref('fix/child', { stackParent: 'feature/base' }), ref('326-top', { stackParent: 'fix/child' })])
    expect(outline(rows)).toEqual(['feature/', '  base/', '    326-top', '    fix/child', '    base', '  other'])
    expect(branch(rows, 'fix/child').stacked).toBe(true)
    expect(branch(rows, 'feature/base').stacked).toBe(false)
  })

  // #462: a stack of ten stepped off the side of the panel.
  describe('as a group', () => {
    const STACK = [ref('a'), ref('b', { stackParent: 'a' }), ref('c', { stackParent: 'b' }), ref('d', { stackParent: 'c' })]
    const key = 'branches:stack:a'
    const merged = { a: 'main', b: 'main', c: 'main', d: 'main' }

    it('lists a stack flat, the top first, under a row of its own', () => {
      const rows = branches(STACK)
      expect(outline(rows)).toEqual(['a/', '  d', '  c', '  b', '  a'])
      expect(rows[0]).toMatchObject({ type: 'folder', stack: true, count: 4, open: true })
      expect(rows.slice(1).every((r) => r.type === 'ref' && r.railed)).toBe(true)
    })

    it('leaves a pair as a parent and its child', () => {
      const rows = branches([ref('a'), ref('b', { stackParent: 'a' })])
      expect(outline(rows)).toEqual(['a', '  b'])
      expect(branch(rows, 'b').railed).toBeFalsy()
    })

    it('keeps a fork in one list, each branch above the one it builds on', () => {
      const rows = branches([ref('a'), ref('b', { stackParent: 'a' }), ref('c', { stackParent: 'a' }), ref('d', { stackParent: 'c' })])
      expect(outline(rows)).toEqual(['a/', '  b', '  d', '  c', '  a'])
    })

    it('starts closed when everything in it is merged, and says so', () => {
      const rows = branches(STACK, { merged })
      expect(outline(rows)).toEqual(['a/'])
      expect(folder(rows, 'a')).toMatchObject({ open: false, note: 'merged', count: 4 })
    })

    it('stays open while anything in it is not merged', () => {
      const rows = branches(STACK, { merged: { a: 'main', b: 'main', c: 'main' } })
      expect(folder(rows, 'a')).toMatchObject({ open: true })
      expect(folder(rows, 'a').note).toBeUndefined()
    })

    it('stays as the person left it', () => {
      expect(outline(branches(STACK, { merged, open: { [key]: true } }))).toHaveLength(5)
      expect(outline(branches(STACK, { open: { [key]: false } }))).toEqual(['a/'])
    })

    it('stays open around the checked-out branch, even when merged', () => {
      const rows = branches([ref('a'), ref('b', { stackParent: 'a' }), ref('c', { stackParent: 'b', head: true })], { merged: { a: 'main', b: 'main', c: 'main' } })
      expect(outline(rows)).toEqual(['a/', '  c', '  b', '  a'])
    })

    it('leads the list when it holds the checked-out branch', () => {
      const rows = branches([ref('0-first'), ref('a'), ref('b', { stackParent: 'a' }), ref('c', { stackParent: 'b', head: true })])
      expect(outline(rows)[0]).toBe('a/')
    })

    it('opens to show what a filter found, keeping what those branches build on', () => {
      const rows = branches(STACK, { merged, query: 'c', open: { [key]: false } })
      expect(outline(rows)).toEqual(['a/', '  c', '  b', '  a'])
      expect(folder(rows, 'a').count).toBe(1)
      expect(branch(rows, 'c').context).toBe(false)
      expect(branch(rows, 'a').context).toBe(true)
    })

    it('opens under Needs care too', () => {
      const rows = branches([ref('a'), ref('b', { stackParent: 'a', stackParentMerged: true }), ref('c', { stackParent: 'b', gone: true })], { care: true, merged: { a: 'main', b: 'main', c: 'main' } })
      expect(outline(rows)).toEqual(['a/', '  c', '  b', '  a'])
    })

    it('sits in the folder of the branch at the bottom', () => {
      const rows = branches([ref('fix/a'), ref('b', { stackParent: 'fix/a' }), ref('c', { stackParent: 'b' }), ref('fix/z')])
      expect(outline(rows)).toEqual(['fix/', '  a/', '    c', '    b', '    a', '  z'])
    })
  })

  it('counts stacked branches in their parent’s folder', () => {
    const rows = branches([ref('feature/base'), ref('other', { stackParent: 'feature/base' })])
    expect(outline(rows)).toEqual(['feature/', '  base', '    other'])
    expect(folder(rows, 'feature').count).toBe(2)
  })

  it('keeps a branch in place when its parent isn’t listed', () => {
    expect(outline(branches([ref('child', { stackParent: 'gone-parent' })]))).toEqual(['child'])
  })

  it('keeps a branch in place when its parent is hidden', () => {
    const refs = [ref('parent', { hidden: true }), ref('child', { stackParent: 'parent' })]
    expect(outline(panelRows(buildPanels(refs).branches))).toEqual(['child'])
    expect(outline(panelRows(buildPanels(refs, { showHidden: true }).branches))).toEqual(['parent', '  child'])
  })

  it('loses no branch to a stack that loops', () => {
    const rows = branches([ref('a', { stackParent: 'b' }), ref('b', { stackParent: 'a' }), ref('self', { stackParent: 'self' }), ref('leaf', { stackParent: 'a' })])
    expect(rows.map((r) => (r.type === 'ref' ? r.ref.short : '')).sort()).toEqual(['a', 'b', 'leaf', 'self'])
  })
})

describe('order', () => {
  it('puts Trunk first, then folders, then loose branches', () => {
    const rows = branches([ref('zeta'), ref('main', { branchKind: 'trunk' }), ref('fix/a'), ref('fix/b'), ref('alpha')])
    expect(outline(rows)).toEqual(['main', 'fix/', '  a', '  b', 'alpha', 'zeta'])
  })

  it('puts the checked-out branch above Trunk and folders', () => {
    const rows = branches([ref('main', { branchKind: 'trunk' }), ref('fix/a'), ref('fix/b'), ref('alpha'), ref('zeta', { head: true })])
    expect(outline(rows)).toEqual(['zeta', 'main', 'fix/', '  a', '  b', 'alpha'])
  })

  it('puts the folder holding the checked-out branch first, at every level', () => {
    const rows = branches([ref('main', { branchKind: 'trunk' }), ref('alpha/a'), ref('alpha/b'), ref('zed/one'), ref('zed/deep/x'), ref('zed/deep/y', { head: true })])
    expect(outline(rows)).toEqual(['zed/', '  deep/', '    y', '    x', '  one', 'main', 'alpha/', '  a', '  b'])
  })

  it('puts a stack holding the checked-out branch first', () => {
    const rows = branches([ref('alpha'), ref('zeta'), ref('zeta-top', { head: true, stackParent: 'zeta' })])
    expect(outline(rows)).toEqual(['zeta', '  zeta-top', 'alpha'])
  })

  it('keeps Trunk first when Trunk is checked out', () => {
    const rows = branches([ref('alpha'), ref('fix/a'), ref('fix/b'), ref('main', { branchKind: 'trunk', head: true })])
    expect(outline(rows)).toEqual(['main', 'fix/', '  a', '  b', 'alpha'])
  })

  it('drops the checked-out branch from the top when the filter excludes it', () => {
    expect(outline(branches([ref('alpha'), ref('zeta', { head: true })], { query: 'alpha' }))).toEqual(['alpha'])
  })

  it('sorts numbers by value', () => {
    expect(outline(branches([ref('b10'), ref('b9')]))).toEqual(['b9', 'b10'])
  })

  it('sorts releases newest first', () => {
    const k = { branchKind: 'release' }
    expect(outline(branches([ref('release/v0.9', k), ref('release/v0.10', k)]))).toEqual(['release/', '  v0.10', '  v0.9'])
  })

  it('sorts tags newest first', () => {
    expect(outline(panelRows(buildPanels([tag('v1.9'), tag('v1.10')]).tags))).toEqual(['v1.10', 'v1.9'])
  })

  it('keeps stashes in the order git gives them', () => {
    const s = (i: number, subject: string) => ref(`stash@{${i}}`, { kind: 'stash', subject })
    expect(outline(panelRows(buildPanels([s(0, 'z newest'), s(1, 'a oldest')]).stashes))).toEqual(['z newest', 'a oldest'])
  })
})

describe('sorting by most recently updated', () => {
  const recent = (refs: Ref[], options?: RowOptions) => panelRows(buildPanels(refs, { sort: 'recent' }).branches, options)

  it('puts the newest branch first', () => {
    expect(outline(recent([ref('a', { updated: 100 }), ref('b', { updated: 300 }), ref('c', { updated: 200 })]))).toEqual(['b', 'c', 'a'])
  })

  it('still leads with the checked-out branch, then Trunk', () => {
    const rows = recent([ref('main', { branchKind: 'trunk', updated: 1 }), ref('new', { updated: 900 }), ref('here', { head: true, updated: 2 })])
    expect(outline(rows)).toEqual(['here', 'main', 'new'])
  })

  it('sorts folders by the newest branch inside, however deep', () => {
    const rows = recent([ref('a/one', { updated: 100 }), ref('a/two', { updated: 200 }), ref('b/one', { updated: 50 }), ref('b/deep/x', { updated: 900 }), ref('b/deep/y', { updated: 1 })])
    expect(outline(rows)).toEqual(['b/', '  deep/', '    x', '    y', '  one', 'a/', '  two', '  one'])
  })

  it('moves a parent up with the newest branch stacked on it', () => {
    const rows = recent([ref('other', { updated: 500 }), ref('base', { updated: 100 }), ref('top', { stackParent: 'base', updated: 900 })])
    expect(outline(rows)).toEqual(['base', '  top', 'other'])
  })

  it('breaks ties by name, and puts undated refs last', () => {
    expect(outline(recent([ref('b', { updated: 5 }), ref('a', { updated: 5 }), ref('undated')]))).toEqual(['a', 'b', 'undated'])
  })

  it('applies to releases and tags, instead of their version order', () => {
    const k = { branchKind: 'release' }
    expect(outline(recent([ref('release/v0.9', { ...k, updated: 900 }), ref('release/v0.10', { ...k, updated: 100 })]))).toEqual(['release/', '  v0.9', '  v0.10'])
    const t = (short: string, updated: number) => ({ ...tag(short), updated })
    expect(outline(panelRows(buildPanels([t('v2', 1), t('v1', 9)], { sort: 'recent' }).tags))).toEqual(['v1', 'v2'])
  })

  it('leaves stashes in git’s order', () => {
    const s = (i: number, subject: string) => ref(`stash@{${i}}`, { kind: 'stash', subject })
    expect(outline(panelRows(buildPanels([s(0, 'z'), s(1, 'a')], { sort: 'recent' }).stashes))).toEqual(['z', 'a'])
  })

  it('uses name order by default', () => {
    expect(outline(branches([ref('a', { updated: 1 }), ref('b', { updated: 9 })]))).toEqual(['a', 'b'])
  })
})

describe('kinds', () => {
  it('marks a top-level folder whose branches share a kind, not its rows', () => {
    const k = { branchKind: 'release' }
    const rows = branches([ref('release/v0.7', k), ref('release/v0.8', k)])
    expect(folder(rows, 'release').kind).toBe('release')
    expect(branch(rows, 'release/v0.7').mark).toBeUndefined()
  })

  it('leaves a folder of mixed kinds unmarked, and marks its rows', () => {
    const rows = branches([ref('x/one', { branchKind: 'docs' }), ref('x/two')])
    expect(folder(rows, 'x').kind).toBeUndefined()
    expect(branch(rows, 'x/one').mark).toBe('docs')
    expect(branch(rows, 'x/two').mark).toBeUndefined()
  })

  it('marks a loose branch of a kind', () => {
    expect(branch(branches([ref('staging', { branchKind: 'environment' })]), 'staging').mark).toBe('environment')
  })

  it('marks a stacked branch whose kind differs from its parent’s folder', () => {
    const k = { branchKind: 'feature' }
    const rows = branches([ref('feature/base', k), ref('feature/two', k), ref('docs-fix', { branchKind: 'docs', stackParent: 'feature/base' }), ref('feature/top', { ...k, stackParent: 'docs-fix' })])
    expect(branch(rows, 'docs-fix').mark).toBe('docs')
    expect(branch(rows, 'feature/top').mark).toBeUndefined()
  })

  it('marks the kind folder under a remote', () => {
    const k = { branchKind: 'release' }
    const rows = panelRows(buildPanels([remote('origin/release/v0.7', k), remote('origin/release/v0.8', k)]).remotes)
    expect(folder(rows, 'origin').kind).toBeUndefined()
    expect(folder(rows, 'release').kind).toBe('release')
  })

  it('starts Bot folders closed, and honours the choice once made', () => {
    const k = { branchKind: 'bot' }
    const refs = [ref('dependabot/a', k), ref('dependabot/b', k)]
    expect(outline(branches(refs))).toEqual(['dependabot/'])
    expect(folder(branches(refs), 'dependabot').open).toBe(false)
    const key = folder(branches(refs), 'dependabot').key
    expect(outline(branches(refs, { open: { [key]: true } }))).toEqual(['dependabot/', '  a', '  b'])
  })
})

describe('open and closed folders', () => {
  const refs = [ref('fix/a'), ref('fix/b', { head: true }), ref('fix/deep/c'), ref('fix/deep/d'), ref('other/x'), ref('other/y')]

  it('opens folders by default', () => {
    expect(folder(branches(refs), 'fix').open).toBe(true)
  })

  it('hides a closed folder’s rows and reports what it holds', () => {
    const key = folder(branches(refs), 'fix').key
    const rows = branches(refs, { open: { [key]: false } })
    expect(outline(rows)).toEqual(['fix/', 'other/', '  x', '  y'])
    expect(folder(rows, 'fix')).toMatchObject({ open: false, count: 4, holdsHead: true })
    expect(folder(rows, 'other').holdsHead).toBe(false)
  })

  it('reports the checked-out branch through a stack', () => {
    const r = [ref('fix/a'), ref('fix/b'), ref('top', { head: true, stackParent: 'fix/a' })]
    expect(folder(branches(r), 'fix').holdsHead).toBe(true)
  })

  it('gives folders in different panels different keys', () => {
    const p = buildPanels([ref('a/one'), ref('a/two'), tag('a/one'), tag('a/two')])
    expect(folder(panelRows(p.branches), 'a').key).not.toBe(folder(panelRows(p.tags), 'a').key)
  })
})

describe('group starts', () => {
  const starts = (rows: TreeRow[]) => rows.filter((r) => r.startsGroup).map((r) => r.label)

  it('separates the checked-out branch from the groups below it', () => {
    const rows = branches([ref('here', { head: true }), ref('main', { branchKind: 'trunk' }), ref('fix/a'), ref('fix/b'), ref('loose')])
    expect(starts(rows)).toEqual(['main', 'fix', 'loose'])
  })

  it('starts a group at each top-level folder and at the loose branches', () => {
    const rows = branches([ref('main', { branchKind: 'trunk' }), ref('fix/a'), ref('fix/b'), ref('old/a'), ref('old/b'), ref('loose'), ref('more')])
    expect(starts(rows)).toEqual(['fix', 'old', 'loose'])
  })

  it('never draws a line above the first row', () => {
    expect(starts(branches([ref('fix/a'), ref('fix/b'), ref('loose')]))).toEqual(['loose'])
    expect(starts(branches([ref('a'), ref('b')]))).toEqual([])
  })

  it('ignores groups the filter removed', () => {
    expect(starts(branches([ref('fix/a'), ref('fix/b'), ref('loose')], { query: 'loose' }))).toEqual([])
  })
})

describe('filter', () => {
  const refs = [ref('main', { branchKind: 'trunk' }), ref('release/v0.7'), ref('release/v0.8'), ref('fix/menu'), ref('fix/keys')]

  it('matches the full name, ignoring case', () => {
    expect(outline(branches(refs, { query: 'RELEASE/V0.8' }))).toEqual(['release/', '  v0.8'])
    expect(outline(branches(refs, { query: 'fix' }))).toEqual(['fix/', '  keys', '  menu'])
  })

  it('opens matching folders without touching what the person closed', () => {
    const key = folder(branches(refs), 'release').key
    const open = { [key]: false }
    expect(folder(branches(refs, { open, query: 'v0.8' }), 'release').open).toBe(true)
    expect(open).toEqual({ [key]: false })
    expect(folder(branches(refs, { open }), 'release').open).toBe(false)
  })

  it('keeps a folder when the filter leaves one branch in it, so rows don’t jump', () => {
    expect(outline(branches(refs, { query: 'menu' }))).toEqual(['fix/', '  menu'])
  })

  it('counts only matches on a folder', () => {
    expect(folder(branches(refs, { query: 'v0.8' }), 'release').count).toBe(1)
  })

  it('returns no rows when nothing matches', () => {
    expect(branches(refs, { query: 'nope' })).toEqual([])
  })

  it('ignores surrounding spaces, and treats a blank query as no filter', () => {
    expect(outline(branches([ref('a'), ref('b')], { query: ' a ' }))).toEqual(['a'])
    expect(outline(branches([ref('a'), ref('b')], { query: '   ' }))).toEqual(['a', 'b'])
  })

  it('shows a stacked match’s parents as context', () => {
    const rows = branches([ref('base'), ref('middle', { stackParent: 'base' }), ref('top', { stackParent: 'middle' }), ref('other')], { query: 'top' })
    expect(outline(rows)).toEqual(['base/', '  top', '  middle', '  base'])
    expect(branch(rows, 'base').context).toBe(true)
    expect(branch(rows, 'top').context).toBe(false)
  })

  it('finds a stash by its subject', () => {
    const p = buildPanels([ref('stash@{0}', { kind: 'stash', subject: 'icon sketches' }), ref('stash@{1}', { kind: 'stash', subject: 'other' })])
    expect(outline(panelRows(p.stashes, { query: 'sketch' }))).toEqual(['icon sketches'])
  })

  it('counts matches in every panel', () => {
    const p = buildPanels([ref('fix/v1'), ref('v1-work'), remote('origin/v1'), tag('v1'), tag('v2'), ref('stash@{0}', { kind: 'stash', subject: 'v1 notes' })])
    expect(panelMatches(p.branches, 'v1')).toBe(2)
    expect(panelMatches(p.remotes, 'v1')).toBe(1)
    expect(panelMatches(p.tags, 'v1')).toBe(1)
    expect(panelMatches(p.stashes, 'v1')).toBe(1)
    expect(panelMatches(p.tags, 'nope')).toBe(0)
  })

  it('does not count context rows as matches', () => {
    const p = buildPanels([ref('base'), ref('top', { stackParent: 'base' })])
    expect(panelMatches(p.branches, 'top')).toBe(1)
  })
})

describe('needs care', () => {
  const gone = ref('gone-one', { gone: true })
  const merged = ref('fix/merged', { stackParentMerged: true })
  const refs = [ref('main', { branchKind: 'trunk' }), gone, merged, ref('fix/fine'), ref('feature/parent'), ref('feature/other'), ref('feature/stacked-gone', { gone: true, stackParent: 'feature/parent' })]

  it('recognises gone and parent merged branches', () => {
    expect(needsCare(gone)).toBe(true)
    expect(needsCare(merged)).toBe(true)
    expect(needsCare(ref('fine'))).toBe(false)
  })

  it('keeps only those branches, plus the parents a stacked one needs', () => {
    const rows = branches(refs, { care: true })
    expect(outline(rows)).toEqual(['feature/', '  parent', '    feature/stacked-gone', 'fix/', '  merged', 'gone-one'])
    expect(branch(rows, 'feature/parent').context).toBe(true)
    expect(branch(rows, 'gone-one').context).toBe(false)
  })

  it('combines with the filter', () => {
    expect(outline(branches(refs, { care: true, query: 'gone-one' }))).toEqual(['gone-one'])
    expect(outline(branches(refs, { care: true, query: 'fix/fine' }))).toEqual([])
  })

  it('counts what needs care, leaving out hidden branches', () => {
    expect(careCount(buildPanels(refs).branches)).toBe(3)
    expect(careCount(buildPanels([...refs, ref('old', { gone: true, hidden: true })]).branches)).toBe(3)
    expect(careCount(buildPanels([ref('fine')]).branches)).toBe(0)
  })
})

describe('large repositories', () => {
  it('builds and flattens 50,000 refs quickly', () => {
    const refs = Array.from({ length: 50_000 }, (_, i) => ref(`team${i % 40}/area${i % 400}/branch-${i}`))
    const start = performance.now()
    const panels = buildPanels(refs)
    const all = panelRows(panels.branches)
    const filtered = panelRows(panels.branches, { query: 'branch-4999' })
    const elapsed = performance.now() - start
    expect(all.filter((r) => r.type === 'ref')).toHaveLength(50_000)
    expect(filtered.filter((r) => r.type === 'ref').length).toBeGreaterThan(0)
    // Generous, so slow CI runners pass: this guards against a quadratic walk, not a benchmark.
    expect(elapsed).toBeLessThan(2000)
  })

  it('handles a very wide folder quickly', () => {
    const refs = Array.from({ length: 50_000 }, (_, i) => ref(`user${i}/a`)).concat(Array.from({ length: 50_000 }, (_, i) => ref(`user${i}/b`)))
    const start = performance.now()
    const rows = panelRows(buildPanels(refs).branches)
    expect(rows.filter((r) => r.type === 'folder')).toHaveLength(50_000)
    expect(performance.now() - start).toBeLessThan(2000)
  })

  it('survives a very long stack', () => {
    const refs = Array.from({ length: 2000 }, (_, i) => ref(`s${i}`, { stackParent: i ? `s${i - 1}` : '' }))
    expect(branches(refs).filter((r) => r.type === 'ref')).toHaveLength(2000)
  })
})

describe('refStatus', () => {
  it('shows nothing for a branch in step with its upstream', () => {
    expect(refStatus(ref('main'))).toBeUndefined()
  })

  it('shows ahead and behind, with words for screen readers', () => {
    expect(refStatus(ref('a', { ahead: 1 }))).toEqual({ text: '↑1', spoken: '1 ahead', tone: 'link' })
    expect(refStatus(ref('a', { behind: 2 }))).toEqual({ text: '↓2', spoken: '2 behind', tone: 'link' })
    expect(refStatus(ref('a', { ahead: 1, behind: 2 }))).toEqual({ text: '↑1↓2', spoken: '1 ahead, 2 behind', tone: 'link' })
  })

  it('puts gone above ahead and behind', () => {
    expect(refStatus(ref('a', { gone: true, ahead: 3 }))?.text).toBe('gone')
  })

  it('puts parent merged above everything', () => {
    expect(refStatus(ref('a', { stackParentMerged: true, gone: true, ahead: 3 }))).toEqual({ text: 'parent merged', spoken: 'parent merged', tone: 'warning' })
  })

  it('shows no chip for a hidden branch or a stack position', () => {
    expect(refStatus(ref('a', { hidden: true, stackDepth: 2, stackSize: 3, stackParent: 'b' }))).toBeUndefined()
  })
})

// #463: "parent merged" suggested a restack on branches that only needed deleting.
describe('a merged branch', () => {
  it('says merged, whatever else is true of it', () => {
    expect(refStatus(ref('a', { stackParentMerged: true, gone: true, ahead: 3 }), 'main')).toEqual({ text: 'merged', spoken: 'merged into main', tone: 'muted' })
  })

  it('leaves the other statuses to branches that are not', () => {
    expect(refStatus(ref('a', { stackParentMerged: true }))?.text).toBe('parent merged')
    expect(refStatus(ref('a', { gone: true }))?.text).toBe('gone')
  })

  it('says why in its tooltip, and stops suggesting a restack', () => {
    const r = ref('a', { stackParentMerged: true, stackParent: 'b' })
    expect(refTooltip(r, 'main')).toContain('already on main')
    expect(refTooltip(r, 'main')).not.toContain('Restack')
    expect(refTooltip(r)).toContain('Restack')
  })
})

describe('refTooltip', () => {
  it('starts with the full name', () => {
    expect(refTooltip(ref('feature/x'))).toBe('feature/x')
  })

  it('uses the subject for a stash', () => {
    expect(refTooltip(ref('stash@{0}', { kind: 'stash', subject: 'icon sketches' }))).toBe('icon sketches')
  })

  it('lists everything the row’s single status leaves out', () => {
    const r = ref('feature/x', { head: true, stackParent: 'base', stackDepth: 2, stackSize: 3, stackParentMerged: true, gone: true, hidden: true })
    expect(refTooltip(r).split('\n')).toEqual([
      'feature/x',
      'Checked out',
      'base was merged. Right-click, Restack, to move this branch onto the Trunk.',
      'Stacked on base (2 of 3)',
      'Upstream branch was deleted',
      'Hidden: shown for now',
    ])
  })

  it('names the upstream for ahead and behind', () => {
    expect(refTooltip(ref('a', { ahead: 1, behind: 2, upstream: 'origin/a' })).split('\n')[1]).toBe('1 ahead, 2 behind origin/a')
  })
})

describe('stackPlace', () => {
  it('says where a branch sits in its stack', () => {
    expect(stackPlace(ref('a', { stackParent: 'b', stackDepth: 2, stackSize: 3 }))).toBe('Stacked on b (2 of 3)')
    expect(stackPlace(ref('a', { stackDepth: 1, stackSize: 3 }))).toBe('The bottom of a stack (1 of 3)')
    expect(stackPlace(ref('a'))).toBe('')
  })
})

describe('matchParts', () => {
  it('splits around the match, keeping the label’s case', () => {
    expect(matchParts('Fix-Menu', 'menu')).toEqual({ pre: 'Fix-', hit: 'Menu', post: '' })
    expect(matchParts('v0.8.1', ' 0.8 ')).toEqual({ pre: 'v', hit: '0.8', post: '.1' })
  })

  it('leaves the label whole with no query or no match in it', () => {
    expect(matchParts('v0.8', '')).toEqual({ pre: 'v0.8', hit: '', post: '' })
    expect(matchParts('v0.8', 'release/')).toEqual({ pre: 'v0.8', hit: '', post: '' })
  })
})
