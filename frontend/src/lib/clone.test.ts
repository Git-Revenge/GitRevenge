import { describe, expect, it } from 'vitest'
import { childrenOf, cloneOptions, cloneSummary, defaultPicks, depthOf, filesWritten, outermost, picksProblem, within, type Folder } from './clone'

const day = (y: number, m: number, d: number) => Math.floor(new Date(y, m - 1, d).getTime() / 1000)

describe('the clone options', () => {
  it('start as a plain git clone', () => {
    const p = defaultPicks('2025-11-26')
    expect(cloneOptions(p)).toEqual({ depth: 0, since: 0, partial: false, branch: '', submodules: false, folders: false })
    expect(cloneSummary(p)).toBe('as git clone does')
    expect(picksProblem(p)).toBe('')
  })

  it('send what each choice means', () => {
    const p = { ...defaultPicks(), history: 'depth' as const, depth: '1,000', contents: 'partial' as const, branches: 'one' as const, branch: ' master ', submodules: true }
    expect(cloneOptions(p)).toEqual({ depth: 1000, since: 0, partial: true, branch: 'master', submodules: true, folders: false })
    expect(cloneSummary(p)).toBe('the last 1,000 commits · contents when needed · only master · with submodules')
  })

  it('send a day as the start of that day where you are', () => {
    const p = { ...defaultPicks(), history: 'since' as const, since: '2025-11-26' }
    expect(cloneOptions(p).since).toBe(day(2025, 11, 26))
    expect(cloneSummary(p)).toBe('since Nov 26, 2025')
    // Only the chosen history is sent.
    expect(cloneOptions({ ...p, history: 'all' })).toMatchObject({ depth: 0, since: 0 })
  })

  it('read a number of commits as people type it', () => {
    expect(depthOf('50')).toBe(50)
    expect(depthOf(' 1,000 ')).toBe(1000)
    expect(depthOf('1 000')).toBe(1000)
    for (const bad of ['', '0', '-3', '2.5', 'ten', '1e3']) expect(depthOf(bad)).toBe(0)
    // With every branch, git counts back from each one's tip.
    expect(cloneSummary({ ...defaultPicks(), history: 'depth', depth: '1' })).toBe('the last commit of each branch')
    expect(cloneSummary({ ...defaultPicks(), history: 'depth', depth: '20' })).toBe('the last 20 commits of each branch')
    expect(cloneSummary({ ...defaultPicks(), history: 'depth', depth: '20', branches: 'one', branch: 'main' })).toBe('the last 20 commits · only main')
  })

  it('say what stops a clone', () => {
    const now = new Date(2026, 10, 26)
    expect(picksProblem({ ...defaultPicks(), history: 'depth', depth: '0' }, now)).toMatch(/whole number/)
    expect(picksProblem({ ...defaultPicks(), history: 'since', since: '' }, now)).toMatch(/Choose the day/)
    expect(picksProblem({ ...defaultPicks(), history: 'since', since: '2027-01-01' }, now)).toMatch(/future/)
    expect(picksProblem({ ...defaultPicks(), branches: 'one', branch: ' ' }, now)).toMatch(/Name the branch/)
    expect(cloneSummary({ ...defaultPicks(), branches: 'one' })).toBe('one branch')
  })

  it('send some folders, for the second step', () => {
    const p = { ...defaultPicks(), folders: 'some' as const }
    expect(cloneOptions(p).folders).toBe(true)
    expect(cloneSummary(p)).toBe('some folders')
  })
})

// The kernel at the bench's HEAD (#935's design): 97,310 files, 18 of them at the top level.
const kernel: Folder[] = [
  { path: '', files: 97310, direct: 18 },
  { path: 'arch', files: 18949, direct: 2 },
  { path: 'arch/arm', files: 4634, direct: 0 },
  { path: 'arch/x86', files: 1401, direct: 0 },
  { path: 'arch/x86/boot', files: 120, direct: 30 },
  { path: 'include', files: 6764, direct: 0 },
  { path: 'kernel', files: 754, direct: 0 },
  { path: 'mm', files: 208, direct: 0 },
]

describe('choosing folders', () => {
  it('counts what cone mode writes: the folders, the top level, and the files directly above', () => {
    // The design's example: four folders bring 9,147 files, 2 of them directly in arch.
    expect(filesWritten(kernel, ['arch/x86', 'include', 'kernel', 'mm'])).toBe(9147)
    expect(filesWritten(kernel, [])).toBe(18)
    // A folder inside a chosen one adds nothing it didn't bring.
    expect(filesWritten(kernel, ['arch', 'arch/x86/boot'])).toBe(18 + 18949)
    // Two levels down: both folders above give their own files.
    expect(filesWritten(kernel, ['arch/x86/boot'])).toBe(18 + 120 + 2 + 0)
  })

  it('keeps only the outermost of nested choices', () => {
    expect(outermost(['arch/x86', 'arch', 'mm', 'mm'])).toEqual(['arch', 'mm'])
    expect(within('arch/x86/boot', ['arch'])).toBe(true)
    expect(within('archive', ['arch'])).toBe(false)
  })

  it('lists a folder’s own folders', () => {
    expect(childrenOf(kernel, '').map((f) => f.path)).toEqual(['arch', 'include', 'kernel', 'mm'])
    expect(childrenOf(kernel, 'arch').map((f) => f.path)).toEqual(['arch/arm', 'arch/x86'])
  })
})
