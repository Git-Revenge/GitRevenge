import { describe, expect, it } from 'vitest'
import { abortedMessage, abortEffect, beads, blockedBy, continueLabel, finishedMessage, opNote, opTitle, rebaseMarks, skippedAtEnd, skipLabel, stopTally, stuckNote, type OpState } from './operation'

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

describe('stopTally', () => {
  const tally = (files: [number, number][], settled: string[] = []) => ({ files: files.map(([places, suggested]) => ({ places, suggested })), settled: settled.map((driver) => ({ driver })) })
  it('counts suggestions and the places beside them', () => {
    expect(stopTally(tally([[2, 1], [1, 1]]))).toBe('2 suggestions · 1 undecided')
    expect(stopTally(tally([[1, 1]]))).toBe('1 suggestion')
  })
  it('says nothing more than the files when nothing can be suggested', () => {
    expect(stopTally(tally([[3, 0]]))).toBe('')
    expect(stopTally(null)).toBe('')
  })
  it('counts the files each merge driver settled', () => {
    expect(stopTally(tally([[3, 0]], ['mergiraf', 'mergiraf']))).toBe('2 settled by mergiraf')
    expect(stopTally(tally([[1, 1]], ['mergiraf']))).toBe('1 suggestion · 1 settled by mergiraf')
  })
})

describe('git am (#1023)', () => {
  const am = (over: Partial<OpState> = {}) =>
    state({ operation: 'am', step: 4, total: 5, ours: 'main', theirs: '', commit: '', written: [], subject: 'Set the alert limit in knots too', author: 'Tyler Brooks', applied: 3, ...over })

  it('names the branch, the patch in hand and who wrote it', () => {
    expect(opTitle(am())).toEqual({ verb: 'Applying patches', a: '', join: 'onto', b: 'main' })
    expect(opTitle(am({ ours: '' })).b).toBe('a detached HEAD')
    expect(opNote(am(), 1)).toBe('Stopped at patch 4 of 5: Set the alert limit in knots too · Tyler Brooks')
    expect(opNote(am({ conflicts: [] }), 1)).toBe('Patch 4 of 5 is resolved and staged.')
    expect(opNote(am({ subject: '', author: '' }), 1)).toBe('Stopped at patch 4 of 5.')
  })

  it('counts what Continue has left, and draws a bead a patch', () => {
    expect(continueLabel(am({ conflicts: [] }))).toBe('Continue with 1 more')
    expect(continueLabel(am({ conflicts: [], step: 5 }))).toBe('Finish applying')
    expect(beads(am())).toEqual(['done', 'done', 'done', 'bad', ''])
    expect(beads(am({ conflicts: [], stuck: true }))).toEqual(['done', 'done', 'done', 'bad', ''])
    expect(skipLabel(am())).toBe('Skip this patch')
    expect(skipLabel(state())).toBe('Skip this commit')
  })

  it("says a patch didn't apply, why, and what's left to do", () => {
    const stuck = am({ step: 2, conflicts: [], stuck: true, stuckReason: 'src/feed.js: patch does not apply', subject: 'Read the depth in metres', applied: 1 })
    expect(opNote(stuck, 0)).toBe('Patch 2 of 5 didn’t apply: Read the depth in metres · Tyler Brooks')
    expect(stuckNote(stuck, 0)).toBe('Git says “src/feed.js: patch does not apply”, and there’s nothing in conflict to resolve. Skip it, abort, or make the change by hand and stage it to bring Continue back.')
    expect(stuckNote({ ...stuck, stuckReason: '' }, 0)).toBe('There’s nothing in conflict to resolve. Skip it, abort, or make the change by hand and stage it to bring Continue back.')
    expect(stuckNote(am(), 1)).toBe('')
  })

  it('says when resolving a patch left nothing of it to commit', () => {
    const empty = am({ conflicts: [], stuck: true, stuckReason: 'patch failed: src/feed.js:4' })
    expect(opNote(empty, 1)).toBe('Nothing of patch 4 of 5 is left to commit: Set the alert limit in knots too · Tyler Brooks')
    expect(stuckNote(empty, 1)).toBe('Resolved, it changes nothing, so there’s nothing to commit. It may be in already: skip it, or abort.')
  })

  it('says what aborting undoes, and the branch made for the patches', () => {
    expect(abortEffect(am(), 1)).toBe('main goes back to ddddddd, where it was before the first patch. The 3 patches already applied and your work on the conflicts are thrown away.')
    expect(abortEffect(am({ step: 2, conflicts: [] }), 0)).toBe('main goes back to ddddddd, where it was before the first patch. The 1 patch already applied is thrown away.')
    expect(abortEffect(am({ step: 1, madeBranch: 'wind/knots', ours: 'wind/knots' }), 0)).toBe('wind/knots goes, as it was made for them, and you’re back where you were.')
    expect(abortedMessage(am())).toBe('Applying the patches stopped. main is back at ddddddd.')
    expect(abortedMessage(am({ madeBranch: 'wind/knots' }))).toBe('Applying the patches stopped, and wind/knots, made for them, is gone.')
  })

  it('counts the patches applied and skipped at the end', () => {
    // Patch 4 of 5 with 3 in: none skipped yet. Continuing finishes with all 5.
    expect(finishedMessage(am(), skippedAtEnd(am(), 'continue'))).toBe('Applied 5 patches onto main.')
    // Patch 5 of 5 with 3 in: one was skipped before, and this one too.
    const last = am({ step: 5, applied: 3 })
    expect(skippedAtEnd(last, 'skip')).toBe(2)
    expect(finishedMessage(last, 2)).toBe('Applied 3 patches onto main; skipped 2.')
    expect(finishedMessage(am({ total: 1, step: 1, applied: 0 }), 1)).toBe('No patches were applied; the one was skipped.')
    expect(finishedMessage(am({ total: 2, step: 2, applied: 0 }), 2)).toBe('No patches were applied; all 2 were skipped.')
    expect(skippedAtEnd(state(), 'skip')).toBe(0)
  })
})
