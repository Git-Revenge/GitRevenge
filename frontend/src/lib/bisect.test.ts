import { describe, expect, it } from 'vitest'
import { expectedSteps, marksOf } from './bisect.svelte'

const info = (over: object = {}) =>
  ({
    running: true,
    current: 'c',
    marks: [
      { commit: 'z', subject: 'Break', term: 'bad', step: 0 },
      { commit: 'a', subject: 'Works', term: 'good', step: 0 },
      { commit: 'm', subject: 'Mid', term: 'skip', step: 1 },
      { commit: 'm', subject: 'Mid', term: 'bad', step: 2 },
    ],
    steps: 2,
    remaining: 1,
    stepsLeft: 1,
    firstBad: '',
    candidates: [],
    origin: 'main',
    inQuestion: ['m', 'c', 'b'],
    rangeCapped: false,
    ...over,
  }) as never

describe('marksOf', () => {
  it('is nothing when no bisect is running', () => {
    expect(marksOf(null)).toBeNull()
    expect(marksOf(info({ running: false }))).toBeNull()
  })

  it('keeps the last answer given for a commit', () => {
    const m = marksOf(info())!
    expect(m.terms).toEqual({ z: 'bad', a: 'good', m: 'bad' })
    expect(m.current).toBe('c')
    expect(m.inQuestion).toEqual(['m', 'c', 'b'])
  })

  it('has no range when there were too many commits to list', () => {
    expect(marksOf(info({ rangeCapped: true }))!.inQuestion).toBeNull()
  })

  it('carries the answer once found', () => {
    expect(marksOf(info({ firstBad: 'm', current: '' }))!.found).toBe('m')
  })
})

describe('expectedSteps', () => {
  it('adds what git says is left to what has been answered', () => {
    expect(expectedSteps(info())).toBe(3)
    expect(expectedSteps(info({ steps: 0, stepsLeft: 3 }))).toBe(3)
  })

  it('counts at least one more while a commit is under test', () => {
    expect(expectedSteps(info({ steps: 2, stepsLeft: 0 }))).toBe(3)
  })

  it('is the steps taken once the answer is known', () => {
    expect(expectedSteps(info({ firstBad: 'm', steps: 3, stepsLeft: 0 }))).toBe(3)
    expect(expectedSteps(info({ candidates: ['m', 'c'], steps: 1, stepsLeft: 0 }))).toBe(1)
  })
})
