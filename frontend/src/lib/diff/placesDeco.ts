import type { Extension, Range } from '@codemirror/state'
import { Decoration, EditorView, WidgetType } from '@codemirror/view'
import { placeLabel, type Parsed, type Place, type Status } from './places'
import { currentField, placesField } from './placesField'

// How the conflict tool's places look in the result editor: a heading above each with its
// choices, the place's own lines tinted by whose they are, and for a place still undecided the
// two candidates side by side, since the result itself holds nothing there yet.

export interface PlaceActions {
  /** Takes a side for a place, or with 'undecided' starts it again. */
  choose: (index: number, status: Status) => void
  /** Makes a place the current one. */
  go: (index: number) => void
}

const CHOICES: { status: Status; label: string; title: string }[] = [
  { status: 'ours', label: 'Ours', title: 'Take ours here' },
  { status: 'theirs', label: 'Theirs', title: 'Take theirs here' },
  { status: 'both', label: 'Both', title: 'Keep both here, ours first' },
]

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text = ''): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag)
  e.className = cls
  if (text) e.textContent = text
  return e
}

class Heading extends WidgetType {
  constructor(
    readonly index: number,
    readonly status: Status,
    readonly place: Place,
    readonly current: boolean,
    readonly act: PlaceActions,
  ) {
    super()
  }

  eq(other: Heading) {
    return other.index === this.index && other.status === this.status && other.current === this.current && other.place === this.place
  }

  toDOM() {
    const box = el('div', `place ${this.status}${this.current ? ' current' : ''}`)
    box.setAttribute('role', 'group')
    box.setAttribute('aria-label', placeLabel(this.index, this.status))
    box.dataset.place = String(this.index)
    // A click on the heading makes it the current place without moving focus out of the text.
    box.addEventListener('mousedown', (e) => {
      e.preventDefault()
      this.act.go(this.index)
    })

    const head = el('div', 'place-head')
    head.append(el('span', 'place-label', placeLabel(this.index, this.status)), el('span', 'place-gap'))
    const chip = (label: string, title: string, on: boolean, run?: () => void) => {
      const b = el('button', `chip${on ? ' on' : ''}`, label)
      b.type = 'button'
      b.title = title
      b.setAttribute('aria-pressed', String(on))
      if (run) {
        b.addEventListener('mousedown', (e) => {
          e.preventDefault()
          e.stopPropagation()
        })
        b.addEventListener('click', run)
      } else b.disabled = true
      head.append(b)
    }
    for (const c of CHOICES) chip(c.label, c.title, this.status === c.status, () => this.act.choose(this.index, c.status))
    if (this.status === 'edited') chip('Edited', "You've written this place by hand", true)
    if (this.status !== 'undecided') chip('Start again', 'Put this place back to undecided', false, () => this.act.choose(this.index, 'undecided'))
    box.append(head)

    if (this.status === 'undecided') {
      // Nothing is in the result here yet, so show what each side would put.
      const sides = el('div', 'place-sides')
      for (const [name, text] of [
        ['Ours', this.place.ours],
        ['Theirs', this.place.theirs],
      ] as const) {
        const side = el('div', `place-side ${name.toLowerCase()}`)
        side.append(el('span', 'place-side-name', name))
        side.append(text ? el('pre', 'place-side-text', text.replace(/\n$/, '')) : el('span', 'place-side-none', 'nothing here: this side removed the lines'))
        sides.append(side)
      }
      box.append(sides)
    }
    return box
  }

  // The heading handles its own clicks.
  ignoreEvent() {
    return true
  }
}

/** Decorates the result editor with its places. */
export function placeDecorations(parsed: Parsed, act: PlaceActions): Extension {
  return EditorView.decorations.compute([placesField, currentField], (state) => {
    const places = state.field(placesField)
    const current = state.field(currentField)
    const out: Range<Decoration>[] = []
    places.forEach((p, i) => {
      if (!parsed.places[i]) return
      out.push(Decoration.widget({ widget: new Heading(i, p.status, parsed.places[i], i === current, act), block: true, side: -1 }).range(p.from))
      for (let pos = p.from; pos < p.to; ) {
        const line = state.doc.lineAt(pos)
        out.push(Decoration.line({ class: `place-line ${p.status}` }).range(line.from))
        pos = line.to + 1
      }
    })
    return Decoration.set(out, true)
  })
}
