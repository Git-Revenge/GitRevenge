import { describe, expect, it } from 'vitest'
import type { app } from '../../../wailsjs/go/models'
import { asSuggested, identitiesOf, identityText, joinTitle, planAction, planHeading, suggestionLabel } from './suggest'

const id = (name: string, email: string) => ({ name, email })

describe('suggestionLabel', () => {
  it('says what the people share and what differs', () => {
    expect(suggestionLabel({ kind: 'name' })).toBe('Same name, another address')
    expect(suggestionLabel({ kind: 'address' })).toBe('Same address, another name')
  })
})

describe('identitiesOf', () => {
  it('gathers every identity once, in order', () => {
    const people = [{ identities: [id('Sarah Lind', 'sarah@x'), id('S. Lind', 'sarah@x')] }, { identities: [id('sarah lind', 'slind@y'), id('Sarah Lind', 'sarah@x')] }]
    expect(identitiesOf(people as never)).toEqual([id('Sarah Lind', 'sarah@x'), id('S. Lind', 'sarah@x'), id('sarah lind', 'slind@y')])
  })

  it('copes with someone who has none', () => {
    expect(identitiesOf([{ identities: undefined }] as never)).toEqual([])
  })
})

describe('asSuggested', () => {
  it('keeps who they are and drops what the dialog has no use for', () => {
    const p = { name: 'Sam Ortiz', email: 'sam@x', commits: 3, coAuthored: 1, bot: false, identities: [{ name: 'sam', email: 'sam@x', commits: 1, coAuthored: 0, lines: [{ source: '.mailmap', line: 2 }] }] } as app.Person
    expect(asSuggested(p)).toEqual({ name: 'Sam Ortiz', email: 'sam@x', commits: 3, coAuthored: 1, bot: false, identities: [id('sam', 'sam@x')] })
  })
})

describe('the dialog words', () => {
  it('counts who is being joined', () => {
    expect(joinTitle(2)).toBe('Two identities, one person')
    expect(joinTitle(3)).toBe('3 identities, one person')
  })

  it('counts the lines a plan adds', () => {
    expect(planHeading({ lines: [] })).toBe('Nothing to add')
    expect(planHeading({ lines: ['a'] })).toBe('This adds one line')
    expect(planHeading({ lines: ['a', 'b'] })).toBe('This adds 2 lines')
    expect(planAction(null)).toBe('Add the line')
    expect(planAction({ lines: ['a'] })).toBe('Add the line')
    expect(planAction({ lines: ['a', 'b'] })).toBe('Add the lines')
  })

  it('writes an identity as a commit would', () => {
    expect(identityText(id('Sam', 'sam@x'))).toBe('Sam <sam@x>')
    expect(identityText(id('Sam', ''))).toBe('Sam')
    expect(identityText(id('', 'sam@x'))).toBe('<sam@x>')
  })
})
