/** A graph row as returned by the backend (graph.Row in Go). */
export interface Seg {
  from: number
  to: number
  color: number
  /** The branch kind of the lane this line belongs to (ADR 0009), if any. */
  kind?: string
}

export interface Row {
  index: number
  col: number
  color: number
  up: Seg[]
  down: Seg[]
  width: number
  /** The branch kind of the lane the commit sits on, if any. */
  kind?: string
  id: string
  parents: number
  author: string
  authorTime: number
  subject: string
}

/** A branch, tag or stash (git.Ref in Go). */
export interface Ref {
  name: string
  short: string
  kind: 'local' | 'remote' | 'tag' | 'stash'
  commit: string
  head: boolean
  upstream: string
  ahead: number
  behind: number
  gone: boolean
  /** A branch the repository's hide patterns match. */
  hidden?: boolean
  /** The branch's kind (ADR 0009), or empty for an ordinary branch. */
  branchKind?: string
  /** The local branch this one is stacked on; depth is its place in the stack (1 at the
   * bottom) and size the stack's height. */
  stackParent?: string
  stackDepth?: number
  stackSize?: number
  /** The parent was merged and deleted upstream: this branch needs restacking. */
  stackParentMerged?: boolean
  subject?: string
  /** When the ref last changed, in Unix seconds; missing for stashes. */
  updated?: number
}

export interface Page {
  rows: Row[]
  refs: Record<string, Ref[]>
  total: number
  loading: boolean
  /** The reader stopped ahead of the viewport; asking for later rows resumes it. */
  paused: boolean
  /** About how many rows the whole history has, or 0 until known. */
  estimate: number
}

/** The payload of the "graph:progress" event (app.GraphProgress in Go). */
export interface GraphProgress {
  path: string
  rows: number
  done: boolean
  paused: boolean
  estimate: number
  /** A reloaded graph replaced the displayed one; cached rows are stale. */
  reset: boolean
  refsReady: boolean
  error: string
}

export const CHUNK = 200

/**
 * Caches graph rows in fixed-size chunks fetched on demand. A chunk that came back short
 * (the graph was still loading) is refetched once more rows exist.
 */
export class RowCache {
  private chunks = new Map<number, Row[]>()
  private labels = new Map<string, Ref[]>()
  private inflight = new Map<number, Promise<void>>()
  private generation = 0
  total = 0
  loading = true
  paused = false
  estimate = 0

  constructor(
    private fetchPage: (start: number, count: number) => Promise<Page>,
    private onUpdate: () => void,
  ) {}

  get(i: number): Row | undefined {
    return this.chunks.get(Math.floor(i / CHUNK))?.[i % CHUNK]
  }

  /** Refs pointing at the commit, as of when its row was fetched. */
  refsFor(id: string): Ref[] {
    return this.labels.get(id) ?? []
  }

  /** Ensures rows [start, end) are loaded or loading. */
  ensure(start: number, end: number) {
    const first = Math.max(0, Math.floor(start / CHUNK))
    const last = Math.floor(Math.max(start, end - 1) / CHUNK)
    for (let c = first; c <= last; c++) {
      const have = this.chunks.get(c)
      const complete = have && (have.length === CHUNK || (!this.loading && c * CHUNK + have.length >= this.total))
      if (!complete && !this.inflight.has(c)) this.load(c)
    }
  }

  /** Records a new total from a progress event; short chunks will be refetched. */
  setTotal(total: number, loading: boolean, paused = false, estimate = 0) {
    this.total = total
    this.loading = loading
    this.paused = paused
    if (estimate) this.estimate = estimate
  }

  /** Drops everything, e.g. after the backend swapped in a reloaded graph. */
  invalidate() {
    this.chunks.clear()
    this.labels.clear()
    this.inflight.clear()
    this.generation++
  }

  private load(c: number) {
    const gen = this.generation
    const p = this.fetchPage(c * CHUNK, CHUNK)
      .then((page) => {
        if (gen !== this.generation) return
        this.chunks.set(c, page.rows)
        for (const [id, refs] of Object.entries(page.refs ?? {})) this.labels.set(id, refs)
        this.total = page.total
        this.loading = page.loading
        this.paused = page.paused
        if (page.estimate) this.estimate = page.estimate
        this.onUpdate()
      })
      .catch(() => {
        // Leave the chunk missing; the next ensure() retries.
      })
      .finally(() => {
        if (this.inflight.get(c) === p) this.inflight.delete(c)
      })
    this.inflight.set(c, p)
  }
}
