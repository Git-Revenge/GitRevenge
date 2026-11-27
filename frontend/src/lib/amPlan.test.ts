import { describe, expect, it } from 'vitest'
import { branchNameOf, fileSummary, seriesOf, titleOf } from './amPlan'

const msg = (subject: string, number = 0, total = 0, version = 1, kind: 'patch' | 'cover' | 'reply' = 'patch') => ({
  subject,
  number,
  total,
  version,
  cover: kind === 'cover',
  hasPatch: kind === 'patch',
})

describe('a file of patches as a series', () => {
  // A thread saved from a list archive: v1, then v2 with a cover letter, out of order, and a
  // reply.
  const thread = [
    msg('Two', 1, 2),
    msg('Three', 2, 2),
    msg('Wind speeds in knots', 0, 3, 2, 'cover'),
    msg('Show wind', 2, 3, 2),
    msg('Time out', 1, 3, 2),
    msg('Looks good', 1, 3, 2, 'reply'),
    msg('Limit', 3, 3, 2),
  ]

  it('sorts each version into [n/m] order, its cover letter beside it, and counts the replies', () => {
    const s = seriesOf(thread)
    expect(s.versions).toEqual([
      { version: 1, cover: -1, patches: [0, 1] },
      { version: 2, cover: 2, patches: [4, 3, 6] },
    ])
    expect(s.replies).toBe(1)
  })

  it('keeps the file’s order when the patches aren’t all numbered', () => {
    const s = seriesOf([msg('b', 2, 2), msg('a'), msg('c', 1, 2)])
    expect(s.versions[0].patches).toEqual([0, 1, 2])
  })

  it('is called after its cover letter, unless that still says git’s placeholder', () => {
    const s = seriesOf(thread)
    expect(titleOf(thread, s.versions[1])).toBe('Wind speeds in knots')
    expect(titleOf(thread, s.versions[0])).toBe('Two')
    const placeholder = [msg('*** SUBJECT HERE ***', 0, 1, 1, 'cover'), msg('Fix it', 1, 1)]
    expect(titleOf(placeholder, seriesOf(placeholder).versions[0])).toBe('Fix it')
  })

  it('names a branch after it', () => {
    expect(branchNameOf('Wind speeds in knots!')).toBe('wind-speeds-in-knots')
    expect(branchNameOf('***')).toBe('patches')
    expect(branchNameOf('a'.repeat(60)).length).toBe(48)
  })

  it('says what the file holds', () => {
    const s = seriesOf(thread)
    expect(fileSummary(thread, s, s.versions[1])).toBe('7 messages: a cover letter, 3 patches, 2 messages of other versions and 1 reply left out')
    const one = [msg('Fix it')]
    expect(fileSummary(one, seriesOf(one), seriesOf(one).versions[0])).toBe('1 message: 1 patch')
  })
})
