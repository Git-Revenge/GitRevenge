/** How far reading a repository's trailers has got (trailerindex.Status in Go). */
export interface TrailerStatus {
  reading: boolean
  read: number
  total: number
  done: boolean
  commits: number
}

/**
 * The words beside the bar: "412,000 of 1,503,134 commits", or "412,000 commits" until the
 * total has been counted. Counting runs beside the read and usually wins, but the read
 * doesn't wait for it.
 */
export function progressText(s: Pick<TrailerStatus, 'read' | 'total'>): string {
  const read = s.read.toLocaleString()
  if (s.total > 0) return `${read} of ${s.total.toLocaleString()} commits`
  return `${read} ${s.read === 1 ? 'commit' : 'commits'}`
}

/** How full the bar is, from 0 to 1; 0 while the total isn't known. */
export function progressPart(s: Pick<TrailerStatus, 'read' | 'total'>): number {
  return s.total > 0 ? Math.min(1, s.read / s.total) : 0
}
