import { describe, expect, it } from 'vitest'
import { day, dueSaid, folderNames, garbageSaid, isWhole, moreHistory, partialWords, shallowLines, sizeWords, sparseLines, storageLines } from './thisClone'

const NOV_26_2025 = Date.UTC(2025, 10, 26, 12) / 1000

describe('this clone', () => {
  it('shows only for a clone that differs', () => {
    expect(isWhole({})).toBe(true)
    expect(isWhole(null)).toBe(true)
    expect(isWhole({ partial: { remote: 'origin', filter: 'blob:none' } })).toBe(false)
  })

  it('says how far back a shallow history goes', () => {
    expect(day(NOV_26_2025)).toBe('Nov 26, 2025')
    expect(shallowLines({ since: NOV_26_2025, commits: 92758 })).toEqual({ what: 'History since Nov 26, 2025', tag: '92,758 commits · shallow' })
    expect(shallowLines({ since: NOV_26_2025, commits: 1 }).tag).toBe('1 commit · shallow')
  })

  it('offers a year more, 1,000 more, or all of it', () => {
    const choices = moreHistory(NOV_26_2025)
    expect(choices.map((c) => c.label)).toEqual(['A year more, since Nov 26, 2024', '1,000 commits more', 'All of it'])
    expect(choices[0].more.since).toBe(NOV_26_2025 - 365 * 86400)
    expect(choices[1].more).toEqual({ commits: 1000, since: 0, all: false })
    expect(choices[2].more.all).toBe(true)
  })

  it('says what a partial clone downloads when needed, by its filter', () => {
    expect(partialWords('blob:none')).toBe("File contents when they're needed")
    expect(partialWords('blob:limit=1m')).toBe("Files over 1 MB when they're needed")
    expect(partialWords('blob:limit=500')).toBe("Files over 500 bytes when they're needed")
    expect(partialWords('tree:0')).toBe("Folders and files when they're needed")
  })

  it('counts a sparse checkout’s folders and files', () => {
    expect(sparseLines({ cone: true, folders: ['arch/x86', 'include', 'kernel', 'mm'], files: 9147, total: 97310 })).toEqual({ what: '4 folders checked out', tag: '9,147 of 97,310 files · sparse' })
    expect(sparseLines({ cone: false, folders: ['/*.md'], files: 3, total: 10 }).what).toBe('1 pattern checked out')
    expect(folderNames(['a', 'b', 'c'])).toBe('a · b · c')
    expect(folderNames(['a', 'b', 'c', 'd'], 2)).toBe('a · b and 2 more')
  })

  it('writes sizes as people read them', () => {
    expect(sizeWords(2.36 * (1 << 30))).toBe('2.4 GB')
    expect(sizeWords(310 * (1 << 20))).toBe('310 MB')
    expect(sizeWords(12_000)).toBe('12 KB')
    expect(sizeWords(10)).toBe('1 KB')
  })
})

describe('storage', () => {
  const s = (more: object = {}) => ({ bytes: 3 * 2 ** 30, loose: 120, packs: 2, garbage: 0, gcAuto: 6700, autoPackLimit: 50, due: false, ...more })

  it('says what the objects take, and how they’re kept', () => {
    expect(storageLines(s())).toEqual({ what: '3.0 GB on disk', tag: '120 loose objects · 2 packs' })
    expect(storageLines(s({ bytes: 0, loose: 1, packs: 1 }))).toEqual({ what: 'Nothing on disk', tag: '1 loose object · 1 pack' })
  })

  it('says why maintenance is due, in git’s terms, and nothing otherwise', () => {
    expect(dueSaid(s())).toBe('')
    expect(dueSaid(s({ loose: 9000, due: true }))).toBe('More than 6,700 objects are loose (gc.auto), so git would pack them.')
    expect(dueSaid(s({ packs: 60, due: true }))).toBe('More than 50 packs (gc.autoPackLimit), so git would join them.')
  })

  it('names stray files only when there are some', () => {
    expect(garbageSaid({ garbage: 0 })).toBe('')
    expect(garbageSaid({ garbage: 2 })).toBe('2 stray files in the object folder, such as a pack left half written.')
  })
})
