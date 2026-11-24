/**
 * Reading the Go diff (internal/semdiff): its blocks become the ops the unified view and
 * staging use, its marks are looked up by line, and its blocks and moves become the stops
 * the views step through and the bands the ribbon draws.
 */
import type { semdiff } from '../../../wailsjs/go/models'
import type { Op } from './lineDiff'

export type Result = semdiff.Result
export type Block = semdiff.Block
export type Move = semdiff.Move

/** What a block is: a change, or when read as code, formatting only or a rename. */
export type Kind = 'change' | 'format' | 'rename'
export const kindOf = (b: Block): Kind => (b.kind === 'format' ? 'format' : b.kind === 'rename' ? 'rename' : 'change')

/**
 * The ops for a result, as diffLines makes them: every line once, deletions before additions
 * within a block. Staging rebuilds a file from these.
 */
export function opsFromBlocks(blocks: readonly Block[], oldCount: number, newCount: number): Op[] {
  const ops: Op[] = []
  let i = 0
  let j = 0
  const same = (to: number) => {
    while (i < to) ops.push({ kind: 'eq', oldIndex: i++, newIndex: j++ })
  }
  for (const b of blocks ?? []) {
    same(b.oldStart)
    for (; i < b.oldEnd; i++) ops.push({ kind: 'del', oldIndex: i, newIndex: -1 })
    for (; j < b.newEnd; j++) ops.push({ kind: 'add', oldIndex: -1, newIndex: j })
  }
  same(oldCount)
  if (j !== newCount) throw new Error(`diff covers ${j} of ${newCount} new lines`)
  return ops
}

/**
 * The ops a view draws: opsFromBlocks's, each change carrying its block's index. With
 * hideFormatting, a formatting block reads as unchanged: its new lines, numbered on both
 * sides as far as the old side goes. Only views that don't stage hide anything.
 */
export function viewOps(blocks: readonly Block[], oldCount: number, newCount: number, hideFormatting = false): Op[] {
  const ops: Op[] = []
  let i = 0
  let j = 0
  const same = (to: number) => {
    while (i < to) ops.push({ kind: 'eq', oldIndex: i++, newIndex: j++ })
  }
  ;(blocks ?? []).forEach((b, k) => {
    same(b.oldStart)
    if (hideFormatting && b.kind === 'format') {
      for (let n = 0; n < b.newEnd - b.newStart; n++) {
        ops.push({ kind: 'eq', oldIndex: b.oldStart + n < b.oldEnd ? b.oldStart + n : -1, newIndex: b.newStart + n })
      }
      i = b.oldEnd
      j = b.newEnd
      return
    }
    for (; i < b.oldEnd; i++) ops.push({ kind: 'del', oldIndex: i, newIndex: -1, block: k })
    for (; j < b.newEnd; j++) ops.push({ kind: 'add', oldIndex: -1, newIndex: j, block: k })
  })
  same(oldCount)
  if (j !== newCount) throw new Error(`diff covers ${j} of ${newCount} new lines`)
  return ops
}

/**
 * Changed text in a line, in UTF-16 offsets, as [from, to), with its kind when it's
 * formatting (the same token written differently) or a rename rather than a change.
 */
export type Mark = [number, number] | [number, number, 'format' | 'rename']

/** The marked text of each side, by line. hideFormatting leaves out formatting. */
export function marksByLine(blocks: readonly Block[], hideFormatting = false): { old: Map<number, Mark[]>; new: Map<number, Mark[]> } {
  const old = new Map<number, Mark[]>()
  const nu = new Map<number, Mark[]>()
  const add = (m: Map<number, Mark[]>, s: semdiff.Span) => {
    if (hideFormatting && s.kind === 'format') return
    const mark: Mark = s.kind === 'format' || s.kind === 'rename' ? [s.from, s.to, s.kind] : [s.from, s.to]
    const list = m.get(s.line)
    if (list) list.push(mark)
    else m.set(s.line, [mark])
  }
  for (const b of blocks ?? []) {
    if (hideFormatting && b.kind === 'format') continue
    for (const s of b.old ?? []) add(old, s)
    for (const s of b.new ?? []) add(nu, s)
  }
  return { old, new: nu }
}

/** A formatting or rename block's note, such as "same value", by the line it goes on. */
export function notesByLine(blocks: readonly Block[], hideFormatting = false): { old: Map<number, string>; new: Map<number, string> } {
  const old = new Map<number, string>()
  const nu = new Map<number, string>()
  for (const b of blocks ?? []) {
    if (!b.note || (hideFormatting && b.kind === 'format')) continue
    if (b.newEnd > b.newStart) nu.set(b.newStart, b.note)
    else old.set(b.oldStart, b.note)
  }
  return { old, new: nu }
}

/** Which move, by index, each moved line belongs to, on each side. */
export function movesByLine(moves: readonly Move[]): { old: Map<number, number>; new: Map<number, number> } {
  const old = new Map<number, number>()
  const nu = new Map<number, number>()
  moves.forEach((m, k) => {
    for (let i = m.oldStart; i < m.oldEnd; i++) old.set(i, k)
    for (let j = m.newStart; j < m.newEnd; j++) nu.set(j, k)
  })
  return { old, new: nu }
}

/** What a moved line says about where it went or came from. Line numbers count from 1. */
export function moveNote(m: Move, side: 'old' | 'new'): string {
  if (side === 'old') return `moved to line ${m.newStart + 1}`
  return `moved from line ${m.oldStart + 1}${m.reindented ? ', indented differently' : ', unchanged'}`
}

/** A place the views step to: a block, or a move. Ranges are [start, end) lines. */
export interface Stop {
  kind: 'change' | 'add' | 'del' | 'move' | 'format' | 'rename'
  oldStart: number
  oldEnd: number
  newStart: number
  newEnd: number
  /** The move's index, for a move. */
  move?: number
  /** The block's index, for a block. */
  block?: number
}

/**
 * The stops, in reading order. A block's stop leaves out the moved lines at its edges, which
 * its moves cover; a block whose lines all moved isn't a stop of its own. hideFormatting
 * leaves out formatting blocks.
 */
export function stops(result: Result, hideFormatting = false): Stop[] {
  const moved = movesByLine(result.moves ?? [])
  const trim = (start: number, end: number, m: Map<number, number>): [number, number] => {
    while (start < end && m.has(start)) start++
    while (end > start && m.has(end - 1)) end--
    return [start, end]
  }
  const out: Stop[] = []
  ;(result.blocks ?? []).forEach((b, k) => {
    const [os, oe] = trim(b.oldStart, b.oldEnd, moved.old)
    const [ns, ne] = trim(b.newStart, b.newEnd, moved.new)
    if (os === oe && ns === ne) return
    if (hideFormatting && b.kind === 'format') return
    const kind = b.kind === 'format' || b.kind === 'rename' ? b.kind : os === oe ? 'add' : ns === ne ? 'del' : 'change'
    out.push({ kind, oldStart: os, oldEnd: oe, newStart: ns, newEnd: ne, block: k })
  })
  ;(result.moves ?? []).forEach((m, k) => out.push({ kind: 'move', oldStart: m.oldStart, oldEnd: m.oldEnd, newStart: m.newStart, newEnd: m.newEnd, move: k }))
  // A move reads where it arrives.
  return out.sort((x, y) => x.newStart - y.newStart || x.oldStart - y.oldStart)
}

/**
 * The ribbon's band for a change: a curve from the old lines' top to the new lines' top, down
 * the new side, and a curve back along the bottoms. A side with no lines is a point, so an
 * addition or a removal is a wedge.
 */
export function bandPath(oldTop: number, oldBottom: number, newTop: number, newBottom: number, width: number): string {
  const m = width / 2
  const r = (n: number) => Math.round(n * 10) / 10
  return (
    `M0 ${r(oldTop)} C${m} ${r(oldTop)} ${m} ${r(newTop)} ${width} ${r(newTop)}` +
    ` L${width} ${r(newBottom)} C${m} ${r(newBottom)} ${m} ${r(oldBottom)} 0 ${r(oldBottom)} Z`
  )
}

/** One edge of a band, for its outline. */
export function edgePath(oldY: number, newY: number, width: number): string {
  const m = width / 2
  const r = (n: number) => Math.round(n * 10) / 10
  return `M0 ${r(oldY)} C${m} ${r(oldY)} ${m} ${r(newY)} ${width} ${r(newY)}`
}

/**
 * Where a line of one side is on the other, for scrolling the sides together. line may be
 * fractional, for a point partway down a line. Between blocks the sides run in step; inside a
 * block the position is spread over the block's lines on the other side.
 */
export function mapLine(blocks: readonly Block[], line: number, from: 'old' | 'new'): number {
  let offset = 0 // other side's line minus this side's, before the block
  for (const b of blocks ?? []) {
    const [s, e, os, oe] = from === 'old' ? [b.oldStart, b.oldEnd, b.newStart, b.newEnd] : [b.newStart, b.newEnd, b.oldStart, b.oldEnd]
    if (line < s) break
    if (line < e) return os + ((line - s) / (e - s)) * (oe - os)
    offset = oe - e
  }
  return line + offset
}
