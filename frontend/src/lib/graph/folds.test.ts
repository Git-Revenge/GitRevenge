import { describe, expect, it } from 'vitest'
import { dayKey, foldDay, foldedAway, foldLabel, type Foldable } from './folds'

const noon = new Date(2026, 10, 9, 12).getTime() / 1000
const c = (index: number, kind: string | undefined, col: number, labelled = false): Foldable => ({ index, kind, col, authorTime: noon - index * 60, labelled })
const folding = new Set(['bot', 'docs'])

describe('foldDay', () => {
  it("folds a Docs lane's commits of the day into one, at the newest", () => {
    const day = [c(0, 'trunk', 0), c(1, 'docs', 1), c(2, 'trunk', 0), c(3, 'docs', 1), c(4, 'docs', 1)]
    const folds = foldDay(day, folding)
    expect([...folds.keys()]).toEqual([1, 3, 4])
    const fold = folds.get(1)!
    expect(fold).toMatchObject({ head: 1, count: 3, kind: 'docs', last: noon - 60 })
    expect(folds.get(3)).toBe(fold)
    expect(folds.get(4)).toBe(fold)
  })

  it('leaves Trunk, Release, Environment, Feature and plain lanes alone', () => {
    const day = [c(0, 'trunk', 0), c(1, 'release', 1), c(2, 'environment', 2), c(3, 'feature', 3), c(4, undefined, 4), c(5, '', 5)]
    expect(foldDay(day, folding).size).toBe(0)
  })

  it('keeps each lane apart, and each kind', () => {
    const day = [c(0, 'docs', 1), c(1, 'bot', 2), c(2, 'docs', 1), c(3, 'bot', 3), c(4, 'bot', 2)]
    const folds = foldDay(day, folding)
    expect(folds.get(0)).toMatchObject({ head: 0, count: 2, kind: 'docs' })
    expect(folds.get(1)).toMatchObject({ head: 1, count: 2, kind: 'bot' })
    expect(folds.get(3)).toMatchObject({ head: 3, count: 1, kind: 'bot' })
    expect(folds.get(1)).not.toBe(folds.get(3))
  })

  it("leaves a commit that carries a label a row of its own, such as the lane's tip", () => {
    const day = [c(0, 'docs', 1, true), c(1, 'docs', 1), c(2, 'docs', 1)]
    const folds = foldDay(day, folding)
    expect(folds.has(0)).toBe(false)
    expect(folds.get(1)).toMatchObject({ head: 1, count: 2 })
  })

  it('folds nothing when no kind folds', () => {
    expect(foldDay([c(0, 'docs', 1), c(1, 'bot', 2)], new Set()).size).toBe(0)
  })

  it('names a fold by its lane and day, so it is the same fold after a reload', () => {
    const before = foldDay([c(0, 'docs', 1), c(1, 'docs', 1)], folding).get(0)!
    // A new commit elsewhere moves every row down one.
    const after = foldDay([c(1, 'docs', 1), c(2, 'docs', 1)], folding).get(1)!
    expect(before.key).toBe(`docs:1:${dayKey(noon)}`)
    expect(after.key).toBe(before.key)
  })
})

describe('foldedAway', () => {
  const fold = foldDay([c(0, 'docs', 1), c(1, 'docs', 1)], folding).get(0)!
  const none = new Set<string>()

  it('hides the commits of a closed fold', () => {
    expect(foldedAway(fold, none, false, false)).toBe(true)
  })

  it('shows them while the fold is open', () => {
    expect(foldedAway(fold, new Set([fold.key]), false, false)).toBe(false)
  })

  it('always shows the selected commit, and a search match', () => {
    expect(foldedAway(fold, none, true, false)).toBe(false)
    expect(foldedAway(fold, none, false, true)).toBe(false)
  })

  it("doesn't hide a commit that isn't in a fold", () => {
    expect(foldedAway(null, none, false, false)).toBe(false)
    expect(foldedAway(undefined, none, false, false)).toBe(false)
  })
})

describe('foldLabel', () => {
  it('counts the commits by kind', () => {
    expect(foldLabel(11, 'Docs')).toBe('11 Docs commits')
    expect(foldLabel(1, 'Bot')).toBe('1 Bot commit')
  })
})
