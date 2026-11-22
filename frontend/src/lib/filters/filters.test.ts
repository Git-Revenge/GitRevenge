import { describe, expect, it } from 'vitest'
import { commitsText, heldText, heldWhy, isOpen, isPicked, kindWord, moreText, queryFor, soFar, valueName } from './filters'

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

describe('queryFor and isPicked', () => {
  it('writes the search that finds a value, and the one for a whole key', () => {
    expect(queryFor(key('Reviewed-by'), value('Sam Ortiz <sam@tidewater.example>'))).toBe('Reviewed-by: Sam Ortiz')
    expect(queryFor(key('Closes', 'ticket'), value('#12', 'ticket'))).toBe('Closes: #12')
    expect(queryFor(key('Changelog', 'label'))).toBe('Changelog:')
  })

  it('knows the search it wrote, whatever the case or the space around it', () => {
    const k = key('Changelog', 'label')
    expect(isPicked('Changelog: fixed', k, value('fixed', 'label'))).toBe(true)
    expect(isPicked('  changelog: FIXED ', k, value('fixed', 'label'))).toBe(true)
    expect(isPicked('Changelog: fix', k, value('fixed', 'label'))).toBe(false)
    expect(isPicked('', k, value('fixed', 'label'))).toBe(false)
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
