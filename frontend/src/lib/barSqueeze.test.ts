import { describe, expect, it } from 'vitest'
import { ALONE_STEPS, searchPlaceholder, SHARED_STEPS, SQUEEZE, squeezeFor } from './barSqueeze'

describe('squeezeFor', () => {
  it('takes no steps in a wide bar, or one not measured yet', () => {
    expect(squeezeFor(1600, SHARED_STEPS)).toBe(0)
    expect(squeezeFor(0, SHARED_STEPS)).toBe(0)
  })

  it('takes the steps in order as the bar narrows', () => {
    const [count, search, undo, network, density] = SHARED_STEPS
    expect(squeezeFor(count, SHARED_STEPS)).toBe(0)
    expect(squeezeFor(count - 1, SHARED_STEPS)).toBe(SQUEEZE.count)
    expect(squeezeFor(search - 1, SHARED_STEPS)).toBe(SQUEEZE.search)
    expect(squeezeFor(undo - 1, SHARED_STEPS)).toBe(SQUEEZE.undo)
    expect(squeezeFor(network - 1, SHARED_STEPS)).toBe(SQUEEZE.network)
    expect(squeezeFor(density - 1, SHARED_STEPS)).toBe(SQUEEZE.density)
  })

  it('lists the steps widest first, so each keeps the ones before it', () => {
    for (const steps of [SHARED_STEPS, ALONE_STEPS]) expect([...steps].sort((a, b) => b - a)).toEqual(steps)
  })

  it("only squeezes the count and the search in the graph's own bar", () => {
    expect(squeezeFor(1, ALONE_STEPS)).toBe(SQUEEZE.search)
  })
})

describe('searchPlaceholder', () => {
  it('says what the field searches while the count is in the bar', () => {
    expect(searchPlaceholder(0, 411, false)).toBe('Search message, author or SHA')
  })

  it('carries the count once it has left the bar', () => {
    expect(searchPlaceholder(SQUEEZE.count, 411, false)).toBe('Search 411 commits')
    expect(searchPlaceholder(SQUEEZE.search, 1, false)).toBe('Search 1 commit')
    expect(searchPlaceholder(SQUEEZE.count, 12345, false)).toBe(`Search ${(12345).toLocaleString()} commits`)
  })

  it('says there are more while the history is still loading', () => {
    expect(searchPlaceholder(SQUEEZE.count, 2000, true)).toBe(`Search ${(2000).toLocaleString()}+ commits`)
    expect(searchPlaceholder(SQUEEZE.count, 1, true)).toBe('Search 1+ commits')
  })
})
