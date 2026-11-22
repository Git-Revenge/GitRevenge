import { describe, expect, it } from 'vitest'
import { progressPart, progressText } from './trailerProgress'

describe('progressText', () => {
  it('says how many of how many, once the total is counted', () => {
    expect(progressText({ read: 412000, total: 1503134 })).toBe(`${(412000).toLocaleString()} of ${(1503134).toLocaleString()} commits`)
  })

  it('says how many so far while the total is still being counted', () => {
    expect(progressText({ read: 20000, total: 0 })).toBe(`${(20000).toLocaleString()} commits`)
    expect(progressText({ read: 1, total: 0 })).toBe('1 commit')
    expect(progressText({ read: 0, total: 0 })).toBe('0 commits')
  })
})

describe('progressPart', () => {
  it('is the share read, and never more than all of it', () => {
    expect(progressPart({ read: 50, total: 200 })).toBe(0.25)
    expect(progressPart({ read: 300, total: 200 })).toBe(1)
    expect(progressPart({ read: 50, total: 0 })).toBe(0)
  })
})
