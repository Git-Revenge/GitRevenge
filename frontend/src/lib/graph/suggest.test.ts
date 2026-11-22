import { describe, expect, it } from 'vitest'
import { completion, isKey, markParts, personParts, stepRow, suggestionLabel, suggestionName, type Suggestion } from './suggest'

const value = (keyLabel: string, label: string, kind: string, commits = 3): Suggestion => ({ key: keyLabel.toLowerCase(), keyLabel, value: `${kind}:${label}`, label, kind, commits })
const key = (keyLabel: string, commits = 9): Suggestion => ({ key: keyLabel.toLowerCase(), keyLabel, value: '', label: '', kind: 'label', commits })

describe('markParts', () => {
  it('cuts text where it holds what was typed, whatever the case', () => {
    expect(markParts('Sarah Lind', 'sa')).toEqual([
      { text: 'Sa', hit: true },
      { text: 'rah Lind', hit: false },
    ])
    expect(markParts('Reviewed-by', 'VIEW')).toEqual([
      { text: 'Re', hit: false },
      { text: 'view', hit: true },
      { text: 'ed-by', hit: false },
    ])
  })

  it('marks every place it is found', () => {
    expect(markParts('anna', 'a').filter((p) => p.hit)).toHaveLength(2)
    expect(
      markParts('anna', 'a')
        .map((p) => p.text)
        .join(''),
    ).toBe('anna')
  })

  it('leaves text whole when nothing was typed or nothing matches', () => {
    expect(markParts('fixed', '')).toEqual([{ text: 'fixed', hit: false }])
    expect(markParts('fixed', '  ')).toEqual([{ text: 'fixed', hit: false }])
    expect(markParts('fixed', 'zz')).toEqual([{ text: 'fixed', hit: false }])
    expect(markParts('', 'a')).toEqual([])
  })
})

describe('a person', () => {
  it('is a name and an address', () => {
    expect(personParts('Sam Ortiz <sam@tidewater.example>')).toEqual({ name: 'Sam Ortiz', email: 'sam@tidewater.example' })
    expect(personParts('Sam Ortiz')).toEqual({ name: 'Sam Ortiz', email: '' })
  })

  it('shows by name, and anything else as it is', () => {
    expect(suggestionName(value('Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person'))).toBe('Sam Ortiz')
    expect(suggestionName(value('Closes', '#12', 'ticket'))).toBe('#12')
    // A value that only looks like a person is left alone.
    expect(suggestionName(value('Note', 'see <the docs>', 'text'))).toBe('see <the docs>')
  })
})

describe('completion', () => {
  it('writes a key ready for a value, and a value with its key', () => {
    expect(completion(key('Reviewed-by'))).toBe('Reviewed-by: ')
    expect(completion(value('Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person'))).toBe('Reviewed-by: Sam Ortiz')
    expect(completion(value('Changelog', 'fixed', 'label'))).toBe('Changelog: fixed')
  })

  it('knows a key from a value', () => {
    expect(isKey(key('Changelog'))).toBe(true)
    expect(isKey(value('Changelog', 'fixed', 'label'))).toBe(false)
  })
})

describe('suggestionLabel', () => {
  it('reads a value with its key and its count', () => {
    expect(suggestionLabel(value('Reviewed-by', 'Sam Ortiz <sam@tidewater.example>', 'person', 12))).toBe('Reviewed-by: Sam Ortiz, 12 commits')
    expect(suggestionLabel(value('Closes', '#12', 'ticket', 1))).toBe('Closes: #12, 1 commit')
  })

  it('reads a key as a trailer', () => {
    expect(suggestionLabel(key('Changelog', 1204))).toBe(`Changelog, a trailer on ${(1204).toLocaleString()} commits`)
  })
})

describe('stepRow', () => {
  it('moves by a step and stops at either end', () => {
    expect(stepRow(0, 1, 4)).toBe(1)
    expect(stepRow(3, 1, 4)).toBe(3)
    expect(stepRow(0, -1, 4)).toBe(0)
    expect(stepRow(2, -1, 4)).toBe(1)
  })
})
