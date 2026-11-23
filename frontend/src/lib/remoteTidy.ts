/** Tidy in the Remotes panel (#924): which remote each branch that could be tidied belongs
 * to, the row at each remote's foot, and the words for a branch's mark. */

import type { Ref } from './graph/rowCache'
import type { RefStatus, TidyRow, TreeRow } from './sidebarTree'

export type TidyKind = 'gone' | 'merged' | 'quiet'

/** The remote a name belongs to: "origin/x" is origin's, and a local branch is its
 * upstream's. The longest remote name wins. */
function remoteFor(name: string, upstream: string | undefined, remotes: string[]): string {
  const of = (s: string) => [...remotes].sort((a, b) => b.length - a.length).find((r) => s.startsWith(`${r}/`)) ?? ''
  return of(name) || (upstream ? of(upstream) : '')
}

/** How many branches of each remote could be tidied, from TidyMarks. */
export function tidyCounts(marks: Record<string, string>, refs: Pick<Ref, 'kind' | 'short' | 'upstream'>[], remotes: string[]): Record<string, number> {
  const upstreams = new Map(refs.filter((r) => r.kind === 'local').map((r) => [r.short, r.upstream]))
  const out: Record<string, number> = {}
  for (const name of Object.keys(marks)) {
    const remote = remoteFor(name, upstreams.get(name), remotes)
    if (remote) out[remote] = (out[remote] ?? 0) + 1
  }
  return out
}

/** Adds a row at the foot of each open remote folder with something to tidy. */
export function addTidyRows(rows: TreeRow[], counts: Record<string, number>): TreeRow[] {
  if (!Object.values(counts).some((n) => n > 0)) return rows
  const out: TreeRow[] = []
  let open: { remote: string; depth: number } | null = null
  const close = () => {
    if (!open) return
    const count = counts[open.remote] ?? 0
    if (count > 0) {
      const row: TidyRow = { type: 'tidy', remote: open.remote, count, depth: open.depth + 1, label: tidyLabel(open.remote, count), startsGroup: false }
      out.push(row)
    }
    open = null
  }
  for (const row of rows) {
    if (open && row.depth <= open.depth) close()
    out.push(row)
    if (row.type === 'folder' && row.remote && row.open) open = { remote: row.label, depth: row.depth }
  }
  close()
  return out
}

export const tidyLabel = (remote: string, count: number) => `${count} ${count === 1 ? 'branch' : 'branches'} on ${remote} could be tidied`

/** A remote branch's mark, in words, never colour alone. */
export function tidyStatus(kind: string | undefined): RefStatus | undefined {
  switch (kind) {
    case 'gone':
      return { text: 'gone', spoken: 'deleted on the remote', tone: 'muted' }
    case 'merged':
      return { text: 'merged', spoken: "merged into the remote's default branch", tone: 'muted' }
    case 'quiet':
      return { text: 'quiet', spoken: 'no commits for a while', tone: 'muted' }
  }
  return undefined
}
