import { describe, expect, it } from 'vitest'
import { alsoWritten, filterKeys, groupQuestions, keysText, kindRule, openText, peopleText, placeLabel, sameText, sortKeys, whyText, type CareKey, type Question } from './care'

const slip = (other: string, key: string, otherCommits = 2, commits = 58): Question => ({ kind: 'spelling', key: key.toLowerCase(), keyLabel: key, commits, other: other.toLowerCase(), otherLabel: other, otherCommits, shared: 0, examples: [] })
const same = (other: string, key: string, shared: number, examples: string[]): Question => ({ kind: 'same', key: key.toLowerCase(), keyLabel: key, commits: 24, other: other.toLowerCase(), otherLabel: other, otherCommits: 12, shared, examples })
const key = (label: string, kind: string, commits: number, values: number, over: Partial<CareKey> = {}, held = ''): CareKey =>
  ({
    key: { id: label.toLowerCase(), label, spellings: [{ text: label, commits }], kind, commits, lines: commits, values, held, known: false },
    where: held ? 'held' : 'panel',
    kindSaid: false,
    whereSaid: false,
    also: [],
    scope: '',
    ...over,
  }) as CareKey

describe('groupQuestions', () => {
  it('asks about the slips of one key together, where its first slip stood', () => {
    const { slips, same: both } = groupQuestions([slip('Signed-of-by', 'Signed-off-by', 150), slip('Reviwed-by', 'Reviewed-by', 80), slip('Singed-off-by', 'Signed-off-by', 64), same('Fixes', 'Closes', 0, [])])
    expect(slips.map((g) => `${g.keyLabel}: ${g.slips.map((s) => s.otherLabel).join(', ')}`)).toEqual(['Signed-off-by: Signed-of-by, Singed-off-by', 'Reviewed-by: Reviwed-by'])
    expect(slips[0].commits).toBe(58)
    expect(both).toHaveLength(1)
  })

  it('has nothing for nothing', () => {
    expect(groupQuestions([])).toEqual({ slips: [], same: [] })
  })
})

describe('what a question says', () => {
  it('says what two keys share, and how many more', () => {
    expect(sameText(same('Refs', 'Ticket', 6, ['#0', '#1', '#2']))).toBe('hold the same things: #0, #1, #2 and 3 more.')
    expect(sameText(same('Refs', 'Ticket', 2, ['#0', '#1']))).toBe('hold the same things: #0, #1.')
  })

  it('says why the ticket verbs are offered though they share nothing', () => {
    expect(sameText(same('Fixes', 'Closes', 0, []))).toBe('both settle tickets, though no ticket is under both.')
  })

  it('counts the open questions, and says when more wait than are listed', () => {
    expect(openText(0, 0)).toBe('Nothing needs care.')
    expect(openText(1, 1)).toBe('1 question')
    expect(openText(12, 12)).toBe('12 questions')
    expect(openText(232, 60)).toBe('232 questions. The first 60 are here; answer some and the next ones show.')
  })

  it('counts the people written two ways', () => {
    expect(peopleText(1)).toBe('1 person is written two ways.')
    expect(peopleText(16143)).toBe(`${(16143).toLocaleString()} people are written two ways.`)
  })
})

describe('a key in the table', () => {
  it('names its kind as a rule would, and its place as the column does', () => {
    expect(kindRule('person')).toBe('people')
    expect(kindRule('ticket')).toBe('tickets')
    expect(kindRule('label')).toBe('labels')
    expect(placeLabel('rows')).toBe('On rows')
    expect(placeLabel('held')).toBe('Held back')
    expect(placeLabel('')).toBe('Panel')
  })

  it('says a guess is a guess, with its reason', () => {
    expect(whyText(key('Note', 'text', 1, 1, {}, 'rare'))).toBe('A guess: it’s on only a commit or two.')
    expect(whyText(key('Change-Id', 'identifier', 400, 400, {}, 'identifier'))).toBe('A guess: it’s a different value on nearly every commit.')
    expect(whyText(key('Changelog', 'label', 21, 3))).toBe('A guess: its values are labels, on 21 commits.')
  })

  it('says whose rule it is when a rule decides', () => {
    expect(whyText(key('Note', 'text', 1, 1, { whereSaid: true, scope: 'local' }))).toBe('This repository’s git config says so.')
    expect(whyText(key('Note', 'text', 1, 1, { whereSaid: true, scope: 'global', where: 'held' }, 'rule'))).toBe('Your global git config says so.')
  })

  it('lists the other ways it is written', () => {
    const k = key('Closes', 'ticket', 30, 30)
    k.key.spellings = [
      { text: 'Closes', commits: 20 },
      { text: 'closed', commits: 6 },
      { text: 'Close', commits: 4 },
    ] as never
    expect(alsoWritten(k)).toEqual(['closed', 'Close'])
    expect(alsoWritten(key('Changelog', 'label', 21, 3))).toEqual([])
  })
})

describe('sortKeys and filterKeys', () => {
  const keys = [key('Signed-off-by', 'person', 900, 40), key('reviewed-by', 'person', 300, 25, { where: 'rows' }), key('Closes', 'ticket', 120, 110), key('Change-Id', 'identifier', 80, 80, {}, 'identifier'), key('Changelog', 'label', 80, 3)]
  const names = (ks: CareKey[]) => ks.map((x) => x.key.label)

  it('sorts numbers from the most and words from A, whatever the case', () => {
    expect(names(sortKeys(keys, 'commits'))).toEqual(['Signed-off-by', 'reviewed-by', 'Closes', 'Change-Id', 'Changelog'])
    expect(names(sortKeys(keys, 'values'))).toEqual(['Closes', 'Change-Id', 'Signed-off-by', 'reviewed-by', 'Changelog'])
    expect(names(sortKeys(keys, 'key'))).toEqual(['Change-Id', 'Changelog', 'Closes', 'reviewed-by', 'Signed-off-by'])
  })

  it('keeps keys that tie in the order they came', () => {
    expect(names(sortKeys(keys, 'kind'))).toEqual(['Change-Id', 'Changelog', 'Signed-off-by', 'reviewed-by', 'Closes'])
    expect(names(sortKeys(keys, 'where'))).toEqual(['reviewed-by', 'Signed-off-by', 'Closes', 'Changelog', 'Change-Id'])
  })

  it('turns round when asked, and leaves what it was given alone', () => {
    expect(names(sortKeys(keys, 'commits', true))).toEqual(['Changelog', 'Change-Id', 'Closes', 'reviewed-by', 'Signed-off-by'])
    expect(names(keys)[0]).toBe('Signed-off-by')
  })

  it('finds a key by its name, a way it is written, its kind or its place', () => {
    expect(names(filterKeys(keys, 'BY'))).toEqual(['Signed-off-by', 'reviewed-by'])
    expect(names(filterKeys(keys, 'tickets'))).toEqual(['Closes'])
    expect(names(filterKeys(keys, 'held'))).toEqual(['Change-Id'])
    expect(names(filterKeys(keys, 'on rows'))).toEqual(['reviewed-by'])
    expect(filterKeys(keys, '  ')).toBe(keys)
    expect(filterKeys(keys, 'zzz')).toEqual([])
    const joined = key('Reviewed-by', 'person', 13, 1, { also: ['Rviewed-by'] })
    expect(filterKeys([joined], 'rview')).toEqual([joined])
  })

  it('counts the keys, and how many a filter keeps', () => {
    expect(keysText(12, 12)).toBe('12 keys')
    expect(keysText(1, 1)).toBe('1 key')
    expect(keysText(3, 2168)).toBe(`3 of ${(2168).toLocaleString()} keys`)
  })
})
