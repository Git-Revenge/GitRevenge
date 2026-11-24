import type { Extension, Range } from '@codemirror/state'
import { Decoration, EditorView, WidgetType } from '@codemirror/view'
import { placeLabel, type Parsed, type Place, type Status, type Suggestion } from './places'
import { currentField, placesField, suggestionsField } from './placesField'

// How the conflict tool's places look in the result editor: a heading above each with its
// choices, the place's own lines tinted by whose they are, and for a place still undecided the
// two candidates side by side, since the result itself holds nothing there yet.

export interface PlaceActions {
  /** Takes a side for a place, keeps or restores its suggestion, or with 'undecided' starts it
   * again. */
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
    readonly suggestion: Suggestion | null,
  ) {
    super()
  }

  eq(other: Heading) {
    return other.index === this.index && other.status === this.status && other.current === this.current && other.place === this.place && other.suggestion === this.suggestion
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

    const s = this.suggestion
    const head = el('div', 'place-head')
    head.append(el('span', 'place-label', placeLabel(this.index, this.status)))
    // What the token merge found: what each side did, or why they can't be combined.
    const why = s?.ok ? (this.status === 'suggested' || this.status === 'combined' ? s.reason : '') : s?.reason && this.status === 'undecided' ? `can't be combined: ${s.reason}` : ''
    if (why) head.append(el('span', 'place-why', why))
    head.append(el('span', 'place-gap'))
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
    const take = (status: Status) => () => this.act.choose(this.index, status)
    if (s?.ok) {
      // A suggestion is already in the result: keep it, or take a side instead.
      chip('Keep the suggestion', 'Keep both sides combined, as suggested (3)', this.status === 'combined', take('combined'))
      for (const c of CHOICES.slice(0, 2)) chip(c.label, c.title, this.status === c.status, take(c.status))
      if (this.status === 'edited') chip('Edited', "You've written this place by hand", true)
    } else {
      for (const c of CHOICES) chip(c.label, c.title, this.status === c.status, take(c.status))
      if (this.status === 'edited') chip('Edited', "You've written this place by hand", true)
      else chip('Write it myself', 'Start from both sides, ours first, and edit it', false, take('edited'))
    }
    const fresh = s?.ok ? 'suggested' : 'undecided'
    if (this.status !== fresh) chip('Start again', s?.ok ? 'Put the suggestion back' : 'Put this place back to undecided', false, take(fresh))
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

/** Decorates the result editor with its places. A suggestion's text is washed by where each
 * part came from: ours, theirs, or both. */
export function placeDecorations(parsed: Parsed, act: PlaceActions): Extension {
  return EditorView.decorations.compute([placesField, currentField, suggestionsField], (state) => {
    const places = state.field(placesField)
    const current = state.field(currentField)
    const suggestions = state.field(suggestionsField)
    const out: Range<Decoration>[] = []
    places.forEach((p, i) => {
      if (!parsed.places[i]) return
      const s = suggestions[i] ?? null
      out.push(Decoration.widget({ widget: new Heading(i, p.status, parsed.places[i], i === current, act, s), block: true, side: -1 }).range(p.from))
      for (let pos = p.from; pos < p.to; ) {
        const line = state.doc.lineAt(pos)
        out.push(Decoration.line({ class: `place-line ${p.status}` }).range(line.from))
        pos = line.to + 1
      }
      if (s?.ok && (p.status === 'suggested' || p.status === 'combined') && p.to - p.from === s.text.length) {
        for (const o of s.origins) {
          if (o.to > o.from) out.push(Decoration.mark({ class: `origin ${o.side}` }).range(p.from + o.from, p.from + o.to))
        }
      }
    })
    return Decoration.set(out, true)
  })
}
