/**
 * The marks one side of the split view draws from the Go diff: changed lines, the text that
 * changed in them, moved lines with a note saying where they went, formatting and renames
 * with theirs, and the change being read.
 */
import { RangeSetBuilder, StateEffect, StateField, type EditorState, type Extension } from '@codemirror/state'
import { Decoration, EditorView, WidgetType, type DecorationSet } from '@codemirror/view'
import { kindOf, marksByLine, moveNote, movesByLine, moveTone, notesByLine, renameTone, type Kind, type Mark, type Result } from './blocks'

export type Side = 'old' | 'new'

/** What one side shows, by line (counting from 0). */
export interface SideModel {
  side: Side
  /** Lines in blocks, other than moved ones, by their block's kind. */
  changed: Map<number, Kind>
  /** Lines that moved, with their move's index. */
  moved: Map<number, number>
  marks: Map<number, Mark[]>
  /** A note at the end of a line: the first line of each move, formatting block and rename.
   * A move's says which, and whether it changed on the way. */
  notes: Map<number, { text: string; kind: 'move' | Kind; move?: number; changed?: boolean }>
}

/** What a side's marks can do when clicked. */
export interface SideActions {
  /** A rename was clicked: light up its places, by its number from 1. */
  rename?: (r: number) => void
  /** A move's note was clicked: go to the move. */
  pick?: (move: number) => void
  /** Open a move that changed against where it came from. */
  compare?: (move: number) => void
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
  ;(result.moves ?? []).forEach((m, k) => notes.set(side === 'old' ? m.oldStart : m.newStart, { text: moveNote(m, side), kind: 'move', move: k, changed: !!m.changed }))
  return { side, changed, moved: moves, marks: marksByLine(result.blocks, hideFormatting)[side], notes }
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

/** Sets the rename whose places are lit up, from 1, or 0 for none. */
export const setHot = StateEffect.define<number>()

class Note extends WidgetType {
  constructor(
    readonly text: string,
    readonly kind: string,
    readonly move: number | undefined,
    readonly changed: boolean,
    readonly act: SideActions,
  ) {
    super()
  }
  eq(other: Note) {
    return other.text === this.text && other.kind === this.kind && other.move === this.move && other.changed === this.changed
  }
  toDOM() {
    const el = document.createElement('span')
    el.className = this.kind === 'move' ? `cm-move-note t${moveTone(this.move ?? 0)}` : `cm-block-note cm-block-note-${this.kind}`
    const move = this.move
    if (move === undefined) {
      el.textContent = this.text
      return el
    }
    // A move's note goes to the move, and one that changed opens it against where it came from.
    const button = (label: string, cls: string, title: string, run?: (k: number) => void) => {
      if (!run) {
        el.append(document.createTextNode(label))
        return
      }
      const b = document.createElement('button')
      b.type = 'button'
      b.className = cls
      b.textContent = label
      b.title = title
      b.addEventListener('mousedown', (e) => e.preventDefault())
      b.addEventListener('click', (e) => {
        e.stopPropagation()
        run(move)
      })
      el.append(b)
    }
    button(this.text, 'cm-note-link', 'Go to this move', this.act.pick)
    if (this.changed && this.act.compare) button('Compare with the original', 'cm-note-link cm-compare', 'Open these lines against where they came from', this.act.compare)
    return el
  }

  ignoreEvent() {
    return true
  }
}

const lineClass: Record<Kind, Record<Side, string>> = {
  change: { old: 'cm-del', new: 'cm-add' },
  format: { old: 'cm-format', new: 'cm-format' },
  rename: { old: 'cm-rename', new: 'cm-rename' },
}

function build(state: EditorState, model: SideModel, current: Current | null, hot: number, act: SideActions): DecorationSet {
  const out = new RangeSetBuilder<Decoration>()
  const lines = new Set<number>([...model.changed.keys(), ...model.moved.keys(), ...model.marks.keys(), ...model.notes.keys()])
  const isCurrent = (i: number) => !!current && i >= current.from && i < current.to && model.moved.has(i) === current.moved
  if (current) for (let i = current.from; i < current.to; i++) if (isCurrent(i)) lines.add(i)
  const doc = state.doc
  const word = {
    change: Decoration.mark({ class: model.side === 'old' ? 'cm-word-del' : 'cm-word-add' }),
    format: Decoration.mark({ class: 'cm-word-format' }),
    rename: Decoration.mark({ class: 'cm-word-rename' }),
    move: Decoration.mark({ class: 'cm-word-move' }),
  }
  // Each rename has its colour, and its places light up together.
  const renamed = (r: number) =>
    Decoration.mark({ class: `cm-word-rename r${renameTone(r)}${r === hot ? ' cm-hot' : ''}`, attributes: { 'data-rename': String(r), title: 'Show every place of this rename' } })
  for (const i of [...lines].sort((a, b) => a - b)) {
    if (i >= doc.lines) break
    const line = doc.line(i + 1)
    const cls: string[] = []
    const kind = model.changed.get(i)
    if (kind) cls.push(lineClass[kind][model.side])
    if (model.moved.has(i)) cls.push('cm-moved', `t${moveTone(model.moved.get(i)!)}`)
    if (isCurrent(i)) cls.push('cm-current')
    if (cls.length) out.add(line.from, line.from, Decoration.line({ class: cls.join(' ') }))
    for (const [from, to, k, r] of [...(model.marks.get(i) ?? [])].sort((a, b) => a[0] - b[0])) {
      const f = Math.min(from, line.length)
      const t = Math.min(to, line.length)
      if (t > f) out.add(line.from + f, line.from + t, k === 'rename' && r ? renamed(r) : word[k ?? 'change'])
    }
    const note = model.notes.get(i)
    if (note) out.add(line.to, line.to, Decoration.widget({ widget: new Note(note.text, note.kind, note.move, !!note.changed, act), side: 1 }))
  }
  return out.finish()
}

/** The extension for one side. */
export function sideMarks(model: SideModel, act: SideActions = {}): Extension {
  const field = StateField.define<{ current: Current | null; hot: number; deco: DecorationSet }>({
    create: (state) => ({ current: null, hot: 0, deco: build(state, model, null, 0, act) }),
    update(value, tr) {
      let { current, hot } = value
      for (const e of tr.effects) {
        if (e.is(setCurrent)) current = e.value
        if (e.is(setHot)) hot = e.value
      }
      if (current === value.current && hot === value.hot && !tr.docChanged) return value
      return { current, hot, deco: build(tr.state, model, current, hot, act) }
    },
    provide: (f) => EditorView.decorations.from(f, (v) => v.deco),
  })
  const clicks = EditorView.domEventHandlers({
    click(e) {
      const r = Number((e.target as HTMLElement).closest<HTMLElement>('[data-rename]')?.dataset.rename ?? 0)
      if (!r || !act.rename) return false
      act.rename(r)
      return true
    },
  })
  return [field, clicks]
}
