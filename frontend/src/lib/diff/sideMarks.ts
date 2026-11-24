/**
 * The marks one side of the split view draws from the Go diff: changed lines, the words that
 * changed in them, moved lines with a note saying where they went, and the change being read.
 */
import { RangeSetBuilder, StateEffect, StateField, type EditorState, type Extension } from '@codemirror/state'
import { Decoration, EditorView, WidgetType, type DecorationSet } from '@codemirror/view'
import { marksByLine, moveNote, movesByLine, type Mark, type Result } from './blocks'

export type Side = 'old' | 'new'

/** What one side shows, by line (counting from 0). */
export interface SideModel {
  side: Side
  /** Changed lines that didn't move. */
  changed: Set<number>
  /** Lines that moved. */
  moved: Set<number>
  marks: Map<number, Mark[]>
  /** A note at the end of a line: the first line of each move. */
  notes: Map<number, string>
}

export function sideModel(result: Result, side: Side): SideModel {
  const moves = movesByLine(result.moves ?? [])[side]
  const changed = new Set<number>()
  for (const b of result.blocks ?? []) {
    const [s, e] = side === 'old' ? [b.oldStart, b.oldEnd] : [b.newStart, b.newEnd]
    for (let i = s; i < e; i++) if (!moves.has(i)) changed.add(i)
  }
  const notes = new Map<number, string>()
  for (const m of result.moves ?? []) notes.set(side === 'old' ? m.oldStart : m.newStart, moveNote(m, side))
  return { side, changed, moved: new Set(moves.keys()), marks: marksByLine(result.blocks)[side], notes }
}

/**
 * Sets the change being read, or null for none: its lines [from, to), the moved ones for a
 * move and the others for a change.
 */
export interface Current {
  from: number
  to: number
  moved: boolean
}
export const setCurrent = StateEffect.define<Current | null>()

class Note extends WidgetType {
  constructor(readonly text: string) {
    super()
  }
  eq(other: Note) {
    return other.text === this.text
  }
  toDOM() {
    const el = document.createElement('span')
    el.className = 'cm-move-note'
    el.textContent = this.text
    return el
  }
}

function build(state: EditorState, model: SideModel, current: Current | null): DecorationSet {
  const out = new RangeSetBuilder<Decoration>()
  const lines = new Set<number>([...model.changed, ...model.moved, ...model.marks.keys(), ...model.notes.keys()])
  const isCurrent = (i: number) => !!current && i >= current.from && i < current.to && model.moved.has(i) === current.moved
  if (current) for (let i = current.from; i < current.to; i++) if (isCurrent(i)) lines.add(i)
  const doc = state.doc
  const word = Decoration.mark({ class: model.side === 'old' ? 'cm-word-del' : 'cm-word-add' })
  for (const i of [...lines].sort((a, b) => a - b)) {
    if (i >= doc.lines) break
    const line = doc.line(i + 1)
    const cls: string[] = []
    if (model.changed.has(i)) cls.push(model.side === 'old' ? 'cm-del' : 'cm-add')
    if (model.moved.has(i)) cls.push('cm-moved')
    if (isCurrent(i)) cls.push('cm-current')
    if (cls.length) out.add(line.from, line.from, Decoration.line({ class: cls.join(' ') }))
    for (const [from, to] of [...(model.marks.get(i) ?? [])].sort((a, b) => a[0] - b[0])) {
      const f = Math.min(from, line.length)
      const t = Math.min(to, line.length)
      if (t > f) out.add(line.from + f, line.from + t, word)
    }
    const note = model.notes.get(i)
    if (note) out.add(line.to, line.to, Decoration.widget({ widget: new Note(note), side: 1 }))
  }
  return out.finish()
}

/** The extension for one side. */
export function sideMarks(model: SideModel): Extension {
  const field = StateField.define<{ current: Current | null; deco: DecorationSet }>({
    create: (state) => ({ current: null, deco: build(state, model, null) }),
    update(value, tr) {
      let current = value.current
      for (const e of tr.effects) if (e.is(setCurrent)) current = e.value
      if (current === value.current && !tr.docChanged) return value
      return { current, deco: build(tr.state, model, current) }
    },
    provide: (f) => EditorView.decorations.from(f, (v) => v.deco),
  })
  return field
}
