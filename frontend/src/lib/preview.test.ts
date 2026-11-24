import { describe, expect, it } from 'vitest'
import { mergeChoices, mergeSummary, mergeVerb, placesNote, plural, rebaseVerb } from './preview'

const merge = (over: object = {}) => ({ fastForward: false, conflicts: [] as { path: string; places: number; combinable: number }[], unknown: false, intoOnly: 2, commitCount: 3, ...over })
const conflicted = merge({ conflicts: [{ path: 'a', places: 2, combinable: 0 }, { path: 'b', places: 1, combinable: 0 }] })

describe('mergeChoices', () => {
  it('offers a fast-forward first when one is possible', () => {
    const choices = mergeChoices(merge({ fastForward: true, intoOnly: 0 }), 'main', 'topic')
    expect(choices.map((c) => c.off)).toEqual([false, false, false])
    expect(choices[0].how).toBe('ff-only')
  })

  it('turns fast-forward off, and says why, when the branch has commits of its own', () => {
    const [ff, commit] = mergeChoices(merge(), 'main', 'topic')
    expect(ff.off).toBe(true)
    expect(ff.why).toContain("main has 2 commits topic doesn't")
    expect(commit).toMatchObject({ how: 'no-ff', off: false })
  })

  it('turns squash off when files would conflict', () => {
    const squash = mergeChoices(conflicted, 'main', 'topic').find((c) => c.how === 'squash')!
    expect(squash.off).toBe(true)
    expect(squash.why).not.toBe('')
  })
})

describe('mergeVerb', () => {
  it('says what the button will do', () => {
    expect(mergeVerb(merge(), 'no-ff', 'main')).toBe('Merge')
    expect(mergeVerb(merge({ fastForward: true }), 'ff-only', 'main')).toBe('Fast-forward main')
    expect(mergeVerb(merge(), 'squash', 'main')).toBe('Squash into main')
    expect(mergeVerb(conflicted, 'no-ff', 'main')).toBe('Merge, and resolve 2 conflicts')
    expect(mergeVerb(merge({ conflicts: [{ path: 'a', places: 1, combinable: 0 }] }), 'no-ff', 'main')).toBe('Merge, and resolve 1 conflict')
  })

  it("doesn't promise conflicts it couldn't check", () => {
    expect(mergeVerb(merge({ unknown: true }), 'no-ff', 'main')).toBe('Merge')
  })
})

describe('mergeSummary', () => {
  it('says what each way makes', () => {
    expect(mergeSummary(merge(), 'no-ff', 'main', 'topic')).toContain('makes a merge commit on main')
    expect(mergeSummary(merge({ fastForward: true }), 'ff-only', 'main', 'topic')).toContain('no merge commit')
    expect(mergeSummary(merge({ fastForward: true }), 'no-ff', 'main', 'topic')).toContain('makes a merge commit anyway')
    expect(mergeSummary(merge(), 'squash', 'main', 'topic')).toContain('Nothing is committed yet')
  })
})

describe('rebaseVerb and plural', () => {
  it('counts in words', () => {
    expect(plural(1, 'file', 'files')).toBe('1 file')
    expect(plural(1200, 'file', 'files')).toBe(`${(1200).toLocaleString()} files`)
    expect(rebaseVerb({ commitCount: 5 }, false)).toBe('Rebase 5 commits')
    expect(rebaseVerb({ commitCount: 1 }, false)).toBe('Rebase 1 commit')
    expect(rebaseVerb({ commitCount: 5 }, true)).toBe('Choose what to do with each commit')
  })
})

describe('placesNote', () => {
  it('counts the places, and says which look combinable', () => {
    expect(placesNote({ places: 0, combinable: 0 })).toBe('')
    expect(placesNote({ places: 2, combinable: 0 })).toBe('2 places')
    expect(placesNote({ places: 1, combinable: 1 })).toBe('1 place, looks combinable')
    expect(placesNote({ places: 2, combinable: 2 })).toBe('2 places, both look combinable')
    expect(placesNote({ places: 3, combinable: 3 })).toBe('3 places, all look combinable')
    expect(placesNote({ places: 3, combinable: 1 })).toBe('3 places, 1 looks combinable')
  })
})
