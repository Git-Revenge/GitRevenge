import { describe, expect, it } from 'vitest'
import { ago, earlierTips, isSame, kindOf, looseOffer, mergesNote, nothingChanged, noteOf, placesOf, summaryOf, tipLabel, tooBigSaid } from './versions'

const pair = (over: object = {}) => ({ old: 1, new: 1, kind: 'same', subject: 'Cache the last reading', oldSubject: '', into: 0, moved: false, newId: 'b'.repeat(40), ...over })

describe('a pair', () => {
  it('says its kind as a word, with a tone', () => {
    expect(kindOf(pair())).toEqual({ word: 'the same', tone: 'same' })
    expect(kindOf(pair({ moved: true }))).toEqual({ word: 'the same, moved', tone: 'same' })
    expect(kindOf(pair({ kind: 'changed' }))).toEqual({ word: 'changed', tone: 'changed' })
    expect(kindOf(pair({ kind: 'around' }))).toEqual({ word: 'changed only around it', tone: 'around' })
    expect(kindOf(pair({ kind: 'message' }))).toEqual({ word: 'message changed', tone: 'changed' })
    expect(kindOf(pair({ kind: 'folded', new: 0, into: 1 }))).toEqual({ word: 'folded into 1', tone: 'gone' })
    expect(kindOf(pair({ kind: 'dropped', new: 0 }))).toEqual({ word: 'dropped', tone: 'gone' })
    expect(kindOf(pair({ kind: 'new', old: 0 }))).toEqual({ word: 'new', tone: 'new' })
  })

  it('notes what its kind leaves out', () => {
    expect(noteOf(pair())).toBe('')
    expect(noteOf(pair({ kind: 'message', oldSubject: 'Report wind in metres' }))).toBe('was “Report wind in metres”')
    expect(noteOf(pair({ kind: 'dropped', new: 0 }))).toBe('only in the old version')
    expect(noteOf(pair({ kind: 'new', old: 0 }))).toBe('only in the new version')
    expect(noteOf(pair({ moved: true, old: 4, new: 1 }))).toBe('was 4th, now 1st')
    expect(noteOf(pair({ moved: true, old: 12, new: 22 }))).toBe('was 12th, now 22nd')
    expect(noteOf(pair({ kind: 'around' }))).toBe('the same change; lines around it changed')
  })

  it('names both places', () => {
    expect(placesOf(pair({ old: 4, new: 3 }))).toBe('4 → 3')
    expect(placesOf(pair({ old: 3, new: 0 }))).toBe('3 → –')
  })

  it('hides the same and the changed only around it', () => {
    expect([pair(), pair({ kind: 'around' }), pair({ kind: 'changed' }), pair({ kind: 'new' })].map(isSame)).toEqual([true, true, false, false])
  })
})

describe('the summary', () => {
  it('counts then and now, and what became of them', () => {
    const demo = [pair({ kind: 'changed' }), pair({ kind: 'around' }), pair({ kind: 'folded' }), pair(), pair()]
    expect(summaryOf(demo, 5, 4)).toBe('5 commits then, 4 now: 1 changed, 1 changed only around it, 2 the same, 1 folded in.')
    expect(summaryOf([pair({ kind: 'message' }), pair({ kind: 'dropped' }), pair({ kind: 'new' })], 2, 2)).toBe('2 commits then, 2 now: 1 with a new message, 1 dropped, 1 new.')
    expect(summaryOf([], 0, 0)).toBe('0 commits then, 0 now.')
  })

  it('counts the merges left out', () => {
    expect(mergesNote(0, 0)).toBe('')
    expect(mergesNote(1, 2)).toBe('3 merges left out, as range-diff leaves them out.')
    expect(mergesNote(1, 0)).toBe('1 merge left out, as range-diff leaves them out.')
  })
})

describe('earlier tips', () => {
  it('are named for what moved the branch away from them', () => {
    expect(tipLabel('rebase (finish): returning to refs/heads/topic')).toBe('before the rebase')
    expect(tipLabel('commit (amend): Fall back')).toBe('before the amend')
    expect(tipLabel('reset: moving to HEAD~1')).toBe('before the reset')
    expect(tipLabel('commit: debugging')).toBe('before the next commit')
    expect(tipLabel('pull: Fast-forward')).toBe('before the pull')
    expect(tipLabel('branch: Created from main')).toBe('earlier')
  })

  it('say how long ago', () => {
    const now = 1_800_000_000
    expect(ago(now - 30, now)).toBe('just now')
    expect(ago(now - 240, now)).toBe('4 min ago')
    expect(ago(now - 7200, now)).toBe('2 h ago')
    expect(ago(now - 90000, now)).toBe('yesterday')
    expect(ago(now - 3 * 86400, now)).toBe('3 days ago')
  })
})

describe('earlierTips', () => {
  const at = (commit: string, action: string, time: number) => ({ commit, action, time })
  it('names the tips a rebase, an amend or a reset moved the branch away from', () => {
    const log = [
      at('e', 'rebase (finish): refs/heads/topic onto c', 500),
      at('d', 'commit (amend): Fall back', 400),
      at('c', 'commit: Fall back', 300),
      at('b', 'reset: moving to HEAD~1', 200),
      at('a', 'commit: wip', 100),
      at('0', 'branch: Created from main', 50),
    ]
    expect(earlierTips(log)).toEqual([
      { commit: 'd', said: 'Before the rebase', time: 500 },
      { commit: 'c', said: 'Before the amend', time: 400 },
      { commit: 'a', said: 'Before the reset', time: 200 },
    ])
    expect(earlierTips(log, 1)).toHaveLength(1)
  })

  it('leaves out plain commits, the tip it is at, and repeats', () => {
    expect(earlierTips([at('b', 'commit: two', 2), at('a', 'commit: one', 1)])).toEqual([])
    expect(earlierTips([at('a', 'reset: moving to a', 3), at('a', 'commit: one', 2)])).toEqual([])
    expect(earlierTips([])).toEqual([])
  })
})

describe('the edges', () => {
  const c = (pairs: object[], more: object = {}) => ({ pairs: pairs as never[], old: { tip: 'a' }, new: { tip: 'b' }, loose: false, ...more })

  it('offers matching more loosely when a commit was dropped and one is new', () => {
    expect(looseOffer(c([{ kind: 'dropped' }, { kind: 'new' }]))).toBe('1 dropped and 1 new, which may be one commit reworked a lot.')
    expect(looseOffer(c([{ kind: 'dropped' }, { kind: 'dropped' }, { kind: 'new' }]))).toBe('2 dropped and 1 new, which may be commits reworked a lot.')
    expect(looseOffer(c([{ kind: 'dropped' }]))).toBe('')
    expect(looseOffer(c([{ kind: 'dropped' }, { kind: 'new' }], { loose: true }))).toBe('')
  })

  it('says when nothing changed but the ids', () => {
    expect(nothingChanged(c([{ kind: 'same' }, { kind: 'same' }]))).toBe('All 2 commits are the same in both versions: only their ids changed.')
    expect(nothingChanged(c([{ kind: 'same' }, { kind: 'same', moved: true }]))).toBe('')
    expect(nothingChanged(c([{ kind: 'same' }, { kind: 'around' }]))).toBe('')
    expect(nothingChanged(c([], { old: { tip: 'a' }, new: { tip: 'a' } }))).toBe('Both versions are the same commit, so nothing changed.')
  })

  it('asks before comparing a big version', () => {
    expect(tooBigSaid(1240, 3)).toBe('The old version has 1,240 commits. Compare them anyway?')
    expect(tooBigSaid(3, 700)).toBe('The new version has 700 commits. Compare them anyway?')
  })
})
