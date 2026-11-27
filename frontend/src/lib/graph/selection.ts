// Several commits selected in the graph (#1018), kept as runs of rows: Shift+click across the
// kernel selects 100,000 rows, and a run holds them all in two numbers. Runs are sorted, apart
// and never touching, so whether a row is selected is a binary search.

import type { app } from '../../../wailsjs/go/models'

/** Rows lo to hi, both included. */
export type Run = { lo: number; hi: number }

/** What the graph says of a selection of several: how many, at once, and their commits once
 * they're read. */
export type Selected = { count: number; info: app.Selection | null }

/** The runs sorted, with any that overlap or touch made one. */
export function tidy(runs: readonly Run[]): Run[] {
  const sorted = runs.filter((r) => r.hi >= r.lo && r.hi >= 0).map((r) => ({ lo: Math.max(0, r.lo), hi: r.hi }))
  sorted.sort((a, b) => a.lo - b.lo)
  const out: Run[] = []
  for (const r of sorted) {
    const last = out[out.length - 1]
    if (last && r.lo <= last.hi + 1) last.hi = Math.max(last.hi, r.hi)
    else out.push({ ...r })
  }
  return out
}

/** Whether row i is in the runs. */
export function has(runs: readonly Run[], i: number): boolean {
  let lo = 0
  let hi = runs.length - 1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    const r = runs[mid]
    if (i < r.lo) hi = mid - 1
    else if (i > r.hi) lo = mid + 1
    else return true
  }
  return false
}

/** How many rows the runs hold. */
export function size(runs: readonly Run[]): number {
  return runs.reduce((n, r) => n + r.hi - r.lo + 1, 0)
}

/** Every row from a to b, whichever is first. */
export function between(a: number, b: number): Run[] {
  return [{ lo: Math.min(a, b), hi: Math.max(a, b) }]
}

/** The runs with these added. */
export function add(runs: readonly Run[], more: readonly Run[]): Run[] {
  return tidy([...runs, ...more])
}

/** The runs with row i taken out if it's in them, and put in if it isn't. */
export function toggle(runs: readonly Run[], i: number): Run[] {
  if (!has(runs, i)) return add(runs, [{ lo: i, hi: i }])
  const out: Run[] = []
  for (const r of runs) {
    if (i < r.lo || i > r.hi) out.push({ ...r })
    else {
      if (r.lo < i) out.push({ lo: r.lo, hi: i - 1 })
      if (i < r.hi) out.push({ lo: i + 1, hi: r.hi })
    }
  }
  return out
}

/** Runs from rows, in any order; rows below 0 (commits with no row) are left out. */
export function fromRows(rows: readonly number[]): Run[] {
  return tidy(rows.filter((i) => i >= 0).map((i) => ({ lo: i, hi: i })))
}

/** The selected row nearest to i, the later one on a tie; -1 when there are none. */
export function nearest(runs: readonly Run[], i: number): number {
  let best = -1
  for (const r of runs) {
    const at = i < r.lo ? r.lo : i > r.hi ? r.hi : i
    if (best < 0 || Math.abs(at - i) < Math.abs(best - i) || (Math.abs(at - i) === Math.abs(best - i) && at > best)) best = at
  }
  return best
}
