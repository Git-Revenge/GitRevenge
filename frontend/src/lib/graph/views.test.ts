import { describe, expect, it } from 'vitest'
import { currentView, isRows, menuViews, savedFor, VIEWS, type ViewInfo } from './views'

const ALL: ViewInfo[] = [...VIEWS.filter((v) => !v.experimental), { id: 'first', label: 'First', icon: 'rows', title: '', experimental: true }, { id: 'second', label: 'Second', icon: 'rows', title: '', experimental: true }]

describe('menuViews', () => {
  it('lists only the rows until an experimental view is switched on', () => {
    expect(menuViews([], ALL).map((v) => v.id)).toEqual(['timeline', 'compact'])
  })

  it('adds the experimental views switched on, in the order the app lists them', () => {
    expect(menuViews(['second', 'first'], ALL).map((v) => v.id)).toEqual(['timeline', 'compact', 'first', 'second'])
    expect(menuViews(['second'], ALL).map((v) => v.id)).toEqual(['timeline', 'compact', 'second'])
  })
})

describe('currentView', () => {
  it('is the rows at the saved density when nothing else is saved', () => {
    expect(currentView('comfortable', '', [], ALL)).toBe('timeline')
    expect(currentView('compact', '', [], ALL)).toBe('compact')
  })

  it('is the saved experimental view while that view is switched on', () => {
    expect(currentView('compact', 'first', ['first'], ALL)).toBe('first')
  })

  it('falls back to the rows when the saved view is switched off or unknown', () => {
    expect(currentView('compact', 'first', ['second'], ALL)).toBe('compact')
    expect(currentView('comfortable', 'later', ['later'], ALL)).toBe('timeline')
    // The rows aren't experimental views, whatever a settings file says.
    expect(currentView('comfortable', 'compact', ['compact'], ALL)).toBe('timeline')
  })
})

describe('savedFor', () => {
  it('saves the rows as no view and a density', () => {
    expect(savedFor('timeline')).toEqual({ view: '', density: 'comfortable' })
    expect(savedFor('compact')).toEqual({ view: '', density: 'compact' })
  })

  it('leaves the density alone for an experimental view', () => {
    expect(savedFor('first')).toEqual({ view: 'first' })
  })
})

describe('isRows', () => {
  it('is true for the two densities only', () => {
    expect(isRows('timeline')).toBe(true)
    expect(isRows('compact')).toBe(true)
    expect(isRows('first')).toBe(false)
  })
})
