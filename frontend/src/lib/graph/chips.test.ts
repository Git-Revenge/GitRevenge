import { describe, expect, it } from 'vitest'
import { chipId, chipName, chipsWidth, chipValue, CHIPS_SHOWN, fitChips } from './draw'
import type { RowChip } from './rowCache'

const chip = (keyLabel: string, label: string, kind = 'label'): RowChip => ({ key: keyLabel.toLowerCase(), keyLabel, value: `${kind}:${label}`, label, kind })
// Every letter is 6 wide, as mono is, and text is cut to a width by dropping letters.
const measure = (text: string) => text.length * 6
const cut = (text: string, width: number) => (measure(text) <= width ? text : text.slice(0, Math.max(0, Math.floor(width / 6) - 1)) + '…')

describe('a chip', () => {
  it('says a person by name and anything else as it is', () => {
    expect(chipValue(chip('Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person'))).toBe('Sam Ortiz')
    expect(chipValue(chip('Closes', '#14', 'ticket'))).toBe('#14')
    expect(chipValue(chip('Note', 'see <the docs>', 'label'))).toBe('see <the docs>')
  })

  it('is named as the button it is', () => {
    expect(chipName(chip('Changelog', 'fixed'))).toBe('Changelog: fixed, filter')
    expect(chipName(chip('Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person'))).toBe('Reviewed-by: Sam Ortiz, filter')
  })

  it('is told from another by its key and value', () => {
    expect(chipId(chip('Changelog', 'fixed'))).not.toBe(chipId(chip('Changelog', 'added')))
    expect(chipId(chip('Changelog', 'fixed'))).toBe(chipId(chip('Changelog', 'fixed')))
  })
})

describe('fitChips', () => {
  const fixed = chip('Changelog', 'fixed')
  const closes = chip('Closes', '#14', 'ticket')
  const sam = chip('Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person')

  it('shows nothing for a row with no chips', () => {
    expect(fitChips([], 500, measure, cut)).toEqual([])
  })

  it('shows every chip while there are no more than the row shows', () => {
    const fit = fitChips([fixed, closes], 500, measure, cut)
    expect(fit.map((c) => `${c.key} ${c.value}`)).toEqual(['Changelog fixed', 'Closes #14'])
    expect(fit.every((c) => c.chip && !c.more)).toBe(true)
    expect(CHIPS_SHOWN).toBe(2)
  })

  it('shows two and a count of the rest', () => {
    const fit = fitChips([fixed, closes, sam, chip('Closes', '#15', 'ticket')], 500, measure, cut)
    expect(fit.map((c) => c.value)).toEqual(['fixed', '#14', '+2'])
    expect(fit[2]).toMatchObject({ more: 2 })
    expect(fit[2].chip).toBeUndefined()
  })

  it('gives up chips for a count as the room runs short, and shows nothing when even that does not fit', () => {
    // Values too short to be cut: "fixed" and "#14".
    const all = [fixed, closes, sam]
    const wide = fitChips(all, 500, measure, cut)
    expect(wide.map((c) => c.value)).toEqual(['fixed', '#14', '+1'])
    const one = fitChips(all, chipsWidth(wide) - 1, measure, cut)
    expect(one.map((c) => c.value)).toEqual(['fixed', '+2'])
    const none = fitChips(all, chipsWidth(one) - 1, measure, cut)
    expect(none.map((c) => c.value)).toEqual(['+3'])
    expect(fitChips(all, chipsWidth(none) - 1, measure, cut)).toEqual([])
    expect(fitChips(all, 0, measure, cut)).toEqual([])
  })

  it('cuts a long value shorter before a chip gives way to a count', () => {
    // A row's one chip, a few letters too wide for it: a shorter name beats "+1".
    const whole = fitChips([sam], 500, measure, cut)
    expect(whole.map((c) => c.value)).toEqual(['Sam Ortiz'])
    const tight = fitChips([sam], chipsWidth(whole) - 12, measure, cut)
    expect(tight).toHaveLength(1)
    expect(tight[0].chip).toBe(sam)
    expect(tight[0].value.endsWith('…')).toBe(true)
    expect(chipsWidth(tight)).toBeLessThanOrEqual(chipsWidth(whole) - 12)
    // With too little left of the name to read, the count it is.
    expect(fitChips([sam], chipsWidth(whole) - 30, measure, cut).map((c) => c.value)).toEqual(['+1'])
  })

  it('never takes more room than it was given', () => {
    for (const room of [40, 90, 150, 220, 400]) {
      expect(chipsWidth(fitChips([fixed, closes, sam], room, measure, cut))).toBeLessThanOrEqual(room)
    }
  })

  it('cuts a long value short, and says a person by name', () => {
    const long = chip('Link', 'https://example.com/a/very/long/path/that/goes/on/and/on/for/ever', 'label')
    const [fit] = fitChips([long], 1000, measure, cut)
    expect(fit.value.endsWith('…')).toBe(true)
    expect(measure(fit.value)).toBeLessThanOrEqual(150)
    expect(fitChips([sam], 1000, measure, cut)[0].value).toBe('Sam Ortiz')
  })
})
