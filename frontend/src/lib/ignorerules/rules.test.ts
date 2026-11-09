import { describe as group, expect, it } from 'vitest'
import { appendLine, deleteLine, describe, lineKind, moveLine, parseLines, setLine, splitLines } from './rules'

group('lines', () => {
  it('splits without a phantom last line', () => {
    expect(splitLines('')).toEqual([])
    expect(splitLines('a\nb\n')).toEqual(['a', 'b'])
    expect(splitLines('a\r\nb')).toEqual(['a', 'b'])
    expect(splitLines('\n')).toEqual([''])
  })

  it('tells rules from comments and blanks', () => {
    expect(lineKind('')).toBe('blank')
    expect(lineKind('   ')).toBe('blank')
    expect(lineKind('# build')).toBe('comment')
    expect(lineKind('\\#notes')).toBe('rule')
    expect(lineKind('!keep.log')).toBe('negation')
    expect(parseLines('# a\n*.log\n')).toEqual([
      { n: 1, text: '# a', kind: 'comment' },
      { n: 2, text: '*.log', kind: 'rule' },
    ])
  })

  it('edits, moves, deletes and appends lines', () => {
    const t = 'a\nb\nc\n'
    expect(setLine(t, 2, 'B\nignored')).toBe('a\nB\nc\n')
    expect(moveLine(t, 1, 1)).toBe('b\na\nc\n')
    expect(moveLine(t, 1, -1)).toBe(t)
    expect(moveLine(t, 3, 1)).toBe(t)
    expect(deleteLine(t, 2)).toBe('a\nc\n')
    expect(deleteLine('a\n', 1)).toBe('')
    expect(appendLine('', '*.log')).toBe('*.log\n')
    expect(appendLine(t, 'd')).toBe('a\nb\nc\nd\n')
  })
})

group('describe', () => {
  const rule = { n: 1, text: '*.log', kind: 'rule' as const }
  const stat = (o: Partial<{ ignores: number; restores: number; tracked: number }>) => ({ source: '.gitignore', line: 1, ignores: 0, restores: 0, tracked: 0, paths: [], ...o })

  it('says what a rule does now', () => {
    expect(describe(rule, undefined, false)).toBe('ignores nothing now')
    expect(describe(rule, stat({ ignores: 1 }), false)).toBe('ignores 1 path')
    expect(describe(rule, stat({ ignores: 2000 }), true)).toBe('ignores 2,000 paths+')
    expect(describe(rule, stat({ ignores: 2, tracked: 1 }), false)).toBe('ignores 2 paths · 1 tracked file still tracked')
    expect(describe({ n: 2, text: '!keep', kind: 'negation' }, stat({ restores: 1 }), false)).toBe('brings back 1 file')
    expect(describe({ n: 3, text: '# x', kind: 'comment' }, undefined, false)).toBe('')
  })
})
