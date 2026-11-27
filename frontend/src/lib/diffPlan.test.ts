import { describe, expect, it } from 'vitest'
import { appliedMessage, applyLabel, blockedBy, diffSummary, listed, rowStatus, undoLabel } from './diffPlan'

const file = (over: object = {}) => ({ path: 'src/feed.js', created: false, removed: false, ...over })

describe('rowStatus', () => {
  it('says what applying does to each file', () => {
    expect(rowStatus(file(), { status: 'applies' }, false, false)).toEqual({ text: 'applies', tone: 'ok', can: true })
    expect(rowStatus(file({ created: true }), { status: 'applies' }, false, false).text).toBe('a new file')
    expect(rowStatus(file({ removed: true }), { status: 'applies' }, false, false).text).toBe('deleted')
    expect(rowStatus(file({ oldPath: 'src/old.js' }), { status: 'applies' }, false, false).text).toBe('renamed from src/old.js')
    expect(rowStatus(file(), { status: 'merges' }, false, false).text).toBe('applies by merging')
    expect(rowStatus(file(), { status: 'conflicts' }, false, false)).toEqual({ text: 'merges with a conflict', tone: 'warn', can: true })
    expect(rowStatus(file(), undefined, false, false).text).toBe('trying…')
  })

  it("can't tick a file that doesn't fit or is in already", () => {
    expect(rowStatus(file(), { status: 'fails' }, false, false)).toEqual({ text: 'doesn’t fit', tone: 'warn', can: false })
    expect(rowStatus(file(), { status: 'applied' }, false, false)).toEqual({ text: 'in already', tone: 'mute', can: false })
  })

  it('says each the other way round in reverse', () => {
    expect(rowStatus(file({ created: true }), { status: 'applies' }, true, false).text).toBe('deleted')
    expect(rowStatus(file({ removed: true }), { status: 'applies' }, true, false).text).toBe('comes back')
    expect(rowStatus(file({ oldPath: 'src/old.js' }), { status: 'applies' }, true, false).text).toBe('renamed back to src/old.js')
    expect(rowStatus(file(), { status: 'applied' }, true, false).text).toBe('not here to take out')
  })

  it('puts changes of yours first', () => {
    expect(rowStatus(file(), { status: 'applies' }, false, true)).toEqual({ text: 'has your changes', tone: 'warn', can: true })
  })
})

describe('blockedBy', () => {
  it('names ticked files with changes of yours', () => {
    expect(blockedBy(['a.js', 'b.js'], ['a.js'], {}, 'files')).toBe('a.js has changes of yours. Commit or stash them first, so the diff can be undone cleanly, or leave it out.')
    expect(blockedBy(['a.js', 'b.js', 'c.js'], ['a.js', 'b.js', 'c.js'], {}, 'files')).toBe('a.js and 2 others have changes of yours. Commit or stash them first, so the diff can be undone cleanly, or leave them out.')
    expect(blockedBy(['b.js'], ['a.js'], {}, 'files')).toBe('')
  })

  it('refuses a conflict only when staging only', () => {
    const tries = { 'a.js': { status: 'conflicts' } }
    expect(blockedBy(['a.js'], [], tries, 'cached')).toBe('a.js would merge with a conflict, which staging only can’t hold. Apply to the files, or leave it out.')
    expect(blockedBy(['a.js'], [], tries, 'files')).toBe('')
    expect(blockedBy(['a.js'], [], tries, 'index')).toBe('')
  })
})

describe('the words around it', () => {
  it('names files', () => {
    expect(listed(['a.js'])).toBe('a.js')
    expect(listed(['a.js', 'b.js'])).toBe('a.js and b.js')
    expect(listed(['a.js', 'b.js', 'c.js', 'd.js'])).toBe('a.js and 3 others')
  })

  it('says what Apply does and what happened', () => {
    expect(applyLabel(1, false)).toBe('Apply to 1 file')
    expect(applyLabel(4, true)).toBe('Take out of 4 files')
    expect(appliedMessage({ files: 4, conflicts: [] }, false)).toBe('Applied the diff to 4 files.')
    expect(appliedMessage({ files: 1, conflicts: [] }, true)).toBe('Took the diff out of 1 file.')
    expect(appliedMessage({ files: 4, conflicts: ['a.js'] }, false)).toBe('Applied the diff to 4 files; a.js has a conflict to resolve in Changes.')
    expect(appliedMessage({ files: 4, conflicts: ['a.js', 'b.js'] }, false)).toBe('Applied the diff to 4 files; 2 have conflicts to resolve in Changes.')
    expect(undoLabel({ files: 4, conflicts: [] })).toBe('Undo')
    expect(undoLabel({ files: 4, conflicts: ['a.js'] })).toBe('Put these files back')
  })

  it('sums the diff up', () => {
    expect(diffSummary({ files: [{ added: 3, deleted: 1 }, { added: 12, deleted: 0 }] })).toBe('a plain diff · 2 files · +15 −1')
  })
})
