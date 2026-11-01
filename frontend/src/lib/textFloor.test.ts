import { describe, expect, it } from 'vitest'

// The design system's floor for text is 12px, in stylesheets and on the graph's canvas alike.
const FLOOR = 12

const sources = {
  ...import.meta.glob('./**/*.svelte', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../style.css', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('./graph/draw.ts', { query: '?raw', import: 'default', eager: true }),
} as Record<string, string>

/** Every pixel font size in a source: CSS rules, and the sizes in canvas font strings. */
export function fontSizes(source: string): number[] {
  const css = [...source.matchAll(/font-size:\s*([\d.]+)px/g)].map((m) => Number(m[1]))
  const canvas = [...source.matchAll(/setFont\([^\n]*?\$\{([^}]*)\}px|setFont\(ctx, `(?:\d+ )?([\d.]+)px/g)].flatMap((m) => (m[2] ? [Number(m[2])] : [...m[1].matchAll(/\d+(?:\.\d+)?/g)].map((n) => Number(n[0]))))
  return [...css, ...canvas]
}

describe('fontSizes', () => {
  it('reads CSS and canvas sizes, including ones chosen by a condition', () => {
    expect(fontSizes('font-size: 11px; font-size: 14.5px;')).toEqual([11, 14.5])
    expect(fontSizes('setFont(ctx, `500 9.5px ${MONO}`)')).toEqual([9.5])
    expect(fontSizes('setFont(ctx, `${comfortable ? 14.5 : 13.5}px ${SERIF}`)')).toEqual([14.5, 13.5])
  })
})

describe('text sizes', () => {
  it('finds the sources to check', () => {
    expect(Object.keys(sources).length).toBeGreaterThan(40)
  })

  for (const [file, source] of Object.entries(sources)) {
    it(`${file} stays at or above ${FLOOR}px`, () => {
      expect(fontSizes(source).filter((s) => s < FLOOR)).toEqual([])
    })
  }
})
