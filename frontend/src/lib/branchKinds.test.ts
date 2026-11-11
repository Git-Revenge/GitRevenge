import { describe, expect, it } from 'vitest'
import { CONFIRM_DELETE, CONFIRM_FORCE_PUSH, foldingKinds, kindFilterRows, kindInfo, kindWarning } from './branchKinds'

describe('kindFilterRows', () => {
  it('lists the kinds the repository has, in the usual order', () => {
    const rows = kindFilterRows([{ branchKind: 'feature' }, { branchKind: 'trunk' }, { branchKind: '' }, {}], [])
    expect(rows.map((r) => r.kind)).toEqual(['trunk', 'feature'])
    expect(rows.every((r) => !r.hidden)).toBe(true)
  })

  it('keeps a hidden kind, so it can come back with no branches left to show', () => {
    const rows = kindFilterRows([{ branchKind: 'trunk' }], ['docs'])
    expect(rows.map((r) => [r.kind, r.hidden])).toEqual([
      ['trunk', false],
      ['docs', true],
    ])
  })

  it('ignores a kind it does not know', () => {
    expect(kindFilterRows([{ branchKind: 'nightly' }], ['nightly'])).toEqual([])
  })

  // #624: Bot and Docs lanes fold to a line a day unless they're shown in full.
  it('says which kinds can fold, and which do', () => {
    const refs = [{ branchKind: 'trunk' }, { branchKind: 'docs' }, { branchKind: 'bot' }]
    const rows = kindFilterRows(refs, [], ['bot'], ['bot', 'docs'])
    expect(rows.map((r) => [r.kind, r.foldable, r.folded])).toEqual([
      ['trunk', false, false],
      ['docs', true, true],
      ['bot', true, false],
    ])
  })

  it('folds nothing when told of no kinds that can', () => {
    expect(kindFilterRows([{ branchKind: 'docs' }], []).every((r) => !r.foldable && !r.folded)).toBe(true)
  })
})

describe('foldingKinds', () => {
  it('is the kinds that can fold, less those shown in full', () => {
    expect([...foldingKinds(['bot', 'docs'], [])]).toEqual(['bot', 'docs'])
    expect([...foldingKinds(['bot', 'docs'], ['docs'])]).toEqual(['bot'])
    expect(foldingKinds([], ['docs']).size).toBe(0)
  })
})

describe('kindWarning', () => {
  it('names the kind for kinds that ask', () => {
    expect(kindWarning('main', 'trunk', CONFIRM_FORCE_PUSH)).toBe('main is a Trunk branch.')
    expect(kindWarning('release/1.4', 'release', CONFIRM_FORCE_PUSH)).toBe('release/1.4 is a Release branch.')
    expect(kindWarning('production', 'environment', CONFIRM_DELETE)).toBe('production is an Environment branch.')
  })

  it('stays quiet for kinds that do not ask, and ordinary branches', () => {
    expect(kindWarning('release/1.4', 'release', CONFIRM_DELETE)).toBe('')
    expect(kindWarning('gh-pages', 'docs', CONFIRM_FORCE_PUSH)).toBe('')
    expect(kindWarning('dependabot/x', 'bot', CONFIRM_DELETE)).toBe('')
    expect(kindWarning('feature/x', '', CONFIRM_FORCE_PUSH)).toBe('')
    expect(kindWarning('feature/x', undefined, CONFIRM_DELETE)).toBe('')
  })

  it('knows every kind', () => {
    for (const k of ['trunk', 'release', 'environment', 'docs', 'bot']) expect(kindInfo(k)).toBeDefined()
    expect(kindInfo('nightly')).toBeUndefined()
  })
})
