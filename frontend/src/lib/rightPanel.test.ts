import { describe, expect, it } from 'vitest'
import { DEFAULT_WIDTH, fitWidth, loadRightPanel, maxWidth, MIN_WIDTH, saveRightPanel } from './rightPanel'

const store = (seed: Record<string, string> = {}) => {
  const data = { ...seed }
  return { data, getItem: (k: string) => data[k] ?? null, setItem: (k: string, v: string) => void (data[k] = v) }
}

describe('fitting the panel to the window', () => {
  it('caps it at 60% of a big window', () => {
    expect(maxWidth(2000)).toBe(1200)
  })

  it('leaves the sidebar and graph their room in a smaller one', () => {
    expect(maxWidth(1000)).toBe(440)
  })

  it('never goes below the minimum', () => {
    expect(maxWidth(600)).toBe(MIN_WIDTH)
    expect(fitWidth(100, 2000)).toBe(MIN_WIDTH)
  })

  it('leaves a width that fits alone', () => {
    expect(fitWidth(500, 2000)).toBe(500)
  })

  it('narrows a chosen width to fit, without changing the choice', () => {
    const chosen = 900
    expect(fitWidth(chosen, 1000)).toBe(440)
    expect(fitWidth(chosen, 2000)).toBe(900)
  })
})

describe('remembering the width', () => {
  it('starts at the default', () => {
    expect(loadRightPanel('/r', store()).width).toBe(DEFAULT_WIDTH)
  })

  it('keeps each repository apart', () => {
    const s = store()
    saveRightPanel('/a', { width: 500, hidden: false }, s)
    expect(loadRightPanel('/a', s).width).toBe(500)
    expect(loadRightPanel('/b', s).width).toBe(DEFAULT_WIDTH)
  })

  it('remembers whether it is hidden', () => {
    const s = store()
    expect(loadRightPanel('/a', s).hidden).toBe(false)
    saveRightPanel('/a', { width: 500, hidden: true }, s)
    expect(loadRightPanel('/a', s).hidden).toBe(true)
  })

  it('ignores anything unreadable', () => {
    const s = store({ 'gitrevenge.rightPanel:/r': 'not json' })
    expect(loadRightPanel('/r', s).width).toBe(DEFAULT_WIDTH)
    const odd = store({ 'gitrevenge.rightPanel:/r': '{"width":"wide"}' })
    expect(loadRightPanel('/r', odd).width).toBe(DEFAULT_WIDTH)
  })

  it("doesn't mind a store that refuses", () => {
    const refusing = { getItem: () => null, setItem: () => { throw new Error('full') } }
    expect(() => saveRightPanel('/r', { width: 500, hidden: false }, refusing)).not.toThrow()
  })
})
