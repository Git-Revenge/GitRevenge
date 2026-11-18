import type { Row } from './rowCache'

/**
 * Calls fn for each lane that runs straight through a row, top to bottom in one colour, in
 * columns first to last. The backend sends those apart from the row's other lines (graph.Page
 * in Go), packed as runs of neighbouring lanes: a run's first column, how many lanes, then a
 * colour for each, with the run's kind in passKind. A wide history has thousands of them a row
 * and room to draw a few dozen, so only the ones asked for are visited.
 */
export function eachPass(row: Row, first: number, last: number, fn: (col: number, color: number, kind: string | undefined) => void): void {
  const pass = row.pass
  if (!pass) return
  for (let i = 0, run = 0; i + 1 < pass.length; run++) {
    const col = pass[i]
    const n = Math.max(0, pass[i + 1])
    // Runs are in column order, so nothing further on is in range either.
    if (col > last) return
    const kind = row.passKind?.[run] || undefined
    for (let c = Math.max(first, col), end = Math.min(last, col + n - 1); c <= end; c++) fn(c, pass[i + 2 + c - col], kind)
    i += 2 + n
  }
}
