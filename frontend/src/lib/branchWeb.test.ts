import { describe, expect, it } from 'vitest'
import { remoteBranchHint, remoteBranchOf, splitRemote, type RefExtras } from './branchWeb'

const e = (over: Partial<RefExtras> = {}): RefExtras => ({ remote: 'origin', branch: 'fix', host: '', trackedBy: [], localExists: false, ...over })

describe('branchWeb', () => {
  it('splits a remote branch at its remote, the longest name first', () => {
    expect(splitRemote('origin/fix/a', ['origin'])).toEqual({ remote: 'origin', branch: 'fix/a' })
    expect(splitRemote('team/x/fix', ['team', 'team/x'])).toEqual({ remote: 'team/x', branch: 'fix' })
  })

  it('finds the remote branch a ref is about', () => {
    const base = { name: '', commit: '', head: false, ahead: 0, behind: 0, gone: false }
    expect(remoteBranchOf({ ...base, short: 'origin/a', kind: 'remote', upstream: '' })).toBe('origin/a')
    expect(remoteBranchOf({ ...base, short: 'a', kind: 'local', upstream: 'origin/a' })).toBe('origin/a')
    expect(remoteBranchOf({ ...base, short: 'a', kind: 'local', upstream: 'origin/a', gone: true })).toBeNull()
    expect(remoteBranchOf({ ...base, short: 'a', kind: 'local', upstream: '' })).toBeNull()
    expect(remoteBranchOf({ ...base, short: 'v1', kind: 'tag', upstream: '' })).toBeNull()
  })

  it('says how far a remote branch is from the default branch, and who tracks it', () => {
    expect(remoteBranchHint('origin/fix', e({ status: { base: 'main', ahead: 2, behind: 0 }, trackedBy: ['fix'] }))).toBe('2 ahead origin/main · Tracked by fix')
    expect(remoteBranchHint('origin/fix', e({ status: { base: 'main', ahead: 0, behind: 0 } }))).toBe('Even with origin/main · No local branch tracks it')
    expect(remoteBranchHint('origin/main', e({ branch: 'main', status: { base: 'main', ahead: 0, behind: 0 }, trackedBy: ['main'] }))).toBe("origin's default branch · Tracked by main")
    expect(remoteBranchHint('origin/fix', e())).toBe('No local branch tracks it')
    expect(remoteBranchHint('origin/fix', undefined)).toBe('')
  })
})
