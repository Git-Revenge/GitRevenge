/**
 * Line-level diff, hunk grouping, and applying a subset of changes. Staging, unstaging and
 * discarding hunks or single lines are all expressed as "rebuild the file applying only
 * some of the changes", which the backend then writes to the index or work tree.
 */

export type OpKind = 'eq' | 'del' | 'add'

export interface Op {
  kind: OpKind
  /** Index into the old lines (eq, del). */
  oldIndex: number
  /** Index into the new lines (eq, add). */
  newIndex: number
  /** The Go diff's block a change belongs to, when the ops come from one. */
  block?: number
}

export interface Hunk {
  /** Range of ops shown, including context. */
  start: number
  end: number
  oldStart: number
  oldCount: number
  newStart: number
  newCount: number
}

/**
 * Splits text into lines, each keeping its terminator ("\n" or "\r\n"), so joining the
 * lines reproduces the text exactly and line endings survive a round trip.
 */
export function splitLines(text: string): string[] {
  if (text === '') return []
  const lines = text.match(/[^\n]*\n|[^\n]+$/g)
  return lines ?? []
}

/** Above this edit distance, a diff degrades to "replace everything" to bound cost. */
const MAX_EDIT_DISTANCE = 4000

/** Myers' O(ND) diff over lines, with common prefix and suffix trimmed first. */
export function diffLines(a: string[], b: string[]): Op[] {
  let pre = 0
  while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++
  let suf = 0
  while (suf < a.length - pre && suf < b.length - pre && a[a.length - 1 - suf] === b[b.length - 1 - suf]) suf++

  const ops: Op[] = []
  for (let i = 0; i < pre; i++) ops.push({ kind: 'eq', oldIndex: i, newIndex: i })
  ops.push(...middle(a, b, pre, a.length - suf, pre, b.length - suf))
  for (let i = suf; i > 0; i--) ops.push({ kind: 'eq', oldIndex: a.length - i, newIndex: b.length - i })
  return ops
}

function middle(a: string[], b: string[], a0: number, a1: number, b0: number, b1: number): Op[] {
  const n = a1 - a0
  const m = b1 - b0
  const replaceAll = (): Op[] => [
    ...Array.from({ length: n }, (_, i) => ({ kind: 'del' as const, oldIndex: a0 + i, newIndex: -1 })),
    ...Array.from({ length: m }, (_, i) => ({ kind: 'add' as const, oldIndex: -1, newIndex: b0 + i })),
  ]
  if (n === 0 || m === 0) return replaceAll()

  const max = Math.min(n + m, MAX_EDIT_DISTANCE)
  const offset = max + 1
  const v = new Int32Array(2 * max + 3)
  // For walking back afterwards: how far each diagonal had got before each round. Round d only
  // looks at the 2d-1 diagonals the rounds before it reached, so that's all that's kept of it,
  // and the whole trace is d² numbers. Keeping all of v every round made it d times v's full
  // width: 128 MB for two long files with nothing in common, and 6 MB for 200 changed lines.
  const trace: Int32Array[] = []
  let found = -1
  for (let d = 0; d <= max && found < 0; d++) {
    trace.push(v.slice(offset - d + 1, offset + d))
    for (let k = -d; k <= d; k += 2) {
      let x = k === -d || (k !== d && v[offset + k - 1] < v[offset + k + 1]) ? v[offset + k + 1] : v[offset + k - 1] + 1
      let y = x - k
      while (x < n && y < m && a[a0 + x] === b[b0 + y]) {
        x++
        y++
      }
      v[offset + k] = x
      if (x >= n && y >= m) {
        found = d
        break
      }
    }
  }
  if (found < 0) return replaceAll()

  // Walk the trace backwards to recover the edit script.
  const out: Op[] = []
  let x = n
  let y = m
  for (let d = found; d > 0; d--) {
    // Diagonal k of round d's trace is at k + d - 1: it starts at diagonal -(d-1).
    const vd = trace[d]
    const at = d - 1
    const k = x - y
    const prevK = k === -d || (k !== d && vd[at + k - 1] < vd[at + k + 1]) ? k + 1 : k - 1
    const prevX = vd[at + prevK]
    const prevY = prevX - prevK
    while (x > prevX && y > prevY) {
      x--
      y--
      out.push({ kind: 'eq', oldIndex: a0 + x, newIndex: b0 + y })
    }
    if (x === prevX) {
      y--
      out.push({ kind: 'add', oldIndex: -1, newIndex: b0 + y })
    } else {
      x--
      out.push({ kind: 'del', oldIndex: a0 + x, newIndex: -1 })
    }
  }
  while (x > 0 && y > 0) {
    x--
    y--
    out.push({ kind: 'eq', oldIndex: a0 + x, newIndex: b0 + y })
  }
  out.reverse()
  // Within each run of changes, show deletions before additions, as git does.
  return groupChanges(out)
}

function groupChanges(ops: Op[]): Op[] {
  const out: Op[] = []
  let i = 0
  while (i < ops.length) {
    if (ops[i].kind === 'eq') {
      out.push(ops[i++])
      continue
    }
    const dels: Op[] = []
    const adds: Op[] = []
    while (i < ops.length && ops[i].kind !== 'eq') (ops[i].kind === 'del' ? dels : adds).push(ops[i++])
    out.push(...dels, ...adds)
  }
  return out
}

/**
 * Groups changes into hunks with `context` unchanged lines around them, like git: changes
 * within 2*context unchanged lines of each other share one. group sorts changes into kinds
 * that never share a hunk, so each can be staged on its own.
 */
export function toHunks(ops: Op[], context = 3, group: (i: number) => string = () => ''): Hunk[] {
  // Runs of changes of one group.
  const runs: { start: number; end: number; g: string }[] = []
  for (let i = 0; i < ops.length; ) {
    if (ops[i].kind === 'eq') {
      i++
      continue
    }
    const g = group(i)
    let e = i
    while (e < ops.length && ops[e].kind !== 'eq' && group(e) === g) e++
    runs.push({ start: i, end: e, g })
    i = e
  }
  const groups: { start: number; end: number; g: string }[] = []
  for (const r of runs) {
    const last = groups[groups.length - 1]
    if (last && last.g === r.g && r.start - last.end <= 2 * context) last.end = r.end
    else groups.push({ ...r })
  }
  // Context around each, never reaching into the next one's changes.
  const hunks: Hunk[] = []
  let prevEnd = 0
  groups.forEach((g, k) => {
    const start = Math.max(g.start - context, prevEnd)
    const end = Math.min(g.end + context, groups[k + 1]?.start ?? ops.length)
    hunks.push(describe(ops, start, end))
    prevEnd = end
  })
  return hunks
}

function describe(ops: Op[], start: number, end: number): Hunk {
  let oldStart = -1
  let newStart = -1
  let oldCount = 0
  let newCount = 0
  for (let i = start; i < end; i++) {
    const op = ops[i]
    // Hidden formatting's rows can lack an old line.
    if (op.kind !== 'add' && op.oldIndex >= 0) {
      if (oldStart < 0) oldStart = op.oldIndex
      oldCount++
    }
    if (op.kind !== 'del' && op.newIndex >= 0) {
      if (newStart < 0) newStart = op.newIndex
      newCount++
    }
  }
  // Line numbers are 1-based; an empty side starts at the line before, as in git.
  return {
    start,
    end,
    oldStart: oldStart < 0 ? lineBefore(ops, start, 'old') : oldStart + 1,
    oldCount,
    newStart: newStart < 0 ? lineBefore(ops, start, 'new') : newStart + 1,
    newCount,
  }
}

function lineBefore(ops: Op[], start: number, side: 'old' | 'new'): number {
  for (let i = start - 1; i >= 0; i--) {
    const idx = side === 'old' ? ops[i].oldIndex : ops[i].newIndex
    if (idx >= 0) return idx + 1
  }
  return 0
}

/**
 * Rebuilds the file from the old side, applying only the changes for which `apply(opIndex)`
 * is true. Applying everything yields the new side; applying nothing yields the old side.
 */
export function applyChanges(ops: Op[], oldLines: string[], newLines: string[], apply: (i: number) => boolean): string {
  // A file's last line may have no line ending, on either side. Kept with a line after it, it
  // would run into that line: "b" and an added "c" would be the one line "bc", which is in
  // neither version. So a line that turns out not to be last is given the ending the file uses.
  const eol = [...oldLines, ...newLines].some((l) => l.endsWith('\r\n')) ? '\r\n' : '\n'
  let out = ''
  let open = false
  const add = (line: string) => {
    if (open) out += eol
    out += line
    open = !line.endsWith('\n')
  }
  for (let i = 0; i < ops.length; i++) {
    const op = ops[i]
    if (op.kind === 'eq') add(oldLines[op.oldIndex])
    else if (op.kind === 'del') {
      if (!apply(i)) add(oldLines[op.oldIndex])
    } else if (apply(i)) add(newLines[op.newIndex])
  }
  return out
}
