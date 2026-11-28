import { describe, expect, it } from 'vitest'
import { along, authorInks, changedLines, churnText, markHeight, stepLabel, textLines, trackSteps, yearMarks } from './track'

const v = (id: string, author: string, time: number, added = 1, removed = 0, binary = false) => ({ id, author, time, subject: id, added, removed, binary })
const at = (y: number, m: number) => Math.floor(new Date(y, m - 1, 15).getTime() / 1000)

describe('trackSteps', () => {
  it('lists the versions oldest first, and the work tree last when it has changes', () => {
    const versions = [v('a', 'Sam', at(2025, 3)), v('b', 'Ana', at(2026, 1))]
    expect(trackSteps({ versions, workTree: false }).map((s) => s.id)).toEqual(['a', 'b'])
    const steps = trackSteps({ versions, workTree: true })
    expect(steps.map((s) => s.id)).toEqual(['a', 'b', ''])
    expect(steps[2].subject).toBe('Not committed yet')
  })
})

describe('stepLabel', () => {
  const steps = [{ id: 'a' }, { id: 'b' }, { id: 'c' }]
  it('counts the versions, and says which is the newest', () => {
    expect(stepLabel(0, steps)).toBe('Version 1 of 3')
    expect(stepLabel(2, steps)).toBe('Version 3 of 3, the newest')
  })
  it('names the work tree, which isn’t a version', () => {
    const withTree = [...steps, { id: '' }]
    expect(stepLabel(2, withTree)).toBe('Version 3 of 3, the newest')
    expect(stepLabel(3, withTree)).toBe('Not committed yet')
  })
})

describe('marks and years', () => {
  it('makes a mark taller the more lines it changed, without a big change dwarfing the rest', () => {
    expect(markHeight({ added: 0, removed: 0, binary: false })).toBe(6)
    expect(markHeight({ added: 1, removed: 0, binary: false })).toBeLessThan(markHeight({ added: 5, removed: 2, binary: false }))
    expect(markHeight({ added: 10_000, removed: 0, binary: false })).toBe(26)
    expect(markHeight({ added: 0, removed: 0, binary: true })).toBe(6)
  })
  it('puts each year at its first step', () => {
    const steps = [v('a', 'S', at(2025, 3)), v('b', 'S', at(2025, 9)), v('c', 'S', at(2026, 1)), { ...v('', '', 0) }]
    expect(yearMarks(steps)).toEqual([
      { label: '2025', at: 0 },
      { label: '2026', at: 2 / 3 },
    ])
    expect(along(0, 1)).toBe(0)
  })
  it('gives each author a colour in the order they first appear', () => {
    expect([...authorInks([v('a', 'Sam', 1), v('b', 'Ana', 2), v('c', 'Sam', 3)])]).toEqual([
      ['Sam', 0],
      ['Ana', 1],
    ])
  })
})

describe('a step’s text', () => {
  it('says what it changed', () => {
    expect(churnText({ added: 5, removed: 2, binary: false }, false)).toBe('+5 −2')
    expect(churnText({ added: 31, removed: 0, binary: false }, true)).toBe('+31, the first')
    expect(churnText({ added: 0, removed: 0, binary: true }, false)).toBe('binary')
  })
  it('finds the lines a diff changed on its new side', () => {
    expect([...changedLines([{ newStart: 2, newEnd: 4 }, { newStart: 9, newEnd: 9 }])]).toEqual([2, 3])
  })
  it('splits a text into lines, without the empty one at the end', () => {
    expect(textLines('a\r\nb\n')).toEqual(['a', 'b'])
    expect(textLines('')).toEqual([])
  })
})
