import { describe, expect, it } from 'vitest'
import { actionForKey, autosquashOf, changed, foldable, foldIn, foldTarget, move, newPlan, outcome, problem, setAction, toRun } from './rebasePlan'

const commit = (id: string, subject: string) => ({ commit: id.repeat(40), subject, message: subject })
const plan = () =>
  newPlan([
    commit('a', 'Cache the last reading'),
    commit('b', 'wip'),
    commit('c', 'fixup! Cache the last reading'),
    commit('d', 'Fall back to the cached reading'),
    commit('e', 'debugging'),
  ])
const subjects = (items: { subject: string }[]) => items.map((i) => i.subject)

describe('setAction', () => {
  it('starts a squash from both messages and puts a reword back to its own', () => {
    let items = setAction(plan(), 1, 'squash')
    expect(items[1].message).toBe('Cache the last reading\n\nwip')
    items = setAction(items, 1, 'reword')
    expect(items[1].message).toBe('wip')
  })

  it('leaves the list it was given alone', () => {
    const before = plan()
    setAction(before, 0, 'drop')
    expect(before[0].action).toBe('pick')
  })
})

describe('foldTarget', () => {
  it('is the nearest kept commit above, past drops and other folds', () => {
    let items = setAction(plan(), 1, 'drop')
    items = setAction(items, 2, 'fixup')
    items = setAction(items, 3, 'squash')
    expect(foldTarget(items, 2)).toBe(0)
    expect(foldTarget(items, 3)).toBe(0)
    expect(foldTarget(items, 0)).toBe(-1)
  })
})

describe('move', () => {
  it('moves a commit and keeps the rest in order', () => {
    expect(subjects(move(plan(), 2, 1))).toEqual(['Cache the last reading', 'fixup! Cache the last reading', 'wip', 'Fall back to the cached reading', 'debugging'])
    expect(subjects(move(plan(), 0, 4)).at(-1)).toBe('Cache the last reading')
    const same = plan()
    expect(move(same, 0, -1)).toBe(same)
  })
})

describe('autosquash', () => {
  it('reads fixup!, squash! and amend! subjects, through repeats', () => {
    expect(autosquashOf('fixup! Cache readings')).toEqual({ action: 'fixup', target: 'Cache readings' })
    expect(autosquashOf('squash! Cache readings')).toEqual({ action: 'squash', target: 'Cache readings' })
    expect(autosquashOf('fixup! fixup! Cache readings')).toEqual({ action: 'fixup', target: 'Cache readings' })
    expect(autosquashOf('Cache readings')).toBeNull()
  })

  it('counts the commits that could be folded in, and folds them under what they name', () => {
    expect(foldable(plan())).toBe(1)
    const items = foldIn(plan())
    expect(subjects(items)).toEqual(['Cache the last reading', 'fixup! Cache the last reading', 'wip', 'Fall back to the cached reading', 'debugging'])
    expect(items[1].action).toBe('fixup')
    expect(foldable(items)).toBe(0)
  })

  it('puts a second fixup after the first, and leaves one with no target alone', () => {
    const items = foldIn(newPlan([commit('a', 'One'), commit('b', 'fixup! One'), commit('c', 'Two'), commit('d', 'squash! One'), commit('e', 'fixup! Gone')]))
    expect(subjects(items)).toEqual(['One', 'fixup! One', 'squash! One', 'Two', 'fixup! Gone'])
    expect(items.map((i) => i.action)).toEqual(['pick', 'fixup', 'squash', 'pick', 'pick'])
  })
})

describe('outcome', () => {
  it('says what the branch will hold afterwards', () => {
    let items = foldIn(plan())
    items = setAction(items, 2, 'reword')
    items[2].message = 'Keep readings for an hour\n\nA longer note.'
    items = setAction(items, 4, 'drop')
    const { commits, dropped } = outcome(items)
    expect(commits).toEqual([
      { commit: 'a'.repeat(40), subject: 'Cache the last reading', made: 2, was: '' },
      { commit: 'b'.repeat(40), subject: 'Keep readings for an hour', made: 1, was: 'wip' },
      { commit: 'd'.repeat(40), subject: 'Fall back to the cached reading', made: 1, was: '' },
    ])
    expect(subjects(dropped)).toEqual(['debugging'])
  })

  it("takes a squash's edited message as the folded commit's", () => {
    const items = setAction(plan(), 1, 'squash')
    items[1].message = 'Cache readings for an hour\n\nBoth, in one.'
    expect(outcome(items).commits[0]).toMatchObject({ subject: 'Cache readings for an hour', made: 2, was: 'Cache the last reading' })
  })
})

describe('problem', () => {
  it('finds what stops a plan from running', () => {
    expect(problem(plan())).toBe('')
    expect(problem(setAction(plan(), 0, 'fixup'))).toContain('nothing above it to fold into')
    expect(problem(plan().map((i) => ({ ...i, action: 'drop' as const })))).toBe('Every commit is dropped. Use Reset instead.')
    const blank = setAction(plan(), 1, 'reword')
    blank[1].message = '  '
    expect(problem(blank)).toBe('wip needs a message.')
  })
})

describe('the rest', () => {
  it('maps letters to actions', () => {
    expect(actionForKey('d')).toBe('drop')
    expect(actionForKey('P')).toBe('pick')
    expect(actionForKey('x')).toBeUndefined()
  })

  it('knows a changed plan from a fresh one', () => {
    const ids = plan().map((i) => i.commit)
    expect(changed(plan(), ids)).toBe(false)
    expect(changed(move(plan(), 0, 1), ids)).toBe(true)
    expect(changed(setAction(plan(), 0, 'reword'), ids)).toBe(true)
  })

  it('hands the backend the plan as it runs it', () => {
    expect(toRun(setAction(plan(), 4, 'drop')).at(-1)).toEqual({ commit: 'e'.repeat(40), subject: 'debugging', message: 'debugging', action: 'drop' })
  })
})
