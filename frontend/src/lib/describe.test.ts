import { describe, expect, it } from 'vitest'
import { afterSaid, firstInSaid } from './describe'

const d = (name: string, after: string, since: number, firstIn = '') => ({ name, after, since, firstIn })

describe('a commit among the tags', () => {
  it('says which tag it comes after, and how far', () => {
    expect(afterSaid(d('v1.1.0-12-gabc1234', 'v1.1.0', 12))).toBe('12 commits after v1.1.0')
    expect(afterSaid(d('v1.1.0-1-gabc1234', 'v1.1.0', 1))).toBe('1 commit after v1.1.0')
    expect(afterSaid(d('v5.2-7360-gf632a8170a6b', 'v5.2', 7360))).toBe('7,360 commits after v5.2')
  })

  it('says when the commit is the tag’s own', () => {
    expect(afterSaid(d('v1.1.0', 'v1.1.0', 0))).toBe('It’s the commit v1.1.0 tags.')
  })

  it('says when no tag comes before it, or none has it yet', () => {
    expect(afterSaid(d('', '', 0))).toBe('No tag comes before it.')
    expect(firstInSaid(d('v1.1.0-1-gabc', 'v1.1.0', 1))).toBe('No tag has it yet.')
    expect(firstInSaid(d('v1.1.0-1-gabc', 'v1.1.0', 1, 'v1.2.0'))).toBe('')
  })
})
