import { describe, expect, it } from 'vitest'
import { abortedMessage, abortEffect, beads, blockedBy, continueLabel, finishedMessage, opNote, opTitle, rebaseMarks, type OpState } from './operation'

const id = (c: string) => c.repeat(40)
const state = (over: Partial<OpState> = {}): OpState => ({ operation: 'rebase', step: 3, total: 5, conflicts: ['src/feed.js'], ours: 'feature/offline-cache', theirs: 'main', commit: id('c'), subject: 'Fall back to the cached reading', origHead: id('d'), written: [id('b'), id('a')], ...over })
const merge = (over: Partial<OpState> = {}) => state({ operation: 'merge', step: 0, total: 0, ours: 'main', theirs: 'feature/metric-units', written: [], ...over })

describe('opTitle', () => {
  it('names the operation and both sides', () => {
    expect(opTitle(state())).toEqual({ verb: 'Rebasing', a: 'feature/offline-cache', join: 'onto', b: 'main' })
    expect(opTitle(merge())).toEqual({ verb: 'Merging', a: 'feature/metric-units', join: 'into', b: 'main' })
    expect(opTitle(state({ operation: 'cherry-pick', theirs: '' }))).toEqual({ verb: 'Cherry-picking', a: 'ccccccc', join: 'onto', b: 'feature/offline-cache' })
    expect(opTitle(state({ operation: 'revert', theirs: '' }))).toMatchObject({ verb: 'Reverting', join: 'on' })
  })

  it('says so when HEAD is detached', () => {
    expect(opTitle(state({ ours: '' })).a).toBe('a detached HEAD')
  })
})

describe('opNote', () => {
  it('says where a rebase stopped, and when that commit is ready', () => {
    expect(opNote(state(), 1)).toBe('Stopped at commit 3 of 5: Fall back to the cached reading')
    expect(opNote(state({ conflicts: [] }), 1)).toBe('Commit 3 of 5 is resolved and staged.')
  })

  it('counts a merge down as files are resolved', () => {
    expect(opNote(merge({ conflicts: ['a', 'b'] }), 2)).toBe('2 files need resolving before the merge can be committed.')
    expect(opNote(merge({ conflicts: ['a'] }), 2)).toBe('1 of 2 files resolved.')
    expect(opNote(merge({ conflicts: [] }), 2)).toBe('All 2 conflicts are resolved. Commit the merge to finish.')
    expect(opNote(merge({ conflicts: [] }), 1)).toBe('The conflict is resolved. Commit the merge to finish.')
  })

  it('names the commit of a cherry-pick, which has no steps', () => {
    expect(opNote(state({ operation: 'cherry-pick', step: 0, total: 0 }), 1)).toBe('Stopped at ccccccc: Fall back to the cached reading')
  })
})

describe('continueLabel and blockedBy', () => {
  it('says what Continue will do', () => {
    expect(continueLabel(state())).toBe('Continue with 2 more')
    expect(continueLabel(state({ step: 5 }))).toBe('Finish the rebase')
    expect(continueLabel(merge())).toBe('Commit the merge')
    expect(continueLabel(state({ operation: 'revert' }))).toBe('Finish the revert')
  })

  it('names what is left to resolve', () => {
    expect(blockedBy(state())).toBe('Resolve src/feed.js first')
    expect(blockedBy(state({ conflicts: ['a', 'b', 'c'] }))).toBe('Resolve a and 2 others first')
    expect(blockedBy(state({ conflicts: [] }))).toBe('')
  })
})

describe('abortEffect', () => {
  it('says where a rebase goes back to and what is thrown away', () => {
    expect(abortEffect(state(), 1)).toBe('feature/offline-cache goes back to ddddddd, where it was before. The 2 commits already written again and your work on the conflicts are thrown away.')
    expect(abortEffect(state({ written: [] }), 1)).toBe('feature/offline-cache goes back to ddddddd, where it was before. Your work on the conflicts is thrown away.')
    expect(abortEffect(state({ written: [], conflicts: [] }), 0)).toBe('feature/offline-cache goes back to ddddddd, where it was before.')
  })

  it('says a merge leaves the branch where it was', () => {
    expect(abortEffect(merge(), 1)).toBe('main stays at ddddddd, as it was before the merge. Your work on the conflicts is thrown away.')
  })
})

describe('afterwards', () => {
  it('says what happened', () => {
    expect(finishedMessage(state())).toBe('Rebased feature/offline-cache onto main.')
    expect(finishedMessage(merge())).toBe('Merged feature/metric-units into main.')
    expect(abortedMessage(state())).toBe('Rebase aborted. feature/offline-cache is back at ddddddd.')
  })
})

describe('beads and rebaseMarks', () => {
  it('gives a rebase a bead per step', () => {
    expect(beads(state())).toEqual(['done', 'done', 'bad', '', ''])
    expect(beads(state({ conflicts: [] }))).toEqual(['done', 'done', 'now', '', ''])
    expect(beads(merge())).toEqual([])
    expect(beads(state({ total: 40 }))).toEqual([])
  })

  it('marks the commits written and the one in hand for the graph', () => {
    const marks = rebaseMarks(state())!
    expect(marks.terms).toEqual({ [id('b')]: 'good', [id('a')]: 'good' })
    expect(marks.current).toBe(id('c'))
    expect(marks.currentNote).toBe('3 of 5, stopped here')
    expect(marks.currentGlyph).toBe('3')
    expect(rebaseMarks(state({ step: 12, total: 20 }))!.currentGlyph).toBe('…')
    expect(marks.inQuestion).toBeNull()
    expect(rebaseMarks(merge())).toBeNull()
    expect(rebaseMarks(null)).toBeNull()
  })
})
