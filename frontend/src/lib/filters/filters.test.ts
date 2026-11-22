import { describe, expect, it } from 'vitest'
import { chipText, commitsText, hasPick, heldText, heldWhy, isOpen, keptText, kindWord, loadOnly, moreText, queryFor, saveOnly, soFar, togglePick, valueName } from './filters'

const key = (label: string, kind = 'person') => ({ id: label.toLowerCase(), label, kind })
const value = (label: string, kind = 'person') => ({ label, kind })

describe('kindWord', () => {
  it('names what a key holds', () => {
    expect(['person', 'ticket', 'commit', 'label', 'identifier', 'text'].map(kindWord)).toEqual(['people', 'tickets', 'commits', 'labels', 'identifiers', 'text'])
    expect(kindWord('something new')).toBe('text')
  })
})

describe('heldWhy', () => {
  it('says why a key is held back', () => {
    expect(heldWhy('identifier')).toBe('a different value on nearly every commit')
    expect(heldWhy('everywhere')).toBe('on nearly every commit')
    expect(heldWhy('rare')).toBe('on only a commit or two')
    expect(heldWhy('')).toBe('')
  })
})

describe('valueName', () => {
  it('shows a person without their address', () => {
    expect(valueName(value('Sam Ortiz <sam@tidewater.example>'))).toBe('Sam Ortiz')
    expect(valueName(value('stable@tidewater.example'))).toBe('stable@tidewater.example')
    expect(valueName(value('<stable@tidewater.example>'))).toBe('<stable@tidewater.example>')
  })

  it('leaves anything else as written', () => {
    expect(valueName(value('#12', 'ticket'))).toBe('#12')
    expect(valueName(value('a < b', 'text'))).toBe('a < b')
  })
})

describe('queryFor', () => {
  it('writes the search that finds a value, and the one for a whole key', () => {
    expect(queryFor(key('Reviewed-by'), value('Sam Ortiz <sam@tidewater.example>'))).toBe('Reviewed-by: Sam Ortiz')
    expect(queryFor(key('Closes', 'ticket'), value('#12', 'ticket'))).toBe('Closes: #12')
    expect(queryFor(key('Changelog', 'label'))).toBe('Changelog:')
  })

})

describe('picks', () => {
  const fixed = { key: 'changelog', value: 'text:fixed' }
  const sam = { key: 'reviewed-by', value: 'person:Sam Ortiz <sam@tidewater.example>' }

  it('knows a picked value', () => {
    expect(hasPick([fixed], 'changelog', 'text:fixed')).toBe(true)
    expect(hasPick([fixed], 'changelog', 'text:added')).toBe(false)
    expect(hasPick([], 'changelog', 'text:fixed')).toBe(false)
  })

  it('adds a value that is not picked, and takes out one that is', () => {
    expect(togglePick([fixed], sam.key, sam.value)).toEqual([fixed, sam])
    expect(togglePick([fixed, sam], fixed.key, fixed.value)).toEqual([sam])
    expect(togglePick([fixed], fixed.key, fixed.value)).toEqual([])
  })

  it('sends only what the backend asks for, whatever came with the picks', () => {
    const shown = [{ ...fixed, keyLabel: 'Changelog', valueLabel: 'fixed', kind: 'label' }]
    expect(togglePick(shown, sam.key, sam.value)).toEqual([fixed, sam])
  })

  it('shows a person by name on a chip, and anything else as it is', () => {
    expect(chipText({ valueLabel: 'Sam Ortiz <sam@tidewater.example>', kind: 'person' })).toBe('Sam Ortiz')
    expect(chipText({ valueLabel: '#12', kind: 'ticket' })).toBe('#12')
  })
})

describe('keptText', () => {
  it('says how many of how many, or how many while the total is unknown', () => {
    expect(keptText(11, 212)).toBe('11 of 212 commits')
    expect(keptText(1, 0)).toBe('1 commit')
    expect(keptText(0, 212)).toBe('0 of 212 commits')
  })
})

describe('the mode a repository last used', () => {
  const store = () => {
    const kept: Record<string, string> = {}
    return { getItem: (k: string) => kept[k] ?? null, setItem: (k: string, v: string) => void (kept[k] = v) }
  }

  it('is Only these until Highlight is chosen, for each repository apart', () => {
    const s = store()
    expect(loadOnly('/r', s)).toBe(true)
    saveOnly('/r', false, s)
    expect(loadOnly('/r', s)).toBe(false)
    expect(loadOnly('/other', s)).toBe(true)
    saveOnly('/r', true, s)
    expect(loadOnly('/r', s)).toBe(true)
  })

  it('is Only these when the store cannot be read', () => {
    const blocked = {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('blocked')
      },
    }
    expect(loadOnly('/r', blocked)).toBe(true)
    expect(() => saveOnly('/r', false, blocked)).not.toThrow()
  })
})

describe('counts', () => {
  it('counts more, commits and held keys', () => {
    expect(moreText(24)).toBe('24 more…')
    expect(commitsText(1)).toBe('1 commit')
    expect(commitsText(3)).toBe('3 commits')
    expect(heldText(1)).toBe('1 key held back')
    expect(heldText(4)).toBe('4 keys held back')
  })
})

describe('soFar', () => {
  it('says nothing once the history is read', () => {
    expect(soFar({ done: true, reading: false, read: 0, total: 0 })).toBe('')
  })

  it('says how far a read has got, or that one is on its way', () => {
    expect(soFar({ done: false, reading: true, read: 412000, total: 1503134 })).toBe(`Counts so far: ${(412000).toLocaleString()} of ${(1503134).toLocaleString()} commits read.`)
    expect(soFar({ done: false, reading: true, read: 20000, total: 0 })).toBe('Counts so far: the history is still being read.')
    expect(soFar({ done: false, reading: false, read: 0, total: 0 })).toBe('Counts so far: the history is still being read.')
  })
})

describe('isOpen', () => {
  it('opens the first few keys, and keeps a choice made since', () => {
    expect(isOpen({}, 'reviewed-by', 0)).toBe(true)
    expect(isOpen({}, 'refs', 3)).toBe(true)
    expect(isOpen({}, 'note', 4)).toBe(false)
    expect(isOpen({ 'filter:reviewed-by': false }, 'reviewed-by', 0)).toBe(false)
    expect(isOpen({ 'filter:note': true }, 'note', 9)).toBe(true)
  })
})
