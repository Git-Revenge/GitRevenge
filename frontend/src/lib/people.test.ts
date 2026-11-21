import { describe, expect, it } from 'vitest'
import { shownAs, writtenAs } from './people'

const written = { name: 'sarah lind', email: 'slind@users.noreply.example' }
const mapped = { name: 'Sarah Lind', email: 'sarah@tidewater.example' }

describe('shownAs', () => {
  it('shows the mailmap reading when there is one', () => {
    expect(shownAs(mapped, written)).toEqual(mapped)
  })

  it('shows what the commit says when the mailmap was not asked', () => {
    expect(shownAs(undefined, written)).toEqual(written)
    expect(shownAs({ name: '', email: '' }, written)).toEqual(written)
  })
})

describe('writtenAs', () => {
  it('is null when the mailmap changes nothing', () => {
    expect(writtenAs(written, written)).toBeNull()
    expect(writtenAs(undefined, written)).toBeNull()
  })

  it('says what the commit has when it is shown as someone else', () => {
    expect(writtenAs(mapped, written)).toBe('sarah lind <slind@users.noreply.example>')
  })

  it('counts a changed address alone', () => {
    expect(writtenAs({ name: 'sarah lind', email: 'sarah@tidewater.example' }, written)).toBe('sarah lind <slind@users.noreply.example>')
  })

  it('leaves out the brackets when the commit has no address', () => {
    expect(writtenAs(mapped, { name: 'sarah lind', email: '' })).toBe('sarah lind')
  })
})
