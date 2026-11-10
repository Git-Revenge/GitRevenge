import { StateEffect, StateField, type Transaction } from '@codemirror/state'
import { touches, type PlaceState } from './places'

// The conflict tool's places as editor state: they move with the text, and typing inside one
// makes it edited by hand. No view code here, so it can be tested without a browser.

/** Replaces the places outright: on load, and when a side is taken or a place is started again. */
export const setPlaces = StateEffect.define<PlaceState[]>()

/** The place the tool is on, by index, or -1. */
export const setCurrent = StateEffect.define<number>()

function move(places: PlaceState[], tr: Transaction): PlaceState[] {
  return places.map((p) => {
    let touched = false
    tr.changes.iterChangedRanges((from, to) => {
      if (touches(p, from, to)) touched = true
    })
    // A place that was typed into grows with what was typed; one that wasn't keeps its edges
    // against text added just outside them.
    return touched ? { status: 'edited', from: tr.changes.mapPos(p.from, -1), to: tr.changes.mapPos(p.to, 1) } : { ...p, from: tr.changes.mapPos(p.from, 1), to: Math.max(tr.changes.mapPos(p.from, 1), tr.changes.mapPos(p.to, -1)) }
  })
}

export const placesField = StateField.define<PlaceState[]>({
  create: () => [],
  update(places, tr) {
    for (const e of tr.effects) if (e.is(setPlaces)) return e.value
    return tr.docChanged ? move(places, tr) : places
  },
})

export const currentField = StateField.define<number>({
  create: () => -1,
  update(current, tr) {
    for (const e of tr.effects) if (e.is(setCurrent)) return e.value
    return current
  },
})
