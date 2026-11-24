/**
 * The marks one side of the split view draws from the Go diff: changed lines, the text that
 * changed in them, moved lines with a note saying where they went, formatting and renames
 * with theirs, and the change being read.
 */
import { RangeSetBuilder, StateEffect, StateField, type EditorState, type Extension } from '@codemirror/state'
import { Decoration, EditorView, WidgetType, type DecorationSet } from '@codemirror/view'
import { kindOf, marksByLine, moveNote, movesByLine, notesByLine, type Kind, type Mark, type Result } from './blocks'

export type Side = 'old' | 'new'

/** What one side shows, by line (counting from 0). */
export interface SideModel {
  side: Side
  /** Lines in blocks, other than moved ones, by their block's kind. */
  changed: Map<number, Kind>
  /** Lines that moved. */
  moved: Set<number>
  marks: Map<number, Mark[]>
  /** A note at the end of a line: the first line of each move, formatting block and rename. */
  notes: Map<number, { text: string; kind: 'move' | Kind }>
}

/** One side's model. hideFormatting leaves out formatting, as if it were unchanged. */
export function sideModel(result: Result, side: Side, hideFormatting = false): SideModel {
  const moves = movesByLine(result.moves ?? [])[side]
  const changed = new Map<number, Kind>()
  for (const b of result.blocks ?? []) {
    const kind = kindOf(b)
    if (hideFormatting && kind === 'format') continue
    const [s, e] = side === 'old' ? [b.oldStart, b.oldEnd] : [b.newStart, b.newEnd]
    for (let i = s; i < e; i++) if (!moves.has(i)) changed.set(i, kind)
  }
  const notes: SideModel['notes'] = new Map()
  const blockNotes = notesByLine(result.blocks, hideFormatting)[side]
  for (const [line, text] of blockNotes) notes.set(line, { text, kind: changed.get(line) ?? 'change' })
  for (const m of result.moves ?? []) notes.set(side === 'old' ? m.oldStart : m.newStart, { text: moveNote(m, side), kind: 'move' })
  return { side, changed, moved: new Set(moves.keys()), marks: marksByLine(result.blocks, hideFormatting)[side], notes }
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
  constructor(
    readonly text: string,
    readonly kind: string,
  ) {
    super()
  }
  eq(other: Note) {
    return other.text === this.text && other.kind === this.kind
  }
  toDOM() {
    const el = document.createElement('span')
    el.className = this.kind === 'move' ? 'cm-move-note' : `cm-block-note cm-block-note-${this.kind}`
    el.textContent = this.text
    return el
  }
}

const lineClass: Record<Kind, Record<Side, string>> = {
  change: { old: 'cm-del', new: 'cm-add' },
  format: { old: 'cm-format', new: 'cm-format' },
  rename: { old: 'cm-rename', new: 'cm-rename' },
}

function build(state: EditorState, model: SideModel, current: Current | null): DecorationSet {
  const out = new RangeSetBuilder<Decoration>()
  const lines = new Set<number>([...model.changed.keys(), ...model.moved, ...model.marks.keys(), ...model.notes.keys()])
  const isCurrent = (i: number) => !!current && i >= current.from && i < current.to && model.moved.has(i) === current.moved
  if (current) for (let i = current.from; i < current.to; i++) if (isCurrent(i)) lines.add(i)
  const doc = state.doc
  const word = {
    change: Decoration.mark({ class: model.side === 'old' ? 'cm-word-del' : 'cm-word-add' }),
    format: Decoration.mark({ class: 'cm-word-format' }),
    rename: Decoration.mark({ class: 'cm-word-rename' }),
  }
  for (const i of [...lines].sort((a, b) => a - b)) {
    if (i >= doc.lines) break
    const line = doc.line(i + 1)
    const cls: string[] = []
    const kind = model.changed.get(i)
    if (kind) cls.push(lineClass[kind][model.side])
    if (model.moved.has(i)) cls.push('cm-moved')
    if (isCurrent(i)) cls.push('cm-current')
    if (cls.length) out.add(line.from, line.from, Decoration.line({ class: cls.join(' ') }))
    for (const [from, to, k] of [...(model.marks.get(i) ?? [])].sort((a, b) => a[0] - b[0])) {
      const f = Math.min(from, line.length)
      const t = Math.min(to, line.length)
      if (t > f) out.add(line.from + f, line.from + t, word[k ?? 'change'])
    }
    const note = model.notes.get(i)
    if (note) out.add(line.to, line.to, Decoration.widget({ widget: new Note(note.text, note.kind), side: 1 }))
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
