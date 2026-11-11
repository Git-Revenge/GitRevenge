/**
 * Folding machine commits. A lane of a kind that folds (Bot, Docs) takes one line a day in the
 * graph: "11 Docs commits", in the place of the lane's newest commit that day. Folding goes by
 * the lane's kind, never by guessing from an author's name.
 */

/** What folding needs to know of a commit. */
export interface Foldable {
  /** The commit's row in the graph. */
  index: number
  /** The kind of the lane it sits on, if any. */
  kind?: string
  /** The lane's column. */
  col: number
  /** When it was made, in Unix seconds. */
  authorTime: number
  /** A branch, tag or stash points at it: it carries a label, so it stays a row of its own. */
  labelled: boolean
}

/** One lane's commits of one day, folded into a line. */
export interface Fold {
  /** Which fold this is, the same across reloads: the lane's kind and column, and the day. */
  key: string
  /** The newest of its commits, where the line sits. */
  head: number
  /** How many commits it holds. */
  count: number
  /** When the newest was made. */
  last: number
  kind: string
}

const pad = (n: number) => String(n).padStart(2, '0')

/** A day as a key: the local date. */
export function dayKey(unixSeconds: number): string {
  const d = new Date(unixSeconds * 1000)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/**
 * The folds in one day's commits (newest first, as the graph lists them), by commit. A commit
 * folds when its lane's kind is one that folds and it carries no label; each lane's commits of
 * the day share a fold. Commits that don't fold aren't in the answer.
 */
export function foldDay(day: readonly Foldable[], folding: ReadonlySet<string>): Map<number, Fold> {
  const out = new Map<number, Fold>()
  const byLane = new Map<string, Fold>()
  for (const c of day) {
    if (!c.kind || !folding.has(c.kind) || c.labelled) continue
    const lane = `${c.kind}:${c.col}`
    let fold = byLane.get(lane)
    if (!fold) {
      fold = { key: `${lane}:${dayKey(c.authorTime)}`, head: c.index, count: 0, last: c.authorTime, kind: c.kind }
      byLane.set(lane, fold)
    }
    fold.count++
    out.set(c.index, fold)
  }
  return out
}

/** Whether a commit in a fold is out of sight: its fold is closed, and it isn't the selected
 * commit or a search match, which always show. */
export function foldedAway(fold: Fold | null | undefined, open: ReadonlySet<string>, selected: boolean, matched: boolean): boolean {
  return !!fold && !open.has(fold.key) && !selected && !matched
}

/** What a fold's line says: "11 Docs commits", "1 Bot commit". */
export function foldLabel(count: number, kindLabel: string): string {
  return `${count.toLocaleString()} ${kindLabel} ${count === 1 ? 'commit' : 'commits'}`
}
