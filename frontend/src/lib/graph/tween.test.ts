import { describe, expect, it } from 'vitest'
import { Washes, at, settled } from './tween'

describe('at', () => {
  it('starts where it set off and lands on its target', () => {
    const t = { from: 0, to: 1, start: 1000 }
    expect(at(t, 1000, 200)).toBe(0)
    expect(at(t, 1200, 200)).toBe(1)
    expect(at(t, 5000, 200)).toBe(1)
  })

  it('is its target when there is no time to move in', () => {
    expect(at({ from: 0, to: 1, start: 0 }, 0, 0)).toBe(1)
  })

  it('moves monotonically', () => {
    const t = { from: 0.2, to: 0.9, start: 0 }
    let last = 0.2
    for (let now = 0; now <= 200; now += 10) {
      const v = at(t, now, 200)
      expect(v).toBeGreaterThanOrEqual(last)
      last = v
    }
  })
})

describe('settled', () => {
  it('is settled once the duration has passed, or when there was nowhere to go', () => {
    expect(settled({ from: 0, to: 1, start: 0 }, 100, 200)).toBe(false)
    expect(settled({ from: 0, to: 1, start: 0 }, 200, 200)).toBe(true)
    expect(settled({ from: 1, to: 1, start: 0 }, 0, 200)).toBe(true)
  })
})

describe('Washes', () => {
  it('starts a row at its target the first time it is seen, so rows scrolling in do not fade', () => {
    const w = new Washes(() => 200)
    expect(w.value('s1', 1, 0)).toBe(1)
    expect(w.value('s2', 0, 0)).toBe(0)
    expect(w.busy(0)).toBe(false)
  })

  it('eases toward a new target and reports itself busy until it arrives', () => {
    const w = new Washes(() => 200)
    w.value('s1', 0, 0)
    expect(w.value('s1', 1, 0)).toBe(0)
    expect(w.busy(0)).toBe(true)
    const mid = w.value('s1', 1, 100)
    expect(mid).toBeGreaterThan(0)
    expect(mid).toBeLessThan(1)
    expect(w.value('s1', 1, 200)).toBe(1)
    expect(w.busy(200)).toBe(false)
  })

  it('turns round from where it is when the target changes mid-way', () => {
    const w = new Washes(() => 200)
    w.value('s1', 0, 0)
    w.value('s1', 1, 0)
    const half = w.value('s1', 1, 100)
    expect(w.value('s1', 0, 100)).toBeCloseTo(half, 10)
    expect(w.value('s1', 0, 300)).toBe(0)
  })

  it('is always at its target under reduced motion', () => {
    const w = new Washes(() => 0)
    w.value('s1', 0, 0)
    expect(w.value('s1', 1, 0)).toBe(1)
    expect(w.busy(0)).toBe(false)
  })

  it('forgets rows that are pruned', () => {
    const w = new Washes(() => 200)
    w.value('s1', 1, 0)
    w.value('s2', 1, 0)
    w.prune((k) => k === 's2')
    expect(w.size).toBe(1)
    // Seen afresh, it starts at its target again.
    expect(w.value('s1', 0, 0)).toBe(0)
  })
})
