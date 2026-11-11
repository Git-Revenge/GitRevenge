import { describe, expect, it } from 'vitest'
import { LABEL_COLUMN } from './draw'
import { loadLabelWidth, nextLabelWidth, saveLabelWidth } from './labelColumn'

function store(initial: Record<string, string> = {}) {
  const items = new Map(Object.entries(initial))
  return {
    items,
    getItem: (k: string) => items.get(k) ?? null,
    setItem: (k: string, v: string) => void items.set(k, v),
    removeItem: (k: string) => void items.delete(k),
  }
}

describe('nextLabelWidth', () => {
  it('starts at the default until a label has been measured', () => {
    expect(nextLabelWidth(null, null, 0, true)).toBe(LABEL_COLUMN.default)
    expect(nextLabelWidth(null, null, 190, false)).toBe(190)
  })

  it('fits the labels in view once the graph holds still', () => {
    expect(nextLabelWidth(150, null, 200, true)).toBe(200)
    expect(nextLabelWidth(200, null, 130, true)).toBe(130)
  })

  it('holds its width while the graph is moving', () => {
    expect(nextLabelWidth(150, null, 200, false)).toBe(150)
  })

  it('holds its width when no labels are in view', () => {
    expect(nextLabelWidth(200, null, 0, true)).toBe(200)
  })

  it('stays within its bounds', () => {
    expect(nextLabelWidth(150, null, 40, true)).toBe(LABEL_COLUMN.min)
    expect(nextLabelWidth(150, null, 900, true)).toBe(LABEL_COLUMN.max)
  })

  it('uses a chosen width whatever the labels need', () => {
    expect(nextLabelWidth(150, 180, 230, true)).toBe(180)
    expect(nextLabelWidth(null, 500, 0, false)).toBe(LABEL_COLUMN.max)
  })
})

describe('the chosen width', () => {
  it('is kept for each repository', () => {
    const s = store()
    saveLabelWidth('/repo/a', 200, s)
    expect(loadLabelWidth('/repo/a', s)).toBe(200)
    expect(loadLabelWidth('/repo/b', s)).toBeNull()
  })

  it('is forgotten when the column goes back to fitting', () => {
    const s = store()
    saveLabelWidth('/repo/a', 200, s)
    saveLabelWidth('/repo/a', null, s)
    expect(loadLabelWidth('/repo/a', s)).toBeNull()
    expect(s.items.size).toBe(0)
  })

  it('ignores anything unreadable, and fits what it reads', () => {
    expect(loadLabelWidth('/r', store({ 'gitrevenge.labelColumn:/r': '{oops' }))).toBeNull()
    expect(loadLabelWidth('/r', store({ 'gitrevenge.labelColumn:/r': '"wide"' }))).toBeNull()
    expect(loadLabelWidth('/r', store({ 'gitrevenge.labelColumn:/r': '9000' }))).toBe(LABEL_COLUMN.max)
  })

  it('carries on when the store refuses', () => {
    const s = store()
    s.setItem = () => {
      throw new Error('full')
    }
    expect(() => saveLabelWidth('/r', 200, s)).not.toThrow()
  })
})
