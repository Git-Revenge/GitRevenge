import { describe, expect, it } from 'vitest'
import { branchMove, headAway, headLabel, isOwnMove, LANDING, landingTop, moveAnnouncement, OWN_MOVE_MS, tipsOf, type Tips } from './follow'

const A = 'a'.repeat(40)
const B = 'b'.repeat(40)
const C = 'c'.repeat(40)
const tips = (head: string, headCommit: string, branches: Record<string, string>, behind = 0): Tips => ({ head, headCommit, behind, branches })

describe('tipsOf', () => {
  it('reads the local branches and how far the checked-out one is behind', () => {
    const refs = [
      { kind: 'local' as const, short: 'main', commit: A, head: true, behind: 3 },
      { kind: 'local' as const, short: 'topic', commit: B, head: false, behind: 9 },
      { kind: 'remote' as const, short: 'origin/main', commit: C, head: false, behind: 0 },
      { kind: 'tag' as const, short: 'v1', commit: C, head: false, behind: 0 },
    ]
    expect(tipsOf(refs, { branch: 'main', commit: A })).toEqual({ head: 'main', headCommit: A, behind: 3, branches: { main: A, topic: B } })
  })
})

describe('branchMove', () => {
  it('is nothing on the first reading, or when nothing moved', () => {
    const now = tips('main', A, { main: A })
    expect(branchMove(null, now)).toBeNull()
    expect(branchMove(now, now)).toBeNull()
  })

  it('follows the checked-out branch when it moves, saying where it was', () => {
    expect(branchMove(tips('main', A, { main: A }), tips('main', B, { main: B }))).toEqual({ branch: 'main', from: A, to: B })
  })

  it('follows a checkout, which brings nothing with it', () => {
    expect(branchMove(tips('main', A, { main: A, topic: B }), tips('topic', B, { main: A, topic: B }))).toEqual({ branch: 'topic', from: '', to: B })
  })

  it('follows a detached HEAD by its commit', () => {
    expect(branchMove(tips('main', A, { main: A }), tips('', B, { main: A }))).toEqual({ branch: '', from: '', to: B })
    expect(branchMove(tips('', A, { main: C }), tips('', B, { main: C }))).toEqual({ branch: '', from: A, to: B })
  })

  it('follows the one other branch that moved', () => {
    expect(branchMove(tips('main', A, { main: A, topic: B }), tips('main', A, { main: A, topic: C }))).toEqual({ branch: 'topic', from: B, to: C })
  })

  it('prefers the checked-out branch when another moved too', () => {
    expect(branchMove(tips('main', A, { main: A, topic: B }), tips('main', C, { main: C, topic: C }))?.branch).toBe('main')
  })

  it('goes nowhere when several other branches moved, or one only appeared or went', () => {
    expect(branchMove(tips('main', A, { main: A, x: B, y: B }), tips('main', A, { main: A, x: C, y: C }))).toBeNull()
    expect(branchMove(tips('main', A, { main: A }), tips('main', A, { main: A, fresh: B }))).toBeNull()
    expect(branchMove(tips('main', A, { main: A, old: B }), tips('main', A, { main: A }))).toBeNull()
  })

  it('goes nowhere when only the upstream moved, as after a fetch', () => {
    expect(branchMove(tips('main', A, { main: A }, 0), tips('main', A, { main: A }, 3))).toBeNull()
  })

  it('goes nowhere before the first commit', () => {
    expect(branchMove(tips('main', '', {}), tips('main', '', {}))).toBeNull()
  })
})

describe('isOwnMove', () => {
  it('counts a move soon after the app ran something that moves a branch', () => {
    expect(isOwnMove(0)).toBe(true)
    expect(isOwnMove(OWN_MOVE_MS - 1)).toBe(true)
  })

  it("doesn't count one long after, or when the app has moved nothing", () => {
    expect(isOwnMove(OWN_MOVE_MS)).toBe(false)
    expect(isOwnMove(-1)).toBe(false)
    expect(isOwnMove(null)).toBe(false)
    expect(isOwnMove(undefined)).toBe(false)
  })
})

describe('landingTop', () => {
  it('puts the row a third of the way down the view', () => {
    expect(LANDING).toBeCloseTo(1 / 3)
    expect(landingTop(3000, 600, 100_000)).toBe(2800)
  })

  it('stays within the history', () => {
    expect(landingTop(52, 600, 100_000)).toBe(0)
    expect(landingTop(99_900, 600, 99_500)).toBe(99_500)
  })
})

describe('headAway', () => {
  it('is null while any of the row shows', () => {
    expect(headAway(100, 52, 100, 600)).toBeNull()
    expect(headAway(100, 52, 151, 600)).toBeNull()
    expect(headAway(699, 52, 100, 600)).toBeNull()
  })

  it('says which way the row is once it has left', () => {
    expect(headAway(100, 52, 152, 600)).toBe('up')
    expect(headAway(700, 52, 100, 600)).toBe('down')
  })
})

describe('what it is called and announced', () => {
  it('names a branch, or a detached HEAD by its short SHA', () => {
    expect(headLabel('main', A)).toBe('main')
    expect(headLabel('', A)).toBe('aaaaaaa')
  })

  it('says where the branch went and what came with it', () => {
    expect(moveAnnouncement({ branch: 'main', from: A, to: B }, 3)).toBe('main moved to bbbbbbb, 3 new commits')
    expect(moveAnnouncement({ branch: 'main', from: A, to: B }, 1)).toBe('main moved to bbbbbbb, 1 new commit')
    expect(moveAnnouncement({ branch: 'main', from: A, to: B }, 0)).toBe('main moved to bbbbbbb')
    expect(moveAnnouncement({ branch: 'topic', from: '', to: B }, 0)).toBe('topic is at bbbbbbb')
  })
})
