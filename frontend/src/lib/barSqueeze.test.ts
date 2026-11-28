import { describe, expect, it } from 'vitest'
import { ALONE_STEPS, FOCUS_ICON_W, focusChipWidth, searchPlaceholder, SHARED_STEPS, SQUEEZE, squeezeFor, squeezeWithChip } from './barSqueeze'

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

  it('says since when in a shallow clone', () => {
    expect(searchPlaceholder(SQUEEZE.count, 411, false, 'Nov 26, 2025')).toBe('Search 411 commits since Nov 26, 2025')
    expect(searchPlaceholder(0, 411, false, 'Nov 26, 2025')).toBe('Search message, author or SHA')
  })

  it('says there are more while the history is still loading', () => {
    expect(searchPlaceholder(SQUEEZE.count, 2000, true)).toBe(`Search ${(2000).toLocaleString()}+ commits`)
    expect(searchPlaceholder(SQUEEZE.count, 1, true)).toBe('Search 1+ commits')
  })
})

describe('squeezeWithChip', () => {
  const chip = focusChipWidth(151) // "Focused on src/alerts/"

  it('is squeezeFor with no focus', () => {
    for (const w of [0, 1236, 1000, 500]) expect(squeezeWithChip(w, SHARED_STEPS, 0)).toBe(squeezeFor(w, SHARED_STEPS))
  })

  it('counts the chip: a bar that holds everything else gives way for it', () => {
    // The bar at 1280 px with both panels hidden: no step without the chip, the count's step with it.
    expect(squeezeFor(1236, SHARED_STEPS)).toBe(0)
    expect(squeezeWithChip(1236, SHARED_STEPS, chip)).toBe(SQUEEZE.count)
    expect(squeezeWithChip(1600, SHARED_STEPS, chip)).toBe(0)
  })

  it('once the search is an icon, counts only the chip’s icon', () => {
    const [, search, undo] = SHARED_STEPS
    // Too narrow for the whole chip beside the search field, wide enough with its icon alone.
    expect(squeezeWithChip(search - 1 + chip, SHARED_STEPS, chip)).toBe(SQUEEZE.search)
    expect(squeezeWithChip(undo - 1 + FOCUS_ICON_W, SHARED_STEPS, chip)).toBe(SQUEEZE.undo)
    expect(squeezeWithChip(undo + FOCUS_ICON_W, SHARED_STEPS, chip)).toBe(SQUEEZE.search)
  })

  it("works in the graph's own bar", () => {
    expect(squeezeWithChip(ALONE_STEPS[0] + chip - 1, ALONE_STEPS, chip)).toBe(SQUEEZE.count)
    expect(squeezeWithChip(100, ALONE_STEPS, chip)).toBe(SQUEEZE.search)
  })

  it('caps a long name as the chip does', () => {
    expect(focusChipWidth(900)).toBe(focusChipWidth(260))
  })
})
