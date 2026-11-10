import { EditorState } from '@codemirror/state'
import { describe, expect, it } from 'vitest'
import { nextPlace, parsePlaces, placeLabel, placeLines, sideView, sourceOf, startingResult, textFor, touches, undecided, withChoice, type PlaceState } from './places'
import { currentField, placesField, setCurrent, setPlaces } from './placesField'

const merged = ['one', '<<<<<<< ours', 'two ours', '=======', 'two theirs', '>>>>>>> theirs', 'three', 'four', '<<<<<<< ours', 'five ours', 'more ours', '=======', '>>>>>>> theirs', 'six', ''].join('\n')
const diff3 = ['one', '<<<<<<< ours', 'two ours', '||||||| base', 'two', '=======', 'two theirs', '>>>>>>> theirs', 'three', 'four', '<<<<<<< ours', 'five ours', 'more ours', '||||||| base', 'five', '=======', '>>>>>>> theirs', 'six', ''].join('\n')

describe('parsePlaces and startingResult', () => {
  it('splits a file into shared text and places, and starts with every place empty', () => {
    const parsed = parsePlaces(merged)
    expect(parsed.text).toEqual(['one\n', 'three\nfour\n', 'six\n'])
    expect(parsed.places).toEqual([
      { ours: 'two ours\n', base: null, theirs: 'two theirs\n' },
      { ours: 'five ours\nmore ours\n', base: null, theirs: '' },
    ])
    const start = startingResult(parsed)
    expect(start.doc).toBe('one\nthree\nfour\nsix\n')
    expect(start.places).toEqual([
      { status: 'undecided', from: 4, to: 4 },
      { status: 'undecided', from: 15, to: 15 },
    ])
    expect(start.doc).not.toMatch(/<<<<<<<|>>>>>>>|=======/)
  })

  it('keeps the original of each place when the re-merge has the same places', () => {
    const file = { merged: { text: merged }, diff3 }
    expect(sourceOf(file)).toBe(diff3)
    expect(parsePlaces(sourceOf(file)).places[0].base).toBe('two\n')
    // A re-merge that disagrees with the file on disk isn't trusted.
    expect(sourceOf({ merged: { text: merged }, diff3: diff3.replace('two theirs', 'something else') })).toBe(merged)
    expect(sourceOf({ merged: { text: merged }, diff3: '' })).toBe(merged)
  })

  it('has no places in a file with no markers', () => {
    expect(parsePlaces('plain\n')).toEqual({ text: ['plain\n'], places: [] })
  })
})

describe('choices', () => {
  const parsed = parsePlaces(merged)

  it('gives the text for a choice', () => {
    expect(textFor(parsed.places[0], 'ours')).toBe('two ours\n')
    expect(textFor(parsed.places[0], 'both')).toBe('two ours\ntwo theirs\n')
    expect(textFor(parsed.places[1], 'theirs')).toBe('')
    expect(textFor(parsed.places[0], 'undecided')).toBe('')
  })

  it('moves later places along when one takes text', () => {
    const places = withChoice(startingResult(parsed).places, 0, 'theirs', 'two theirs\n')
    expect(places).toEqual([
      { status: 'theirs', from: 4, to: 15 },
      { status: 'undecided', from: 26, to: 26 },
    ])
    expect(undecided(places)).toBe(1)
    // And back again.
    expect(withChoice(places, 0, 'undecided', '')).toEqual(startingResult(parsed).places)
  })
})

describe('touches', () => {
  const p: PlaceState = { status: 'ours', from: 10, to: 20 }
  const empty: PlaceState = { status: 'undecided', from: 10, to: 10 }

  it('counts typing inside a place, and not just outside it', () => {
    expect(touches(p, 10, 10)).toBe(true)
    expect(touches(p, 15, 15)).toBe(true)
    expect(touches(p, 20, 20)).toBe(false)
    expect(touches(p, 9, 9)).toBe(false)
    expect(touches(p, 5, 12)).toBe(true)
    expect(touches(p, 20, 25)).toBe(false)
    expect(touches(p, 0, 10)).toBe(false)
  })

  it('counts typing at an empty place', () => {
    expect(touches(empty, 10, 10)).toBe(true)
    expect(touches(empty, 9, 9)).toBe(false)
    expect(touches(empty, 11, 11)).toBe(false)
    expect(touches(empty, 8, 12)).toBe(true)
  })
})

describe('placesField', () => {
  const parsed = parsePlaces(merged)
  const start = startingResult(parsed)
  const state = () => EditorState.create({ doc: start.doc, extensions: [placesField, currentField] }).update({ effects: setPlaces.of(start.places) }).state

  it('takes a side with one change, and typing in it makes it edited', () => {
    let s = state()
    const text = textFor(parsed.places[0], 'ours')
    s = s.update({ changes: { from: 4, to: 4, insert: text }, effects: setPlaces.of(withChoice(s.field(placesField), 0, 'ours', text)) }).state
    expect(s.doc.toString()).toBe('one\ntwo ours\nthree\nfour\nsix\n')
    expect(s.field(placesField)[0]).toEqual({ status: 'ours', from: 4, to: 13 })

    s = s.update({ changes: { from: 7, insert: '!' } }).state
    expect(s.field(placesField)[0]).toEqual({ status: 'edited', from: 4, to: 14 })
    expect(s.field(placesField)[1]).toEqual({ status: 'undecided', from: 25, to: 25 })
  })

  it('leaves places alone when the text around them changes, and keeps them in step', () => {
    let s = state()
    s = s.update({ changes: { from: 0, insert: 'zero\n' } }).state
    expect(s.field(placesField)).toEqual([
      { status: 'undecided', from: 9, to: 9 },
      { status: 'undecided', from: 20, to: 20 },
    ])
    // Typing at an empty place fills it by hand.
    s = s.update({ changes: { from: 9, insert: 'mine\n' } }).state
    expect(s.field(placesField)[0]).toEqual({ status: 'edited', from: 9, to: 14 })
  })

  it('typing at the start of the line after a place is not typing in the place', () => {
    let s = state()
    const text = textFor(parsed.places[0], 'theirs')
    s = s.update({ changes: { from: 4, insert: text }, effects: setPlaces.of(withChoice(s.field(placesField), 0, 'theirs', text)) }).state
    s = s.update({ changes: { from: 15, insert: 'x' } }).state
    expect(s.field(placesField)[0]).toEqual({ status: 'theirs', from: 4, to: 15 })
  })

  it('remembers which place the tool is on', () => {
    expect(state().update({ effects: setCurrent.of(1) }).state.field(currentField)).toBe(1)
  })
})

describe('getting around', () => {
  const places = (...s: PlaceState['status'][]) => s.map((status) => ({ status, from: 0, to: 0 }))

  it('goes to the next undecided place, round the end', () => {
    expect(nextPlace(places('ours', 'undecided', 'theirs', 'undecided'), 1, 1)).toBe(3)
    expect(nextPlace(places('ours', 'undecided', 'theirs', 'undecided'), 3, 1)).toBe(1)
    expect(nextPlace(places('ours', 'undecided', 'theirs', 'undecided'), 3, -1)).toBe(1)
    expect(nextPlace(places('undecided', 'ours'), -1, 1)).toBe(0)
  })

  it('steps through every place once all are decided, and has nowhere to go with none', () => {
    expect(nextPlace(places('ours', 'theirs', 'both'), 0, 1)).toBe(1)
    expect(nextPlace(places('ours', 'theirs', 'both'), 0, -1)).toBe(2)
    expect(nextPlace([], -1, 1)).toBe(-1)
  })

  it('labels a place by what was decided', () => {
    expect(placeLabel(0, 'theirs')).toBe('Place 1 · took theirs')
    expect(placeLabel(1, 'edited')).toBe("Place 2 · you're writing this one by hand")
  })
})

describe('the sides of a place', () => {
  const parsed = parsePlaces(diff3)

  it('shows one side with the lines around it', () => {
    expect(sideView(parsed, 0, 'theirs', 1)).toEqual({ lines: ['one', 'two theirs', 'three'], from: 1, to: 2, first: 1 })
    expect(sideView(parsed, 0, 'base', 1).lines).toEqual(['one', 'two', 'three'])
    // The second place on ours comes after ours' version of the first.
    expect(sideView(parsed, 1, 'ours', 1)).toEqual({ lines: ['four', 'five ours', 'more ours', 'six'], from: 1, to: 3, first: 4 })
    // Theirs deleted the lines there: nothing of its own to show.
    expect(sideView(parsed, 1, 'theirs', 1)).toMatchObject({ lines: ['four', 'six'], from: 1, to: 1 })
  })

  it("finds a place's lines in the side's real file", () => {
    const ours = 'intro\none\ntwo ours\nthree\nfour\nfive ours\nmore ours\nsix\n'
    expect(placeLines(ours, parsed, 0, 'ours')).toEqual({ start: 3, end: 3 })
    expect(placeLines(ours, parsed, 1, 'ours')).toEqual({ start: 6, end: 7 })
    expect(placeLines('one\ntwo theirs\nthree\nfour\nsix\n', parsed, 1, 'theirs')).toBeNull()
    expect(placeLines('nothing alike\n', parsed, 0, 'ours')).toBeNull()
  })

  it('picks the nearest match when the lines repeat', () => {
    const repeated = parsePlaces(['a', '<<<<<<< ours', 'x', '=======', 'y', '>>>>>>> theirs', 'b', ''].join('\n'))
    expect(placeLines('x\nx\na\nx\nb\n', repeated, 0, 'ours')).toEqual({ start: 2, end: 2 })
  })
})
