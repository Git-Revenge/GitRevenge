import { describe, expect, it } from 'vitest'
import { eachPass } from './pass'
import type { Row } from './rowCache'

const row = (pass?: number[], passKind?: string[]): Row => ({ index: 0, col: 0, color: 0, up: [], down: [], width: 0, id: 'a', parents: 0, author: '', authorTime: 0, subject: '', pass, passKind })

const lanes = (r: Row, first = 0, last = Infinity) => {
  const out: [number, number, string | undefined][] = []
  eachPass(r, first, last, (col, color, kind) => out.push([col, color, kind]))
  return out
}

describe('eachPass', () => {
  it('visits nothing in a row with no lanes passing through', () => {
    expect(lanes(row())).toEqual([])
    expect(lanes(row([], []))).toEqual([])
  })

  it('unpacks runs into their lanes, each with its own colour and its run’s kind', () => {
    // Lanes 1-3 of no kind, then 5 of docs, then 6-7 of none again.
    const r = row([1, 3, 10, 11, 12, 5, 1, 20, 6, 2, 30, 31], ['', 'docs', ''])
    expect(lanes(r)).toEqual([
      [1, 10, undefined],
      [2, 11, undefined],
      [3, 12, undefined],
      [5, 20, 'docs'],
      [6, 30, undefined],
      [7, 31, undefined],
    ])
  })

  it('only visits the columns asked for, wherever a run starts or ends', () => {
    const r = row([0, 4, 10, 11, 12, 13, 6, 3, 20, 21, 22, 100, 2, 30, 31], ['', 'bot', ''])
    expect(lanes(r, 2, 7).map((l) => l[0])).toEqual([2, 3, 6, 7])
    expect(lanes(r, 2, 7).map((l) => l[1])).toEqual([12, 13, 20, 21])
    expect(lanes(r, 9, 99)).toEqual([])
    expect(lanes(r, 101, 500)).toEqual([[101, 31, undefined]])
  })

  it('stops reading runs once they’re past the last column', () => {
    const r = row([0, 1, 1, 50, 1, 2, 900, 1, 3], ['', '', ''])
    let visited = 0
    eachPass(r, 0, 10, () => visited++)
    expect(visited).toBe(1)
  })

  it('gets through a run that claims no lanes, or more than it has', () => {
    expect(lanes(row([3, 0, 9, 1, 7], ['', '']))).toEqual([[9, 7, undefined]])
    expect(() => lanes(row([3, 5, 1], ['']))).not.toThrow()
  })
})
