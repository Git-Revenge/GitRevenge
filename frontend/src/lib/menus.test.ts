import { describe, expect, it, vi } from 'vitest'
import type { Ref, Row } from './graph/rowCache'
import { session } from './session.svelte'
import { stackItems, commitItems, commitMenuItems, deleteBranchDialog, dropItems, groups, refItems, upstreamDialog, type MenuContext } from './menus'

// Its messages listen for events from the app, which isn't here.
vi.mock('./toasts.svelte', () => ({ toasts: { show: vi.fn() } }))

const row = (id: string): Row => ({ index: 0, col: 0, color: 0, up: [], down: [], width: 1, id, parents: 1, author: '', authorTime: 0, subject: '' })
const ref = (short: string, kind: Ref['kind'], head = false): Ref => ({
  name: kind === 'tag' ? `refs/tags/${short}` : short,
  short,
  kind,
  head,
  commit: '',
  upstream: '',
  ahead: 0,
  behind: 0,
  gone: false,
})
const ctx = (current: string): MenuContext => ({ path: '/r', current: () => current, openDialog: () => {}, openRebase: () => {}, openReflog: () => {} })
const labels = (items: { label: string }[]) => items.map((i) => i.label)

describe('dropItems', () => {
  const head = [ref('main', 'local', true)]

  it('dropping a branch onto the checked-out branch merges or rebases', () => {
    const items = dropItems(row('a'.repeat(40)), [ref('feature', 'local')], row('b'.repeat(40)), head, ctx('main'))
    expect(labels(items)).toEqual(['Merge feature into main', 'Merge feature into main (no fast-forward)', 'Rebase main onto feature'])
  })

  it('dropping a bare commit onto the checked-out branch also offers cherry-pick', () => {
    const items = dropItems(row('c'.repeat(40)), [], row('b'.repeat(40)), head, ctx('main'))
    expect(labels(items)).toContain('Cherry-pick ccccccc onto main')
  })

  it('dragging the checked-out branch onto a tag offers rebasing onto it', () => {
    const items = dropItems(row('b'.repeat(40)), head, row('d'.repeat(40)), [ref('v1.0', 'tag')], ctx('main'))
    expect(labels(items)).toContain('Rebase main onto v1.0')
  })

  it('offers nothing when neither side is checked out, or HEAD is detached', () => {
    expect(dropItems(row('a'.repeat(40)), [ref('x', 'local')], row('b'.repeat(40)), [ref('y', 'local')], ctx('main'))).toEqual([])
    expect(dropItems(row('a'.repeat(40)), [ref('x', 'local')], row('b'.repeat(40)), head, ctx(''))).toEqual([])
  })
})

describe('groups', () => {
  const item = (label: string) => ({ label, run: () => {} })

  it('starts each non-empty group after the first with a separator', () => {
    const items = groups([item('a'), item('b')], [], [item('c')])
    expect(items.map((i) => [i.label, !!i.separator])).toEqual([
      ['a', false],
      ['b', false],
      ['c', true],
    ])
  })

  it('drops a repeated label, keeping the first', () => {
    const first = item('same')
    const items = groups([first, item('a')], [item('same'), item('b')])
    expect(labels(items)).toEqual(['same', 'a', 'b'])
    expect(items[0]).toBe(first)
  })

  it('keeps the group rule when a group loses its first item', () => {
    const items = groups([item('same')], [item('same'), item('b')])
    expect(items.map((i) => [i.label, !!i.separator])).toEqual([
      ['same', false],
      ['b', true],
    ])
  })

  it('keeps a rule from an item that was dropped', () => {
    const items = groups([item('same'), item('a')], [item('b'), { ...item('same'), separator: true }, item('c')])
    expect(items.map((i) => [i.label, !!i.separator])).toEqual([
      ['same', false],
      ['a', false],
      ['b', true],
      ['c', true],
    ])
  })
})

// The graph's menu for a commit combines the items of every ref on it (#323).
describe('a commit menu with several refs', () => {
  const menu = (refs: Ref[], current = 'main') => labels(groups(commitItems(row('a'.repeat(40)), refs, ctx(current)), ...refs.map((r) => refItems(r, ctx(current)))))
  const expectUnique = (list: string[]) => expect(list.filter((l, i) => list.indexOf(l) !== i)).toEqual([])

  it('has no repeated labels for a branch and its remote', () => {
    expectUnique(menu([ref('main', 'local', true), ref('origin/main', 'remote')]))
  })

  it('has no repeated labels for branches with different names', () => {
    expectUnique(menu([ref('feature/x', 'local', true), ref('origin/y', 'remote')], 'feature/x'))
    expectUnique(menu([ref('one', 'local'), ref('two', 'local'), ref('v1', 'tag')]))
  })

  it('names the branch on the items a second branch would repeat', () => {
    const list = menu([ref('one', 'local'), ref('two', 'local')])
    for (const label of ['Mark one as…', 'Mark two as…', 'Stack one on…', 'Stack two on…']) expect(list).toContain(label)
  })

  it('offers one Mark and one Hide when a branch and its remote share a name', () => {
    const list = menu([ref('main', 'local'), ref('origin/main', 'remote')])
    expect(list.filter((l) => l === 'Mark main as…')).toHaveLength(1)
    expect(list.filter((l) => l === 'Hide main')).toHaveLength(1)
  })
})

describe('refItems', () => {
  it('keeps checkout and delete on the checked-out branch, disabled', () => {
    const items = refItems(ref('main', 'local', true), ctx('main'))
    expect(items.filter((i) => i.disabled).map((i) => i.label)).toEqual(['Checkout main', 'Delete main…'])
  })
})

// #444: a squash-merged branch made you force the delete.
describe('deleting a branch', () => {
  const topic = ref('topic', 'local')
  const forced = (d: ReturnType<typeof deleteBranchDialog>) => (d.fields ?? []).some((f) => f.key === 'force')

  it('needs no tick when it was merged', () => {
    const d = deleteBranchDialog('/r', topic, { target: 'main', how: 'merged', unmerged: 0 })
    expect(forced(d)).toBe(false)
    expect(d.message).toContain('already merged into main')
    expect(d.danger).toBe(false)
  })

  it('needs no tick when its changes landed by a squash or rebase', () => {
    const d = deleteBranchDialog('/r', topic, { target: 'origin/main', how: 'squashed', unmerged: 0 })
    expect(forced(d)).toBe(false)
    expect(d.message).toContain('already on origin/main')
  })

  it('takes a tick and says what would be lost otherwise', () => {
    const d = deleteBranchDialog('/r', topic, { target: 'main', how: '', unmerged: 2 })
    expect(forced(d)).toBe(true)
    expect(d.message).toContain('2 commits are not on main')
    expect(d.danger).toBe(true)
    expect(deleteBranchDialog('/r', topic, { target: 'main', how: '', unmerged: 1 }).message).toContain('1 commit is not on main')
  })

  it('takes a tick when it could not tell', () => {
    for (const merge of [null, { target: '', how: '', unmerged: 0 }]) {
      const d = deleteBranchDialog('/r', topic, merge)
      expect(forced(d)).toBe(true)
      expect(d.message).toContain('removed locally')
    }
  })

  it('still warns about a kind that asks first', () => {
    const d = deleteBranchDialog('/r', { ...topic, branchKind: 'trunk' }, { target: 'main', how: 'merged', unmerged: 0 })
    expect(d.message).toMatch(/trunk/i)
    expect(d.danger).toBe(true)
  })
})

// #474
describe('a stack', () => {
  const stack = [
    { ...ref('top', 'local'), stackParent: 'mid' },
    { ...ref('mid', 'local'), stackParent: 'bottom' },
    ref('bottom', 'local'),
  ]

  it('offers the top of the stack, then Unstack and Delete', () => {
    expect(labels(stackItems(stack, ctx('main')))).toEqual(['Check out top of stack (top)', 'New branch on top…', 'Update stack', 'Rebase stack onto…', 'Push stack', 'Unstack', 'Delete stack…'])
  })

  it('names each top when it forks', () => {
    const forked = [{ ...ref('other', 'local'), stackParent: 'mid' }, ...stack]
    const got = labels(stackItems(forked, ctx('main')))
    expect(got).toContain('Check out other')
    expect(got).toContain('Check out top')
    expect(got).toContain('New branch on other…')
    expect(got).toContain('New branch on top…')
  })

  it('has nothing to check out when already on top', () => {
    const items = stackItems([{ ...stack[0], head: true }, stack[1], stack[2]], ctx('top'))
    expect(items.find((i) => i.label?.startsWith('Check out'))?.disabled).toBe(true)
  })
})

// #463
describe('Restack', () => {
  const stacked = { ...ref('top', 'local'), stackParentMerged: true, stackParent: 'bottom' }
  const restack = () => labels(refItems(stacked, ctx('main'))).filter((l) => l.startsWith('Restack'))

  it('is offered when the branch below merged and this one has work of its own', () => {
    session.merged['/r'] = {}
    expect(restack()).toHaveLength(1)
  })

  it('is not offered on a branch that is itself merged', () => {
    session.merged['/r'] = { top: 'main' }
    expect(restack()).toEqual([])
    session.merged['/r'] = {}
  })
})

describe('Pull on a branch', () => {
  const pull = (r: Ref) => labels(refItems(r, ctx('main'))).filter((l) => l.startsWith('Pull'))
  const tracking = { ...ref('feat', 'local'), upstream: 'origin/feat' }

  it('is offered on a branch that has an upstream and is not checked out', () => {
    expect(pull(tracking)).toEqual(['Pull feat'])
  })

  it('is not offered on the checked-out branch, or without an upstream', () => {
    expect(pull({ ...tracking, head: true })).toEqual([])
    expect(pull(ref('feat', 'local'))).toEqual([])
    expect(pull({ ...tracking, gone: true })).toEqual([])
  })
})

describe('upstreamDialog', () => {
  const refs = [ref('main', 'local'), ref('origin/main', 'remote'), ref('origin/x', 'remote'), ref('v1', 'tag')]

  it('offers only remote branches as suggestions, in a field that takes typed text', () => {
    const field = upstreamDialog('/r', ref('main', 'local'), refs).fields![0]
    expect(field.type).toBe('combobox')
    expect(field.suggestions).toEqual(['origin/main', 'origin/x'])
  })

  it('starts on the current upstream, and says so', () => {
    const spec = upstreamDialog('/r', { ...ref('main', 'local'), upstream: 'origin/main' }, refs)
    expect(spec.fields![0].value).toBe('origin/main')
    expect(spec.message).toContain('Currently tracking origin/main.')
  })

  it('explains how to stop tracking', () => {
    expect(upstreamDialog('/r', ref('main', 'local'), refs).message).toContain('Not tracking a remote branch. Leave this empty to stop tracking.')
  })
})

describe('commitMenuItems', () => {
  const commit = row('a'.repeat(40))
  const stash = { ...ref('stash@{0}', 'stash'), subject: 'WIP' }

  it('puts each ref in its own submenu, after the commit\'s items', () => {
    const items = commitMenuItems(commit, [ref('main', 'local'), ref('origin/main', 'remote'), ref('v1', 'tag')], ctx('main'))
    const subs = items.filter((i) => i.items)
    expect(subs.map((i) => i.label)).toEqual(['main', 'origin/main', 'v1'])
    expect(subs.map((i) => i.icon)).toEqual(['branch', 'remote', 'tag'])
    expect(subs[0].separator).toBe(true)
    expect(items.indexOf(subs[0])).toBe(commitItems(commit, [], ctx('main')).length)
    expect(labels(subs[1].items!)).toContain('Delete origin/main from origin')
  })

  it('keeps items per ref apart, so each branch has its own Mark', () => {
    const items = commitMenuItems(commit, [ref('one', 'local'), ref('two', 'local')], ctx('main'))
    expect(labels(items.find((i) => i.label === 'one')!.items!)).toContain('Mark one as…')
    expect(labels(items.find((i) => i.label === 'two')!.items!)).toContain('Mark two as…')
  })

  it('has just the commit\'s items with no refs', () => {
    expect(commitMenuItems(commit, [], ctx('main')).some((i) => i.items)).toBe(false)
  })

  it('lists a stash\'s items directly', () => {
    const items = commitMenuItems(commit, [stash], ctx('main'))
    expect(items.some((i) => i.items)).toBe(false)
    expect(labels(items)).toContain('Apply stash')
  })
})
