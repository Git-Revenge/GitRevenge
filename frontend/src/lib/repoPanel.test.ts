import { describe, expect, it } from 'vitest'
import { canOpenWorktree, canRemoveWorktree, folderName, lfsNames, matchesFilter, submoduleStatus, worktreeNames, worktreeStatus, worktreeTooltip, type WorktreeLike } from './repoPanel'

const tree = (extra: Partial<WorktreeLike> = {}): WorktreeLike => ({ path: 'A:/code/repo-fix', head: 'abcdef1234', branch: 'fix/x', main: false, current: false, locked: false, prunable: false, ...extra })

describe('folderName', () => {
  it('takes the last folder on any platform', () => {
    expect(folderName('A:\\code\\repo')).toBe('repo')
    expect(folderName('/home/me/repo')).toBe('repo')
    expect(folderName('/home/me/repo/')).toBe('repo')
    expect(folderName('repo')).toBe('repo')
  })
})

describe('worktreeStatus', () => {
  it('shows the branch', () => {
    expect(worktreeStatus(tree())).toEqual({ text: 'fix/x', tone: 'muted' })
  })

  it('says when HEAD is detached', () => {
    expect(worktreeStatus(tree({ branch: '' })).text).toBe('detached')
  })

  it('puts a problem above the branch, missing first', () => {
    expect(worktreeStatus(tree({ locked: true })).text).toBe('locked')
    expect(worktreeStatus(tree({ locked: true, prunable: true }))).toEqual({ text: 'missing', tone: 'warning' })
  })
})

describe('worktreeTooltip', () => {
  it('lists the path and branch', () => {
    expect(worktreeTooltip(tree())).toBe('A:/code/repo-fix\nOn fix/x')
  })

  it('gives the commit when detached', () => {
    expect(worktreeTooltip(tree({ branch: '' })).split('\n')[1]).toBe('Detached at abcdef1')
  })

  it('lists everything else', () => {
    expect(worktreeTooltip(tree({ main: true, current: true, locked: true, prunable: true })).split('\n').slice(2)).toEqual(['The main worktree', 'Open in this window', 'Locked', 'Missing on disk'])
  })
})

describe('what a worktree can do', () => {
  it('opens unless it is this window or missing', () => {
    expect(canOpenWorktree(tree())).toBe(true)
    expect(canOpenWorktree(tree({ current: true }))).toBe(false)
    expect(canOpenWorktree(tree({ prunable: true }))).toBe(false)
  })

  it('is removable unless it is the main worktree or this window', () => {
    expect(canRemoveWorktree(tree())).toBe(true)
    expect(canRemoveWorktree(tree({ prunable: true }))).toBe(true)
    expect(canRemoveWorktree(tree({ main: true }))).toBe(false)
    expect(canRemoveWorktree(tree({ current: true }))).toBe(false)
  })
})

describe('submoduleStatus', () => {
  it('puts states into words', () => {
    expect(submoduleStatus('uninitialized')).toBe('not initialized')
    expect(submoduleStatus('modified')).toBe('different commit')
    expect(submoduleStatus('conflict')).toBe('conflict')
  })

  it('says nothing when all is well', () => {
    expect(submoduleStatus('ok')).toBe('')
  })
})

describe('matchesFilter', () => {
  it('matches any name, ignoring case and spaces', () => {
    expect(matchesFilter(' FIX ', 'a', 'fix/x')).toBe(true)
    expect(matchesFilter('nope', 'a', 'b')).toBe(false)
  })

  it('matches everything with no query', () => {
    expect(matchesFilter('  ', 'a')).toBe(true)
  })

  it('finds a worktree by folder or branch', () => {
    expect(matchesFilter('repo-fix', ...worktreeNames(tree()))).toBe(true)
    expect(matchesFilter('fix/x', ...worktreeNames(tree()))).toBe(true)
  })

  it('finds the LFS row by name, pattern or file', () => {
    const names = lfsNames([{ pattern: '*.psd' }], [{ name: 'art/logo.psd' }])
    expect(matchesFilter('lfs', ...names)).toBe(true)
    expect(matchesFilter('*.psd', ...names)).toBe(true)
    expect(matchesFilter('logo', ...names)).toBe(true)
    expect(matchesFilter('readme', ...names)).toBe(false)
  })
})
