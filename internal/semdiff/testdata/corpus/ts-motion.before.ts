import { afterEach, describe, expect, it, vi } from 'vitest'
import { DUR, STAGGER_MAX, STAGGER_STEP, cubicBezier, easeExit, easeInk, fadeUp, inkIn, inkOut, rowLand, stagger } from './motion'

const node = {} as Element

function reduce(on: boolean) {
  vi.stubGlobal('window', { matchMedia: () => ({ matches: on }) })
}

afterEach(() => vi.unstubAllGlobals())

describe('cubicBezier', () => {
  it('pins the ends', () => {
    for (const ease of [easeInk, easeExit]) {
      expect(ease(0)).toBe(0)
      expect(ease(1)).toBe(1)
      expect(ease(-1)).toBe(0)
      expect(ease(2)).toBe(1)
    }
  })

  it('is linear for the linear curve', () => {
    const linear = cubicBezier(0.25, 0.25, 0.75, 0.75)
    for (const t of [0.1, 0.33, 0.5, 0.9]) expect(linear(t)).toBeCloseTo(t, 5)
  })

  it('never runs backwards', () => {
    for (const ease of [easeInk, easeExit]) {
      let last = 0
      for (let i = 1; i <= 100; i++) {
        const v = ease(i / 100)
        expect(v).toBeGreaterThanOrEqual(last - 1e-9)
        last = v
      }
    }
  })

  it('front-loads --ease-ink and keeps --ease-exit symmetric', () => {
    expect(easeInk(0.25)).toBeGreaterThan(0.5)
    expect(easeExit(0.5)).toBeCloseTo(0.5, 5)
  })
})

describe('stagger', () => {
  it('steps 40ms and stops after six rows', () => {
    expect(stagger(0)).toBe(0)
    expect(stagger(1)).toBe(STAGGER_STEP)
    expect(stagger(STAGGER_MAX - 1)).toBe((STAGGER_MAX - 1) * STAGGER_STEP)
    expect(stagger(50)).toBe((STAGGER_MAX - 1) * STAGGER_STEP)
    expect(stagger(-3)).toBe(0)
  })
})

describe('transitions', () => {
  it('use the tokens', () => {
    reduce(false)
    expect(inkIn(node).duration).toBe(DUR.enter)
    expect(inkOut().duration).toBe(DUR.base)
    expect(fadeUp(node, { delay: 80 })).toMatchObject({ duration: DUR.move, delay: 80 })
  })

  it('collapse to nothing under reduced motion', () => {
    reduce(true)
    for (const t of [inkIn(node, { delay: 120 }), inkOut(), fadeUp(node, { delay: 80 }), rowLand()]) {
      expect(t.duration).toBe(0)
      expect(t.delay ?? 0).toBe(0)
    }
  })

  it('finish settled', () => {
    reduce(false)
    expect(inkIn(node).css?.(1, 0)).toBe('opacity: 1; filter: blur(0px); transform: translateY(0px)')
    expect(rowLand().css?.(1, 0)).toContain('var(--selected) 0%')
  })
})
